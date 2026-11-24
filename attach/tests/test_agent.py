"""Firmware-independent tests using a disposable, already-running host JVM."""
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
import zipfile
from types import SimpleNamespace
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
JDK = os.environ.get('DAYNIGHT_TEST_JDK')


@unittest.skipUnless(JDK and (Path(JDK)/'bin/javac').is_file(), 'Set DAYNIGHT_TEST_JDK to JDK 17+')
class AgentLifecycleTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.TemporaryDirectory(prefix='daynight-agent-test-')
        cls.root = Path(cls.tmp.name)
        cls.java = Path(JDK)/'bin'
        (cls.root/'Target.java').write_text('public class Target { public static void main(String[] a) throws Exception { System.out.println(java.lang.management.ManagementFactory.getRuntimeMXBean().getName().split("@")[0]); System.out.flush(); Thread.sleep(120000); } }')
        (cls.root/'Load.java').write_text('import com.sun.tools.attach.VirtualMachine; public class Load { public static void main(String[] a) throws Exception { VirtualMachine vm=VirtualMachine.attach(a[0]); try { vm.loadAgent(a[1],a[2]); } finally { vm.detach(); } } }')
        subprocess.run([str(cls.java/'javac'), '-d', str(cls.root), *[str(ROOT/'attach/java'/name) for name in ('DayNightAgent.java','PatchSession.java','UiExecutor.java','AgentRequest.java')],
                        str(cls.root/'Target.java'), str(cls.root/'Load.java')], check=True, capture_output=True)
        (cls.root/'agents').mkdir()
        cls.jar = cls.root/'agents/agent.jar'
        with zipfile.ZipFile(cls.jar,'w') as jar:
            jar.writestr('META-INF/MANIFEST.MF','Manifest-Version: 1.0\r\nAgent-Class: lab.daynight.attach.DayNightAgent\r\nCan-Redefine-Classes: true\r\nCan-Retransform-Classes: true\r\n\r\n')
            for file in (cls.root/'lab').rglob('*.class'):
                jar.write(file,file.relative_to(cls.root).as_posix())
            jar.writestr('pins.properties',''.join('fixture.View%d=%s\n'%(i,'0'*64) for i in range(10)))
        shutil.rmtree(cls.root/'lab')

    @classmethod
    def tearDownClass(cls):
        cls.tmp.cleanup()

    def setUp(self):
        self.target = subprocess.Popen([str(self.java/'java'),'-cp',str(self.root),'Target'],stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
        self.pid = self.target.stdout.readline().strip()
        self.assertTrue(self.pid.isdigit())

    def tearDown(self):
        self.target.terminate()
        self.target.communicate(timeout=10)

    def load(self, action, name):
        report=self.root/name
        result=subprocess.run([str(self.java/'java'),'-cp',str(self.root),'Load',self.pid,str(self.jar),action+'|'+str(report)],capture_output=True,text=True,timeout=30)
        return result,report

    def test_status_is_repeatable_without_restarting_target(self):
        first,path=self.load('status','first.json')
        self.assertEqual(first.returncode,0,first.stderr)
        before=json.loads(path.read_text())
        self.assertTrue(before['ok']);self.assertFalse(before['installed'])
        second,path=self.load('status','second.json')
        self.assertEqual(second.returncode,0,second.stderr)
        after=json.loads(path.read_text())
        self.assertEqual(before['runtime'],after['runtime'])
        self.assertEqual(before['start_time_ms'],after['start_time_ms'])
        self.assertIsNone(self.target.poll())

    def test_unknown_target_is_rejected_and_same_jvm_remains_usable(self):
        result,path=self.load('install','refused.json')
        self.assertNotEqual(result.returncode,0)
        report=json.loads(path.read_text())
        self.assertFalse(report['ok']);self.assertFalse(report['installed'])
        self.assertIn('original control center',report['error'])
        result,path=self.load('status','after-refusal.json')
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(json.loads(path.read_text())['runtime'],report['runtime'])
        self.assertIsNone(self.target.poll())

    def test_existing_result_file_is_never_overwritten(self):
        path=self.root/'existing.json';path.write_text('preserve me')
        result,_=self.load('status','existing.json')
        self.assertNotEqual(result.returncode,0)
        self.assertEqual(path.read_text(),'preserve me')


class BuilderBoundaryTests(unittest.TestCase):
    def load_builder(self):
        spec=importlib.util.spec_from_file_location('attach_build',ROOT/'attach/build.py')
        module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
        return module

    def test_versioned_inputs_check_every_digest(self):
        module=self.load_builder()
        args=SimpleNamespace(version='2995',view_jar=None,hmi_jar=None,osgi_jar=None)
        module.resolve_version_inputs(args)
        self.assertEqual(args.view_jar,ROOT/'source/2995/viewhandler.jar')
        self.assertTrue(args.hmi_jar.is_file())
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp);folder=root/'source/2995';folder.mkdir(parents=True)
            manifest=json.loads((ROOT/'source/2995/inputs.json').read_text())
            (folder/'inputs.json').write_text(json.dumps(manifest))
            for key,entry in manifest['files'].items():
                (folder/entry['path']).write_bytes(b'wrong input')
            with patch.object(module,'ROOT',root):
                with self.assertRaisesRegex(ValueError,'SHA-256 mismatch'):
                    module.resolve_version_inputs(SimpleNamespace(version='2995',view_jar=None,hmi_jar=None,osgi_jar=None))

    def test_version_directory_traversal_rejected(self):
        module=self.load_builder()
        for version in ('../2995','2995/other','２９９５'):
            with self.assertRaises(ValueError):
                module.resolve_version_inputs(SimpleNamespace(version=version))

    def test_failed_publication_removes_only_its_new_directory(self):
        module=self.load_builder()
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp);work=root/'work';work.mkdir();out=root/'out'
            with self.assertRaises(FileNotFoundError): module.publish(work,out)
            self.assertFalse(out.exists())
            out.mkdir();(out/'preserve').write_text('existing build')
            with self.assertRaises(FileExistsError): module.publish(work,out)
            self.assertEqual((out/'preserve').read_text(),'existing build')

    def test_unknown_firmware_does_not_create_output(self):
        spec=importlib.util.spec_from_file_location('attach_build',ROOT/'attach/build.py')
        module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
        with tempfile.TemporaryDirectory() as tmp:
            base=Path(tmp);(base/'unknown.jar').write_bytes(b'not firmware')
            class Args:
                output=base/'out'
                view_jar=base/'unknown.jar'
                hmi_jar=base/'unknown.jar'
            with self.assertRaisesRegex(ValueError,'Unsupported original'):
                module.build(Args())
            self.assertFalse(Args.output.exists())


if __name__=='__main__': unittest.main()

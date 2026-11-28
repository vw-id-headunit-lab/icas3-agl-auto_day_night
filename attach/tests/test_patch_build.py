"""Version rules and direct definition construction, replacing full-JAR build tests."""
import copy
import importlib.util
import json
import os
from pathlib import Path
import sys
import tempfile
import unittest
import zipfile

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT/'attach'))
from patch_config import fixed_edits, load_inputs, render_geometry_rules, sha, validate_spec

JDK = os.environ.get('DAYNIGHT_TEST_JDK')


class VersionRulesTests(unittest.TestCase):
    def setUp(self):
        self.metadata = json.loads((ROOT/'source/2995/inputs.json').read_text())

    def test_version_identity_is_distinct_and_targets_are_verified_for_both(self):
        versions = [load_inputs(version, ROOT)[0] for version in ('2995','3001')]
        self.assertNotEqual(versions[0]['files']['view_jar']['sha256'], versions[1]['files']['view_jar']['sha256'])
        self.assertNotEqual(versions[0]['files']['hmi_jar']['sha256'], versions[1]['files']['hmi_jar']['sha256'])
        self.assertEqual(versions[0]['patch'], versions[1]['patch'])
        for version in ('2995','3001'):
            metadata, paths = load_inputs(version, ROOT)
            with zipfile.ZipFile(paths['view_jar']) as jar:
                for target in metadata['patch']['targets']:
                    name = metadata['patch']['class_prefix']+target['name']+'.class'
                    self.assertEqual(sha(jar.read(name)), target['sha256'])

    def test_invalid_target_sets_are_rejected(self):
        for change in ('duplicate', 'missing_row', 'dimensions'):
            spec = copy.deepcopy(self.metadata['patch'])
            if change == 'duplicate': spec['targets'].append(spec['targets'][0])
            elif change == 'missing_row': spec['targets'].pop()
            else: spec['targets'][0]['widgets'][0]['patched_height'] = 0
            with self.assertRaises(ValueError): validate_spec(spec)

    def test_input_filename_cannot_escape_version_directory(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp); folder = root/'source/2995'; folder.mkdir(parents=True)
            self.metadata['files']['view_jar']['path'] = '../outside.jar'
            (folder/'inputs.json').write_text(json.dumps(self.metadata))
            with self.assertRaisesRegex(ValueError, 'inside its version directory'): load_inputs('2995', root)

    def test_live_geometry_and_fixed_bytes_share_manifest(self):
        spec = copy.deepcopy(self.metadata['patch'])
        spec['targets'][0]['widgets'][0]['patched_height'] = 420
        source = render_geometry_rules(spec)
        self.assertIn('patched ? 420 : 280', source)
        target = spec['targets'][0]
        with zipfile.ZipFile(ROOT/'source/2995/viewhandler.jar') as jar:
            original = jar.read(spec['class_prefix']+target['name']+'.class')
        patched = fixed_edits(original, target)
        offset = target['widgets'][0]['bytecode_offset']
        self.assertEqual(int.from_bytes(patched[offset+1:offset+3], 'big'), 420)

    def test_wrong_target_or_opcode_is_rejected(self):
        spec = self.metadata['patch']; target = copy.deepcopy(spec['targets'][0])
        with zipfile.ZipFile(ROOT/'source/2995/viewhandler.jar') as jar:
            data = jar.read(spec['class_prefix']+target['name']+'.class')
        with self.assertRaisesRegex(ValueError, 'Unknown target'): fixed_edits(b'unknown', target)
        target['widgets'][0]['bytecode_offset'] = 0
        with self.assertRaisesRegex(ValueError, 'opcode'): fixed_edits(data, target)


@unittest.skipUnless(JDK and (Path(JDK)/'bin/javac').is_file(), 'Set DAYNIGHT_TEST_JDK')
class RuntimeAgentBuildTests(unittest.TestCase):
    def test_agent_does_not_request_jvm_class_modification(self):
        spec = importlib.util.spec_from_file_location('direct_attach_build', ROOT/'attach/build.py')
        builder = importlib.util.module_from_spec(spec); spec.loader.exec_module(builder)
        for version in ('2995', '3001'):
            with self.subTest(version=version):
                metadata, paths = load_inputs(version, ROOT)
                before = sha(paths['view_jar'].read_bytes())
                with tempfile.TemporaryDirectory() as tmp:
                    work = Path(tmp)
                    builder.build_jars(metadata, paths, Path(JDK), work)
                    with zipfile.ZipFile(work/'daynight-agent.jar') as agent:
                        manifest = agent.read('META-INF/MANIFEST.MF')
                        self.assertNotIn(b'Can-Redefine-Classes', manifest)
                        self.assertNotIn(b'Can-Retransform-Classes', manifest)
                        self.assertFalse(any(name.startswith('patched/') for name in agent.namelist()))
                        self.assertEqual(len(agent.read('pins.properties').splitlines()), len(metadata['patch']['targets']))
                self.assertEqual(sha(paths['view_jar'].read_bytes()), before)

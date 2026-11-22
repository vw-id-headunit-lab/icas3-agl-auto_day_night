import json
import os
from pathlib import Path
import tempfile
import unittest
import zipfile
from build_patch import build, VISIBILITY, GENERAL, ClassFile, sha

ROOT = Path(__file__).resolve().parents[1]
EVIDENCE = ROOT.parent/'artifacts/day-night-analysis/extracted/agl-vw_hmi.ext4.sparse.vw_eu/hmi'
VIEW = Path(os.environ.get('DAYNIGHT_VIEW_JAR', str(EVIDENCE/'Resources/skin1/viewhandler.jar')))
MAIN = Path(os.environ.get('DAYNIGHT_HMI_JAR', str(EVIDENCE/'jar/MIBHMI-STD-HCN1.1.134-bin.jar')))


class PatchTests(unittest.TestCase):
    def test_scope_originals_and_no_overwrite(self):
        view_hash, main_hash = sha(VIEW.read_bytes()), sha(MAIN.read_bytes())
        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp)/'candidate'
            report = build(VIEW,out,MAIN,True)
            self.assertFalse(report['installed'])
            self.assertFalse(report['vehicle_validated'])
            for original,candidate,targets,expected_bytes in [
                (VIEW,out/'viewhandler.candidate.jar',set(VISIBILITY),2),
                (MAIN,out/'MIBHMI-STD.candidate.jar',{GENERAL},2),
            ]:
                with zipfile.ZipFile(original) as a, zipfile.ZipFile(candidate) as b:
                    self.assertEqual(a.namelist(),b.namelist())
                    changed = {n for n in a.namelist() if a.read(n)!=b.read(n)}
                    self.assertEqual(changed,targets)
                    total = sum(sum(x!=y for x,y in zip(a.read(n),b.read(n))) for n in changed)
                    self.assertEqual(total,expected_bytes)
            before = (out/'patch-report.json').read_bytes()
            with self.assertRaises(ValueError): build(VIEW,out,MAIN,True)
            self.assertEqual((out/'patch-report.json').read_bytes(),before)
            self.assertEqual(sha(VIEW.read_bytes()),view_hash)
            self.assertEqual(sha(MAIN.read_bytes()),main_hash)

    def test_unrecognized_firmware_leaves_no_output(self):
        with tempfile.TemporaryDirectory() as tmp:
            p = Path(tmp)/'unknown.jar';p.write_bytes(b'not inspected firmware')
            out=Path(tmp)/'must-not-exist'
            with self.assertRaises(ValueError): build(p,out)
            self.assertFalse(out.exists())
            with self.assertRaises(ValueError): build(VIEW,out,p,True)
            self.assertFalse(out.exists())


if __name__ == '__main__': unittest.main()

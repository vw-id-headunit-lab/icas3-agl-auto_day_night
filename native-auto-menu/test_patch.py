import os
from pathlib import Path
import tempfile
import unittest
import zipfile
from build_patch import build, VISIBILITY, sha

ROOT = Path(__file__).resolve().parents[1]
VIEW = Path(os.environ.get('DAYNIGHT_VIEW_JAR', str(ROOT/'inputs/viewhandler.jar')))


class PatchTests(unittest.TestCase):
    @unittest.skipUnless(VIEW.is_file(), 'Matching vendor input is supplied locally, never in CI')
    def test_scope_originals_and_no_overwrite(self):
        view_hash = sha(VIEW.read_bytes())
        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp)/'candidate'
            report = build(VIEW, out)
            self.assertFalse(report['installed'])
            self.assertFalse(report['vehicle_validated'])
            self.assertNotIn('selection_fix', report)
            self.assertEqual(report['viewhandler']['candidate_sha256'],
                             'b2b29caa094c181749e7b25f02a66fa98522e195276b1338b15695df1f7f8019')
            with zipfile.ZipFile(VIEW) as a, zipfile.ZipFile(out/'viewhandler.candidate.jar') as b:
                self.assertEqual(a.namelist(), b.namelist())
                changed = {n for n in a.namelist() if a.read(n) != b.read(n)}
                self.assertEqual(changed, set(VISIBILITY))
                for name in changed:
                    self.assertEqual(len(a.read(name)), len(b.read(name)))
                    self.assertEqual(sum(x != y for x,y in zip(a.read(name), b.read(name))), 1)
                self.assertEqual(b.testzip(), None)
            before = (out/'patch-report.json').read_bytes()
            with self.assertRaises(ValueError):
                build(VIEW, out)
            self.assertEqual((out/'patch-report.json').read_bytes(), before)
            self.assertEqual(sha(VIEW.read_bytes()), view_hash)
            self.assertEqual({p.name for p in out.iterdir()}, {'viewhandler.candidate.jar','patch-report.json'})

    def test_unrecognized_firmware_leaves_no_output(self):
        with tempfile.TemporaryDirectory() as tmp:
            p = Path(tmp)/'unknown.jar'
            p.write_bytes(b'not inspected firmware')
            out = Path(tmp)/'must-not-exist'
            with self.assertRaises(ValueError):
                build(p, out)
            self.assertFalse(out.exists())


if __name__ == '__main__':
    unittest.main()

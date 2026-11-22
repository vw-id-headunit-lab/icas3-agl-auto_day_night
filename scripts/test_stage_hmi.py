import json
import os
from pathlib import Path
import tempfile
import unittest
from stage_hmi import patch_bundles, stage


class StageTests(unittest.TestCase):
    def test_all_profiles_and_id_collisions(self):
        text = ('Bundle.Name.80 = Existing\r\nBundle.Startup.Lateload = 70,80\r\n'
                'Bundle.Startup.Lateload.TARGET.DEBUG = 70,80 # debug\r\n')
        candidate, slot = patch_bundles(text)
        self.assertEqual(slot, 81)
        self.assertIn('70,80,81\r\n', candidate)
        self.assertIn('70,80,81 # debug\r\n', candidate)
        self.assertEqual(text.count('VWDayNightBridge'), 0)
        with self.assertRaises(ValueError):
            patch_bundles(candidate)
        with self.assertRaises(ValueError):
            patch_bundles('Bundle.Name.1 = A\n')

    def test_stage_is_copy_only_and_read_only(self):
        root = Path(__file__).resolve().parents[1]
        evidence = root.parent / 'artifacts/day-night-analysis/extracted/agl-vw_hmi.ext4.sparse.vw_eu/hmi'
        source = Path(os.environ.get('DAYNIGHT_BUNDLES', str(evidence / 'config/bundles.properties')))
        main_jar = Path(os.environ.get('DAYNIGHT_HMI_JAR', str(evidence / 'jar/MIBHMI-STD-HCN1.1.134-bin.jar')))
        run_hmi = Path(os.environ.get('DAYNIGHT_RUN_HMI', str(evidence / 'runHMI.sh')))
        before = source.read_bytes()
        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp)/'candidate'
            result = stage(source, main_jar, root/'dist/vw-daynight-bridge.jar', out, run_hmi)
            self.assertFalse(result['vehicle_validated'])
            self.assertFalse(result['writes_enabled'])
            self.assertIn('allow_write=false', (out/'bridge.properties').read_text())
            self.assertEqual((out/'bridge.token').stat().st_mode & 0o777, 0o600)
            self.assertEqual(len((out/'bridge.token').read_text().strip()), 64)
            self.assertEqual(source.read_bytes(), before)
            self.assertEqual((out/'bundles.properties.original').read_bytes(), before)
            with self.assertRaises(ValueError):
                stage(source,main_jar,root/'dist/vw-daynight-bridge.jar',out,run_hmi)


if __name__ == '__main__':
    unittest.main()

"""Deterministic failure, cancellation and geometry regression tests."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
import json
import sys
sys.path.insert(0,str(Path(__file__).resolve().parents[1]))
from patch_config import render_geometry_rules

ROOT = Path(__file__).resolve().parents[2]
JDK = os.environ.get('DAYNIGHT_TEST_JDK')


@unittest.skipUnless(JDK and (Path(JDK)/'bin/javac').is_file(), 'Set DAYNIGHT_TEST_JDK')
class ReviewRegressionTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory(prefix='daynight-review-test-')
        cls.root = Path(cls.temp.name)
        cls.java = Path(JDK)/'bin'
        sources = ['PatchSession.java', 'UiExecutor.java', 'AgentRequest.java']
        geometry=cls.root/'GeometryRules.java'
        geometry.write_text(render_geometry_rules(json.loads((ROOT/'source/2995/inputs.json').read_text())['patch']))
        subprocess.run([str(cls.java/'javac'), '--release', '8', '-d', str(cls.root),
                        *[str(ROOT/'attach/java'/name) for name in sources], str(geometry),
                        str(ROOT/'attach/tests/java/ReviewHarness.java'),
                        str(ROOT/'attach/tests/java/GeometryHarness.java')], check=True, capture_output=True)

    @classmethod
    def tearDownClass(cls):
        cls.temp.cleanup()

    def run_case(self, case):
        result = subprocess.run([str(self.java/'java'), '-cp', str(self.root),
                                 'lab.daynight.attach.ReviewHarness', case, str(self.root)],
                                capture_output=True, text=True, timeout=15)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_failed_recovery_blocks_install_until_successful_remove(self):
        self.run_case('rollback')

    def test_linkage_error_runs_rollback(self):
        self.run_case('linkage')

    def test_expired_queue_never_applies_later(self):
        self.run_case('queue')

    def test_interrupted_started_mutation_finishes_before_return(self):
        self.run_case('interrupt')

    def test_report_reserved_before_mutation_and_control_characters_escaped(self):
        self.run_case('report')

    def test_geometry_does_not_match_unrelated_widget_with_same_height(self):
        result = subprocess.run([str(self.java/'java'), '-cp', str(self.root),
                                 'generated.de.vw.mib.appui.controlcenter.internal.view.GeometryHarness'],
                                capture_output=True, text=True, timeout=15)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

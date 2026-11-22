"""Go CLI -> real loopback transport -> Java bridge -> simulated native API."""
import json
import os
from pathlib import Path
import secrets
import subprocess
import tempfile
import unittest


class IntegrationTests(unittest.TestCase):
    def test_switch_between_policies_and_restore(self):
        root = Path(__file__).resolve().parents[1]
        java = Path(os.environ.get('DAYNIGHT_JDK', os.environ.get('JAVA_HOME', '')))/'bin/java'
        with tempfile.TemporaryDirectory() as tmp:
            tmp = Path(tmp)
            token = tmp/'bridge.token'
            token.write_text(secrets.token_hex(32)+'\n')
            token.chmod(0o600)
            cp = os.pathsep.join(str(root/'dist'/p) for p in ['classes', 'test-classes'])
            proc = subprocess.Popen([str(java), '-cp', cp, 'lab.vw.daynight.BridgeFixture', str(token)],
                                    stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
            try:
                port = int(proc.stdout.readline().strip())
                config = tmp/'config.json'
                binary = root/'dist/daynight-local'

                def cli(*args, success=True):
                    p = subprocess.run([str(binary), '--config', str(config), *args], capture_output=True, text=True, timeout=12)
                    if success:
                        self.assertEqual(p.returncode, 0, p.stderr)
                        return json.loads(p.stdout) if p.stdout.strip() else None
                    self.assertNotEqual(p.returncode, 0)
                    return p

                cli('init')
                c = json.loads(config.read_text())
                c['bridge_address'] = '127.0.0.1:%s' % port
                c['interval_seconds'] = 5
                config.write_text(json.dumps(c))
                self.assertEqual(cli('status')['configured_mode'], 1)
                cli('select', 'native-auto')
                self.assertEqual(cli('apply')['action'], 'would_set')
                self.assertEqual(cli('status')['configured_mode'], 1)
                self.assertEqual(cli('--write', 'apply')['status']['configured_mode'], 2)
                self.assertEqual(cli('--write', 'apply')['action'], 'unchanged')
                cli('configure', '--day-start', '08:00', '--night-start', '20:00', '--latitude', '31.23', '--longitude', '121.47')
                cli('select', 'fixed')
                preview = cli('preview')
                self.assertEqual(cli('--write', 'apply')['status']['configured_mode'], preview['desired_mode'])
                cli('select', 'solar')
                solar = cli('--at', '2026-06-21T12:00:00+08:00', 'preview')
                self.assertEqual(solar['desired_mode'], 0)
                self.assertIn('sunrise', solar)
                cli('--at', '2026-06-21T12:00:00+08:00', '--write', 'apply', success=False)
                cli('--write', 'apply')
                self.assertEqual(cli('--write', 'restore')['status']['configured_mode'], 1)
                # An old controller never races another apply through the same state path.
                daemon = subprocess.Popen([str(binary), '--config', str(config), 'run'], stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)
                try:
                    import time
                    deadline = time.monotonic()+3
                    while time.monotonic()<deadline:
                        p=subprocess.run([str(binary), '--config', str(config), 'apply'], capture_output=True, text=True)
                        if p.returncode and 'another controller' in p.stderr:
                            break
                    else:
                        self.fail('daemon did not retain state lock')
                    # Selecting a mode is permitted while the daemon owns its state lock.
                    cli('select', 'off')
                finally:
                    daemon.terminate();daemon.communicate(timeout=4)
            finally:
                proc.stdin.close()
                proc.wait(timeout=4)
                stderr=proc.stderr.read()
                proc.stdout.close();proc.stderr.close()
                self.assertEqual(proc.returncode, 0, stderr)


if __name__ == '__main__':
    unittest.main()

"""Run the complete offline build/test suite and record exactly which sources passed."""
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import subprocess
import sys

root = Path(__file__).resolve().parents[1]
result = subprocess.run(['sh', 'scripts/build.sh'], cwd=root, capture_output=True, text=True)
(root/'dist').mkdir(exist_ok=True)
(root/'dist/validation.log').write_text(result.stdout+result.stderr)
print(result.stdout, end='')
print(result.stderr, end='', file=sys.stderr)
if result.returncode:
    (root/'dist/validation.json').unlink(missing_ok=True)
    sys.exit(result.returncode)
sources = {}
for directory in ['cmd', 'internal', 'hmi', 'scripts']:
    for p in sorted((root/directory).rglob('*')):
        if p.is_file() and p.suffix in ['.go', '.java', '.py', '.sh']:
            sources[p.relative_to(root).as_posix()] = hashlib.sha256(p.read_bytes()).hexdigest()
report = {'version': '0.1.0-experimental', 'generated_utc': datetime.now(timezone.utc).isoformat(),
          'vehicle_connected': False, 'vehicle_validated': False,
          'checks': {'go_race_tests': 'passed', 'java_hmi_queue_and_protocol_tests': 'passed',
                     'python_stage_and_go_java_integration': 'passed'},
          'sources_sha256': sources}
(root/'dist/validation.json').write_text(json.dumps(report, indent=2)+'\n')

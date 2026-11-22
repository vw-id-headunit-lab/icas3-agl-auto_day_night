"""Package source and tested experimental artifacts without firmware or credentials."""
from pathlib import Path
import hashlib
import json
import shutil
import tarfile
import tempfile

root = Path(__file__).resolve().parents[1]
validation = json.loads((root/'dist/validation.json').read_text())
for name, expected in validation['sources_sha256'].items():
    if hashlib.sha256((root/name).read_bytes()).hexdigest() != expected:
        raise RuntimeError('source changed since validation: '+name)
release = 'vw-daynight-0.1.0-experimental'
with tempfile.TemporaryDirectory(prefix='vw-daynight-package-') as tmp:
    out = Path(tmp)/release
    out.mkdir()
    for folder in ['cmd', 'internal', 'hmi/src', 'hmi/test', 'scripts', 'examples']:
        shutil.copytree(root/folder, out/folder, ignore=shutil.ignore_patterns('__pycache__'))
    for name in ['go.mod', 'firmware.json', '使用说明.txt', 'THIRD_PARTY_NOTICES.txt']:
        shutil.copy2(root/name, out/name)
    shutil.copy2(root/'dist/daynight-linux-arm64', out/'daynight')
    shutil.copy2(root/'dist/vw-daynight-bridge.jar', out/'hmi/vw-daynight-bridge.jar')
    shutil.copy2(root/'dist/validation.json', out/'validation.json')
    shutil.copy2(root/'dist/validation.log', out/'validation.log')
    (out/'daynight').chmod(0o755)
    (out/'scripts/choose-mode.sh').chmod(0o755)
    sums=[]
    for p in sorted(out.rglob('*')):
        if p.is_file():
            sums.append(hashlib.sha256(p.read_bytes()).hexdigest()+'  '+p.relative_to(out).as_posix())
    (out/'SHA256SUMS').write_text('\n'.join(sums)+'\n')
    archive = root/'dist'/(release+'.tar.gz')
    with tarfile.open(archive,'w:gz') as tar:
        tar.add(out,arcname=release)
    digest = hashlib.sha256(archive.read_bytes()).hexdigest()
    Path(str(archive)+'.sha256').write_text(digest+'  '+archive.name+'\n')
    print(json.dumps({'archive':str(archive),'sha256':digest,'bytes':archive.stat().st_size,'vehicle_validated':False},indent=2))

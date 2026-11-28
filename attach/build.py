"""Build the single Attach executable directly from one version manifest."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import zipfile

from patch_config import load_inputs, render_geometry_rules, sha

ROOT = Path(__file__).resolve().parents[1]
AGENT_SOURCES = ('DayNightAgent.java', 'PatchSession.java', 'UiExecutor.java', 'AgentRequest.java')
HELPER_SOURCES = ('DayNightSelection.java', 'RuntimeViews.java', 'ViewNodes.java')


def write_jar(path, entries, manifest=None):
    with zipfile.ZipFile(path, 'w', zipfile.ZIP_DEFLATED) as output:
        if manifest:
            entries = {'META-INF/MANIFEST.MF': manifest.encode(), **entries}
        for name, content in sorted(entries.items()):
            info = zipfile.ZipInfo(name, (1980, 1, 1, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            info.external_attr = 0o100600 << 16
            output.writestr(info, content)


def compile_java(java, destination, sources, classpath=None):
    destination.mkdir()
    command = [str(java/'javac'), '--release', '8', '-Xlint:-options', '-d', str(destination)]
    if classpath:
        command += ['-cp', classpath]
    subprocess.run(command + [str(source) for source in sources], check=True)


def class_entries(directory):
    entries = {path.relative_to(directory).as_posix(): path.read_bytes()
               for path in directory.rglob('*.class')}
    for name, content in entries.items():
        if content[:4] != b'\xca\xfe\xba\xbe' or int.from_bytes(content[6:8], 'big') != 52:
            raise ValueError('Expected Java 8 class: '+name)
    return entries


def build_pins(spec, view):
    pins = []
    prefix = spec['class_prefix']
    with zipfile.ZipFile(view) as original:
        if len(original.namelist()) != len(set(original.namelist())):
            raise ValueError('Duplicate original ZIP entries')
        for target in spec['targets']:
            name = prefix+target['name']+'.class'
            digest = sha(original.read(name))
            if digest != target['sha256']:
                raise ValueError('Unknown target class: '+name)
            pins.append(name.replace('/', '.').removesuffix('.class')+'='+digest)
    return ('\n'.join(pins)+'\n').encode()


def build_jars(metadata, paths, jdk, work):
    java = jdk.resolve()/'bin'
    spec = metadata['patch']
    pins = build_pins(spec, paths['view_jar'])
    geometry = work/'GeometryRules.java'
    geometry.write_text(render_geometry_rules(spec))
    helper_classes = work/'helpers'
    compile_java(java, helper_classes,
                 [*[ROOT/'attach/java'/name for name in HELPER_SOURCES], geometry],
                 os.pathsep.join([str(paths['hmi_jar']), str(paths['osgi_jar'])]))
    helper = class_entries(helper_classes)
    write_jar(work/'daynight-helpers.jar', helper)
    identity = hashlib.sha256()
    for name in AGENT_SOURCES + HELPER_SOURCES:
        identity.update((ROOT/'attach/java'/name).read_bytes())
    identity.update(json.dumps(metadata, sort_keys=True).encode())
    identity.update(pins)
    for name, data in sorted(helper.items()):
        identity.update(name.encode()); identity.update(data)
    package = 'lab.daynight.attach.v'+identity.hexdigest()[:12]
    generated = work/'agent-sources'
    generated.mkdir()
    sources = []
    for name in AGENT_SOURCES:
        target = generated/name
        target.write_text((ROOT/'attach/java'/name).read_text().replace(
            'package lab.daynight.attach;', 'package '+package+';'))
        sources.append(target)
    agent_classes = work/'agent'
    compile_java(java, agent_classes, sources)
    agent = class_entries(agent_classes)
    agent['pins.properties'] = pins
    agent['bundle.properties'] = ('helpers.sha256='+sha((work/'daynight-helpers.jar').read_bytes())+'\n').encode()
    agent['firmware.properties'] = (
        'view.sha256='+metadata['files']['view_jar']['sha256']+'\n'
        'hmi.sha256='+metadata['files']['hmi_jar']['sha256']+'\n'
        'helper.prefix='+spec['class_prefix'].replace('/', '.')+'\n'
        'target.count='+str(len(spec['targets']))+'\n').encode()
    write_jar(work/'daynight-agent.jar', agent,
              'Manifest-Version: 1.0\r\nAgent-Class: '+package+'.DayNightAgent\r\n\r\n')


def publish(work, output, version=None):
    output.mkdir(mode=0o700)
    try:
        shutil.copy2(work/'daynight-patch', output/'daynight-patch')
        (output/'daynight-patch').chmod(0o700)
        records = {'daynight-patch': sha((output/'daynight-patch').read_bytes())}
        (output/'build.json').write_text(json.dumps({
            'mode': 'runtime-attach', 'firmware_version': version,
            'firmware_modified': False, 'vm_verified': False, 'files': records,
        }, indent=2)+'\n')
        return records
    except BaseException:
        shutil.rmtree(output)
        raise


def build(args):
    output = args.output.absolute()
    if output.exists() or output.is_symlink():
        raise FileExistsError('Output must be a new directory')
    metadata, paths = load_inputs(args.version, ROOT)
    with tempfile.TemporaryDirectory(prefix='daynight-attach-') as temporary:
        work = Path(temporary)
        build_jars(metadata, paths, args.jdk, work)
        launcher = work/'launcher'
        shutil.copytree(ROOT/'attach/cmd', launcher/'cmd')
        shutil.copy2(ROOT/'attach/go.mod', launcher/'go.mod')
        payload = launcher/'cmd/daynight-attach/payload'
        for name in ('daynight-agent.jar', 'daynight-helpers.jar'):
            shutil.copy2(work/name, payload/name)
        subprocess.run([
            'go', 'build', '-trimpath', '-o', str(work/'daynight-patch'), './cmd/daynight-attach',
        ], cwd=launcher, env=dict(os.environ, GOOS='linux', GOARCH='arm64', CGO_ENABLED='0'), check=True)
        print(json.dumps(publish(work, output, args.version), indent=2))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--jdk', required=True, type=Path)
    parser.add_argument('--version', required=True, help='Read source/<version>/inputs.json')
    parser.add_argument('--output', required=True, type=Path)
    build(parser.parse_args())

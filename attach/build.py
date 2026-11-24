"""Build a local Java 8 Attach bundle from pinned, user-supplied firmware."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
AGENT_SOURCES = ('DayNightAgent.java', 'PatchSession.java', 'UiExecutor.java', 'AgentRequest.java')
HELPER_SOURCES = ('RuntimeViews.java', 'ViewNodes.java', 'GeometryRules.java')
PREFIX = 'generated/de/vw/mib/appui/controlcenter/internal/view/'


def sha(data):
    return hashlib.sha256(data).hexdigest()


def write_jar(path, entries, manifest=None):
    with zipfile.ZipFile(path, 'w', zipfile.ZIP_DEFLATED) as output:
        if manifest:
            entries = {'META-INF/MANIFEST.MF': manifest.encode(), **entries}
        for name, content in sorted(entries.items()):
            info = zipfile.ZipInfo(name, (1980, 1, 1, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            info.external_attr = 0o100600 << 16
            output.writestr(info, content)


def verify_inputs(args):
    metadata = json.loads((ROOT/'firmware.json').read_text())
    for path, key, label in (
        (args.view_jar, 'viewhandler_original_sha256', 'viewhandler.jar'),
        (args.hmi_jar, 'hmi_original_sha256', 'HMI JAR'),
    ):
        if sha(path.read_bytes()) != metadata[key]:
            raise ValueError('Unsupported original '+label)


def resolve_version_inputs(args):
    version = getattr(args, 'version', None)
    if version is None:
        return
    if not version.isascii() or not version.isdigit():
        raise ValueError('Version must be an ASCII numeric directory name')
    folder = ROOT/'source'/version
    metadata = json.loads((folder/'inputs.json').read_text())
    if metadata.get('firmware_version') != version:
        raise ValueError('Input manifest version differs from selected version')
    for key in ('view_jar', 'hmi_jar', 'osgi_jar'):
        if getattr(args, key, None) is not None:
            raise ValueError('Use --version or explicit JAR paths, not both')
        entry = metadata['files'][key]
        filename = entry['path']
        if not filename or Path(filename).name != filename or filename in ('.', '..'):
            raise ValueError('Input filename must stay inside its version directory')
        path = folder/filename
        if sha(path.read_bytes()) != entry['sha256']:
            raise ValueError('Versioned input SHA-256 mismatch: '+filename)
        setattr(args, key, path)


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


def patch_payload(view, candidate):
    entries, pins = {}, []
    with zipfile.ZipFile(view) as original, zipfile.ZipFile(candidate) as patched:
        changed = sorted(name for name in original.namelist() if original.read(name) != patched.read(name))
        if len(changed) != 10 or any(not name.startswith(PREFIX) for name in changed):
            raise ValueError('Unexpected patch scope')
        for name in changed:
            entries['patched/'+name] = patched.read(name)
            pins.append(name.replace('/', '.').removesuffix('.class')+'='+sha(original.read(name)))
    entries['pins.properties'] = ('\n'.join(pins)+'\n').encode()
    return entries


def build_jars(args, work, candidate):
    java = args.jdk.resolve()/'bin'
    helper_classes, agent_classes = work/'helpers', work/'agent'
    classpath = os.pathsep.join([str(args.hmi_jar.resolve()), str(args.osgi_jar.resolve())])
    compile_java(java, helper_classes,
                 [ROOT/'menu/java'/PREFIX/'DayNightSelection.java',
                  *[ROOT/'attach/java'/name for name in HELPER_SOURCES]], classpath)
    helper = class_entries(helper_classes)
    write_jar(work/'daynight-helpers.jar', helper)
    identity = b''.join((ROOT/'attach/java'/name).read_bytes() for name in AGENT_SOURCES + HELPER_SOURCES)
    identity += (ROOT/'menu/java'/PREFIX/'DayNightSelection.java').read_bytes()
    identity += candidate.read_bytes()
    identity += b''.join(helper[name] for name in sorted(helper))
    package = 'lab.daynight.attach.v' + sha(identity)[:12]
    generated = work/'agent-sources'
    generated.mkdir()
    sources = []
    for name in AGENT_SOURCES:
        target = generated/name
        target.write_text((ROOT/'attach/java'/name).read_text().replace(
            'package lab.daynight.attach;', 'package '+package+';'))
        sources.append(target)
    compile_java(java, agent_classes, sources)
    agent = class_entries(agent_classes)
    agent.update(patch_payload(args.view_jar, candidate))
    agent['bundle.properties'] = ('helpers.sha256='+sha((work/'daynight-helpers.jar').read_bytes())+'\n').encode()
    write_jar(work/'daynight-agent.jar', agent,
              'Manifest-Version: 1.0\r\nAgent-Class: '+package+'.DayNightAgent\r\n'
              'Can-Redefine-Classes: true\r\nCan-Retransform-Classes: true\r\n\r\n')


def publish(work, output):
    # Atomic directory creation prevents a concurrent build from replacing any existing output.
    output.mkdir(mode=0o700)
    try:
        records = {}
        for name in ('daynight-patch',):
            shutil.copy2(work/name, output/name)
            (output/name).chmod(0o700)
            records[name] = sha((output/name).read_bytes())
        (output/'build.json').write_text(json.dumps({
            'mode': 'runtime-attach', 'changed_classes': 10,
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
    resolve_version_inputs(args)
    verify_inputs(args)
    with tempfile.TemporaryDirectory(prefix='daynight-attach-') as temporary:
        work = Path(temporary)
        subprocess.run([
            'go', 'run', './cmd/daynightlab', '--sources', '.', '--jdk', str(args.jdk.resolve()),
            '--view-jar', str(args.view_jar.resolve()), '--hmi-jar', str(args.hmi_jar.resolve()),
            '--output', str(work/'menu'),
        ], cwd=ROOT/'menu', check=True)
        build_jars(args, work, work/'menu/viewhandler.candidate.jar')
        # Embed private generated classes only in the temporary build checkout.
        launcher = work/'launcher'
        shutil.copytree(ROOT/'attach/cmd', launcher/'cmd')
        shutil.copy2(ROOT/'attach/go.mod', launcher/'go.mod')
        payload = launcher/'cmd/daynight-attach/payload'
        for name in ('daynight-agent.jar', 'daynight-helpers.jar'):
            shutil.copy2(work/name, payload/name)
        subprocess.run([
            'go', 'build', '-trimpath', '-o', str(work/'daynight-patch'), './cmd/daynight-attach',
        ], cwd=launcher, env=dict(os.environ, GOOS='linux', GOARCH='arm64', CGO_ENABLED='0'), check=True)
        print(json.dumps(publish(work, output), indent=2))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--jdk', required=True, type=Path)
    parser.add_argument('--version', help='Read inputs from source/<version>/inputs.json')
    parser.add_argument('--view-jar', type=Path)
    parser.add_argument('--hmi-jar', type=Path)
    parser.add_argument('--osgi-jar', type=Path)
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args()
    if not args.version and not all((args.view_jar, args.hmi_jar, args.osgi_jar)):
        parser.error('Supply --version or all three JAR input paths')
    build(args)

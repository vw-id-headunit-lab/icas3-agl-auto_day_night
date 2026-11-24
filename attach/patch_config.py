"""One version manifest drives input checks, bytecode edits and live widget rules."""
import hashlib
import json
from pathlib import Path
import re
import struct

ROOT = Path(__file__).resolve().parents[1]


def sha(data):
    return hashlib.sha256(data).hexdigest()


def validate_spec(spec):
    prefix = spec.get('class_prefix', '')
    if not re.fullmatch(r'(?:[A-Za-z_$][\w$]*/)+', prefix, flags=re.ASCII):
        raise ValueError('Invalid class prefix')
    if not isinstance(spec.get('day_night_property'), int):
        raise ValueError('Missing day/night property')
    names, groups = set(), {}
    for target in spec.get('targets', []):
        name = target['name']
        if not re.fullmatch(r'[A-Za-z_$][\w$]*', name, flags=re.ASCII) or name in names:
            raise ValueError('Invalid or duplicate target class')
        names.add(name)
        if not re.fullmatch('[0-9a-f]{64}', target['sha256']):
            raise ValueError('Invalid target digest')
        kind = target['kind']
        if kind not in ('layout', 'selection', 'row'):
            raise ValueError('Unknown target kind')
        groups.setdefault(target['group'], []).append(target)
        if kind == 'layout':
            indexes, labels = set(), set()
            if not target.get('widgets'):
                raise ValueError('Empty layout geometry')
            for widget in target['widgets']:
                if widget['index'] in indexes or widget['name'] in labels:
                    raise ValueError('Duplicate geometry target')
                indexes.add(widget['index']); labels.add(widget['name'])
                if not isinstance(widget['index'], int) or widget['index'] < 0:
                    raise ValueError('Invalid widget index')
                if not 0 < widget['original_height'] < widget['patched_height'] < 32768:
                    raise ValueError('Invalid widget dimensions')
                if not isinstance(widget['bytecode_offset'], int) or widget['bytecode_offset'] < 0:
                    raise ValueError('Invalid geometry byte offset')
        elif not isinstance(target.get('daynight_reads'), int) or target['daynight_reads'] < 1:
            raise ValueError('Invalid day/night read count')
        if kind == 'row' and target.get('mode') not in (0, 1, 2):
            raise ValueError('Invalid row mode')
        if kind == 'row' and target['mode'] == 2 and 'visibility' not in target:
            raise ValueError('AUTO row requires its visibility edit')
        if 'visibility' in target:
            change = target['visibility']
            if kind != 'row' or target['mode'] != 2 or change['before'] != 3 or change['after'] != 4:
                raise ValueError('Visibility patch must reveal the AUTO row')
            if not isinstance(change['bytecode_offset'], int) or change['bytecode_offset'] < 0:
                raise ValueError('Invalid visibility byte offset')
    if not names:
        raise ValueError('Empty patch target set')
    for targets in groups.values():
        if sum(t['kind'] == 'layout' for t in targets) != 1 or sum(t['kind'] == 'selection' for t in targets) != 1:
            raise ValueError('Each layout needs one root and selection')
        if sorted(t['mode'] for t in targets if t['kind'] == 'row') != [0, 1, 2]:
            raise ValueError('Each layout needs all three mode rows')


def load_inputs(version, root=ROOT):
    if not version or not version.isascii() or not version.isdigit():
        raise ValueError('Version must be an ASCII numeric directory name')
    folder = root/'source'/version
    metadata = json.loads((folder/'inputs.json').read_text())
    if metadata.get('firmware_version') != version:
        raise ValueError('Input manifest version differs from selected version')
    validate_spec(metadata['patch'])
    paths = {}
    for key in ('view_jar', 'hmi_jar', 'osgi_jar'):
        entry = metadata['files'][key]
        name = entry['path']
        if not name or Path(name).name != name or name in ('.', '..'):
            raise ValueError('Input filename must stay inside its version directory')
        path = folder/name
        if path.is_symlink() or sha(path.read_bytes()) != entry['sha256']:
            raise ValueError('Versioned input SHA-256 mismatch: '+name)
        paths[key] = path
    return metadata, paths


def fixed_edits(original, target):
    if sha(original) != target['sha256']:
        raise ValueError('Unknown target class: '+target['name'])
    data = bytearray(original)
    if 'visibility' in target:
        change = target['visibility']; offset = change['bytecode_offset']
        if offset >= len(data) or data[offset] != change['before']:
            raise ValueError('Visibility opcode mismatch')
        data[offset] = change['after']
    for widget in target.get('widgets', []):
        offset = widget['bytecode_offset']
        if offset + 4 > len(data) or data[offset] != 0x11 or data[offset + 3] != 0xb6:
            raise ValueError('Geometry opcode mismatch')
        if struct.unpack_from('>H', data, offset + 1)[0] != widget['original_height']:
            raise ValueError('Geometry original height mismatch')
        struct.pack_into('>H', data, offset + 1, widget['patched_height'])
    return bytes(data)


def render_geometry_rules(spec):
    validate_spec(spec)
    package = spec['class_prefix'].rstrip('/').replace('/', '.')
    lines = ['package '+package+';', '', '// Generated from the selected version manifest; do not edit.',
             'final class GeometryRules {',
             '    static final int DAY_NIGHT_PROPERTY = '+str(spec['day_night_property'])+';',
             '    static final String CLASS_PREFIX = '+json.dumps(package+'.')+';',
             '    static int mode(String node) {']
    for target in spec['targets']:
        mode = {'layout': -1, 'selection': -2}.get(target['kind'], target.get('mode'))
        lines.append('        if ('+json.dumps(target['name'])+'.equals(node)) return '+str(mode)+';')
    lines += ['        return -3;', '    }',
              '    static boolean targetNode(String node) { return mode(node) != -3; }',
              '    static int height(String node, int index, String widget, boolean patched) {']
    for target in spec['targets']:
        for widget in target.get('widgets', []):
            lines.append('        if ('+json.dumps(target['name'])+'.equals(node) && index == '+str(widget['index'])
                         +' && '+json.dumps(widget['name'])+'.equals(widget)) return patched ? '
                         +str(widget['patched_height'])+' : '+str(widget['original_height'])+';')
    lines += ['        return -1;', '    }',
              '    static int originalHeight(String node, int index, String widget) { return height(node,index,widget,false); }',
              '    static int patchedHeight(String node, int index, String widget) { return height(node,index,widget,true); }',
              '    static int widgetCount(String node) {']
    for target in spec['targets']:
        if target['kind'] == 'layout':
            lines.append('        if ('+json.dumps(target['name'])+'.equals(node)) return '+str(len(target['widgets']))+';')
    lines += ['        return 0;', '    }', '    static String[][] layouts() { return new String[][] {']
    for group in dict.fromkeys(t['group'] for t in spec['targets']):
        lines.append('        {'+','.join(json.dumps(t['name']) for t in spec['targets'] if t['group'] == group)+'},')
    return '\n'.join(lines+['    }; }', '}', ''])

"""Build an offline S2995 native AUTO menu candidate. Never install or alter inputs."""
import argparse
import hashlib
import json
from pathlib import Path
import struct
import zipfile

VIEW_JAR_SHA256 = '9bd5764fabe2cf79e5193f2dd39c38954f6170a8f016c229c133e2b8165a1ce1'
PREFIX = 'generated/de/vw/mib/appui/controlcenter/internal/view/'
VISIBILITY = {
    PREFIX+'Sc__5.class': '14f0d5e6984303726c5710f7bb3d4abecf7f590b4948e6ffcfacd62f6b7d4349',
    PREFIX+'Scm__9.class': 'b21c88d6e664a027e2a2581f47aa4360a401ea564c2873324f6e0825fc876722',
}
GENERAL = 'de/vw/mib/asl/internal/system/AslTargetSystemGeneral.class'
GENERAL_SHA256 = '5f447a4e80e3451855c57b11ecabedbd195f02a5032252dd575d4ecef7e6f674'


def sha(data):
    return hashlib.sha256(data).hexdigest()


class ClassFile:
    """Read constant-pool and Code boundaries. No arbitrary byte-pattern matching."""
    def __init__(self, data):
        if data[:4] != b'\xca\xfe\xba\xbe':
            raise ValueError('not a JVM class')
        self.data = data
        self.cp = [None] * self.u2(8)
        pos, i = 10, 1
        while i < len(self.cp):
            tag = data[pos]; pos += 1
            if tag == 1:
                n = self.u2(pos); pos += 2
                self.cp[i] = (tag, data[pos:pos+n].decode('utf-8', errors='replace')); pos += n
            elif tag in (3, 4):
                self.cp[i] = (tag, data[pos:pos+4]); pos += 4
            elif tag in (5, 6):
                self.cp[i] = (tag, data[pos:pos+8]); pos += 8; i += 1
            elif tag in (7, 8, 16, 19, 20):
                self.cp[i] = (tag, self.u2(pos)); pos += 2
            elif tag in (9, 10, 11, 12, 17, 18):
                self.cp[i] = (tag, self.u2(pos), self.u2(pos+2)); pos += 4
            elif tag == 15:
                self.cp[i] = (tag, data[pos], self.u2(pos+1)); pos += 3
            else:
                raise ValueError('unsupported constant pool tag '+str(tag))
            i += 1
        pos += 6  # flags, this class, superclass
        interfaces = self.u2(pos); pos += 2 + 2*interfaces
        self.methods = {}
        self.code_attributes = {}
        for table in range(2):
            count = self.u2(pos); pos += 2
            for _ in range(count):
                name, descriptor, attributes = self.u2(pos+2), self.u2(pos+4), self.u2(pos+6)
                pos += 8
                for _ in range(attributes):
                    kind, size = self.u2(pos), self.u4(pos+2); payload = pos+6
                    if table == 1 and self.utf(kind) == 'Code':
                        length = self.u4(payload+4)
                        key = (self.utf(name), self.utf(descriptor))
                        self.methods[key] = (payload+8, length)
                        nested = payload+8+length
                        nested += 2+8*self.u2(nested)
                        count_nested = self.u2(nested); nested += 2
                        attributes_by_name = {}
                        for _ in range(count_nested):
                            attr_name, attr_size = self.utf(self.u2(nested)), self.u4(nested+2)
                            attributes_by_name[attr_name] = (nested+6, attr_size)
                            nested += 6+attr_size
                        self.code_attributes[key] = attributes_by_name
                    pos = payload+size

    def u2(self, pos): return struct.unpack_from('>H', self.data, pos)[0]
    def u4(self, pos): return struct.unpack_from('>I', self.data, pos)[0]
    def utf(self, index):
        tag, value = self.cp[index]
        if tag != 1: raise ValueError('expected UTF8 constant')
        return value

    def method_ref(self, index):
        tag, cls, nt = self.cp[index]
        if tag not in (10, 11): raise ValueError('expected method reference')
        _, name, descriptor = self.cp[nt]
        return self.utf(self.cp[cls][1]), self.utf(name), self.utf(descriptor)

    def replace_before_call(self, method, descriptor, offset, before, after, owner, callee):
        start, length = self.methods[(method, descriptor)]
        if offset+4 > length: raise ValueError('instruction outside method')
        pos = start+offset
        if self.data[pos] != before or self.data[pos+1] != 0xb6:
            raise ValueError('unexpected opcode at verified patch location')
        ref = self.method_ref(self.u2(pos+2))
        if ref[0] != owner or ref[1] != callee:
            raise ValueError('unexpected call after patch location: '+repr(ref))
        changed = bytearray(self.data); changed[pos] = after
        return bytes(changed), {'method': method, 'descriptor': descriptor, 'code_offset': offset,
                                'class_file_offset': pos, 'before': hex(before), 'after': hex(after),
                                'following_call': list(ref)}

    def preserve_integer_local(self, method, descriptor, frame_offset, slot):
        start, _ = self.code_attributes[(method, descriptor)]['StackMapTable']
        count, pos, current = self.u2(start), start+2, -1

        def verification_type(at):
            tag = self.data[at]
            return at+3 if tag in (7,8) else at+1

        for _ in range(count):
            frame = self.data[pos]; pos += 1
            locals_positions = []
            if frame <= 63:
                delta = frame
            elif frame <= 127:
                delta = frame-64; pos = verification_type(pos)
            else:
                delta = self.u2(pos); pos += 2
                if frame == 247:
                    pos = verification_type(pos)
                elif frame in (248,249,250,251):
                    pass
                elif frame in (252,253,254):
                    for _ in range(frame-251): pos = verification_type(pos)
                elif frame == 255:
                    n = self.u2(pos); pos += 2
                    for _ in range(n):
                        locals_positions.append(pos); pos = verification_type(pos)
                    n = self.u2(pos); pos += 2
                    for _ in range(n): pos = verification_type(pos)
                else:
                    raise ValueError('unsupported stack map frame')
            current += delta+1
            if current == frame_offset:
                if frame != 255 or len(locals_positions) != 3 or slot != 1:
                    raise ValueError('unexpected local layout')
                where = locals_positions[slot]
                if self.data[where] != 0 or self.data[locals_positions[2]] != 1:
                    raise ValueError('unexpected verification types')
                new = bytearray(self.data); new[where] = 1
                return bytes(new), {'frame_code_offset':current, 'local_slot':slot,
                                    'class_file_offset':where,'before':'Top','after':'Integer'}
        raise ValueError('required stack map frame not found')


def patch_visibility(data, expected):
    if sha(data) != expected: raise ValueError('unrecognized AUTO widget class')
    return ClassFile(data).replace_before_call(
        'initNodeWidgets', '(Lde/vw/mib/view/internal/tree/CloneData;)V', 369, 0x03, 0x04,
        'de/vw/mib/introspection/WidgetParamsIterator', 'setZ')


def patch_selection(data):
    if sha(data) != GENERAL_SHA256: raise ValueError('unrecognized system class')
    new, detail = ClassFile(data).replace_before_call(
        'Oq', '(I)V', 155, 0x1c, 0x1b,
        'de/vw/mib/asl/internal/system/clientapi/SystemBaseAPIServiceImpl', 'updateDayNightMode')
    # The original compiler discards parameter 1 from its verification frame after
    # the last original use. Keeping that parameter live requires this metadata fix.
    new, stack_map = ClassFile(new).preserve_integer_local('Oq','(I)V',144,1)
    detail['stack_map_fix'] = stack_map
    detail['expected_changed_bytes'] = 2
    return new, detail


def rewrite_jar(source, output, expected_hash, mutators):
    source, output = Path(source), Path(output)
    if sha(source.read_bytes()) != expected_hash:
        raise ValueError('source jar differs from inspected S2995 file')
    if output.exists() or source.resolve() == output.resolve():
        raise ValueError('output must be a new candidate file')
    changes = []
    with zipfile.ZipFile(source) as zin:
        names = zin.namelist()
        if len(names) != len(set(names)): raise ValueError('duplicate ZIP entries')
        if any(n.upper().startswith('META-INF/') and n.upper().endswith(('.SF', '.RSA', '.DSA', '.EC')) for n in names):
            raise ValueError('signed jar requires a separate signature review')
        if not set(mutators).issubset(names): raise ValueError('missing target classes')
        prepared = {}
        for name, mutate in mutators.items():
            old = zin.read(name); new, detail = mutate(old)
            count = sum(a != b for a,b in zip(old,new))
            if len(old) != len(new) or count != detail.pop('expected_changed_bytes',1):
                raise ValueError('unexpected class modification count')
            detail.update({'entry':name,'original_sha256':sha(old),'candidate_sha256':sha(new),'changed_bytes':count})
            prepared[name] = new; changes.append(detail)
        with output.open('xb') as stream:
            with zipfile.ZipFile(stream, 'w') as zout:
                zout.comment = zin.comment
                for info in zin.infolist():
                    zout.writestr(info, prepared[info.filename] if info.filename in prepared else zin.read(info.filename))
    with zipfile.ZipFile(output) as candidate, zipfile.ZipFile(source) as original:
        if candidate.testzip() is not None: raise ValueError('candidate CRC failure')
        if original.namelist() != candidate.namelist(): raise ValueError('entry order changed')
        actual = {n for n in original.namelist() if original.read(n) != candidate.read(n)}
        if actual != set(mutators): raise ValueError('unexpected modified entries')
    return {'source_sha256':expected_hash,'candidate_sha256':sha(output.read_bytes()),'changes':changes}


def build(view_jar, output, main_jar=None, selection_fix=False):
    output = Path(output)
    if output.exists(): raise ValueError('use a new output directory')
    if sha(Path(view_jar).read_bytes()) != VIEW_JAR_SHA256: raise ValueError('unrecognized viewhandler.jar')
    pin = json.loads((Path(__file__).resolve().parents[1]/'firmware.json').read_text())
    if selection_fix and (main_jar is None or sha(Path(main_jar).read_bytes()) != pin['hmi_jar_sha256']):
        raise ValueError('selection fix requires the inspected original HMI main jar')
    output.mkdir(parents=True)
    report = {'vehicle_validated':False,'installed':False,'scope':'native control-center AUTO menu',
              'visibility_reason':'AUTO root ListItem visible=false in both layouts; day and night are true'}
    report['viewhandler'] = rewrite_jar(view_jar, output/'viewhandler.candidate.jar', VIEW_JAR_SHA256,
        {n: (lambda data, expected=h: patch_visibility(data,expected)) for n,h in VISIBILITY.items()})
    if selection_fix:
        report['selection_fix'] = rewrite_jar(main_jar,output/'MIBHMI-STD.candidate.jar',pin['hmi_jar_sha256'],{GENERAL:patch_selection})
        report['selection_fix']['reason'] = 'Publish requested 0/1/2 as Mode; keep effective DayNightValue and HAS state unchanged.'
    (output/'patch-report.json').write_text(json.dumps(report,indent=2)+'\n')
    return report


if __name__ == '__main__':
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--view-jar', required=True)
    p.add_argument('--output', required=True)
    p.add_argument('--main-jar')
    p.add_argument('--include-selection-fix', action='store_true')
    a = p.parse_args()
    print(json.dumps(build(a.view_jar,a.output,a.main_jar,a.include_selection_fix),indent=2))

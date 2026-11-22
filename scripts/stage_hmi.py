"""Prepare reviewable files from a supplied live HMI configuration; never deploy."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import secrets
import shutil
import zipfile

ACTIVATOR = "lab.vw.daynight.Activator"


def patch_bundles(text):
    if ACTIVATOR in text:
        raise ValueError("bridge already registered; inspect existing installation")
    ids = set(int(x) for x in re.findall(r"^\s*Bundle\.(?:Name|Activator)\.(\d+)\s*=", text, re.M))
    # Include startup references as well as named bundles when checking collisions.
    for value in re.findall(r"^\s*Bundle\.Startup\.[^=]+=(.*)$", text, re.M):
        ids.update(int(x) for x in re.findall(r"\d+", value.split("#")[0]))
    slot = next((i for i in range(80, 90) if i not in ids), None)
    if slot is None:
        raise ValueError("no free extension slot in 80..89")
    changed = []
    newline = "\r\n" if "\r\n" in text else "\n"
    count = 0
    for line in text.splitlines():
        if re.match(r"^\s*Bundle\.Startup\.Lateload(?:\.[\w.]+)?\s*=", line):
            if "#" in line:
                value, comment = line.split("#", 1)
                line = value.rstrip() + "," + str(slot) + " #" + comment
            else:
                line = line.rstrip() + "," + str(slot)
            count += 1
        changed.append(line)
    if count == 0:
        raise ValueError("no recognized late-load list; no patch generated")
    changed.extend(["", "# VW day/night optional bridge", "Bundle.Name.%d = VWDayNightBridge" % slot,
                    "Bundle.Activator.%d = %s" % (slot, ACTIVATOR)])
    return newline.join(changed) + newline, slot


def stage(bundles, hmi_jar, bridge_jar, output, run_hmi):
    output = Path(output)
    if output.exists():
        raise ValueError("output must be a new directory; preserve prior stage/backup/token")
    root = Path(__file__).resolve().parents[1]
    pinned = root / "firmware.json"
    pin = json.loads(pinned.read_text())
    expected = pin["hmi_jar_sha256"]
    actual = hashlib.sha256(Path(hmi_jar).read_bytes()).hexdigest()
    if actual != expected:
        raise ValueError("HMI jar differs from inspected S2995 build; integration review required")
    startup = Path(run_hmi).read_bytes()
    if hashlib.sha256(startup).hexdigest() != pin["run_hmi_sha256"]:
        raise ValueError("runHMI.sh differs from inspected startup; classpath integration review required")
    with zipfile.ZipFile(bridge_jar) as z:
        names = z.namelist()
        if z.testzip() is not None or "lab/vw/daynight/Activator.class" not in names:
            raise ValueError("bridge jar is incomplete")
        if any(not (n.startswith("lab/") or n.startswith("META-INF/")) for n in names):
            raise ValueError("unexpected classes in bridge jar")
    source = Path(bundles).read_bytes()
    patched, slot = patch_bundles(source.decode("utf-8"))
    output.mkdir(mode=0o700, parents=True)
    (output / "bundles.properties.original").write_bytes(source)
    (output / "runHMI.sh.reference").write_bytes(startup)
    (output / "bundles.properties.candidate").write_bytes(patched.encode("utf-8"))
    shutil.copy2(bridge_jar, output / "vw-daynight-bridge.jar")
    (output / "bridge.token").write_text(secrets.token_hex(32) + "\n")
    (output / "bridge.properties").write_text(
        "enabled=true\nallow_write=false\nport=47851\ntoken_file=/data/vw-daynight/bridge.token\n")
    for p in output.iterdir():
        p.chmod(0o600)
    manifest = {"vehicle_validated": False, "writes_enabled": False, "slot": slot,
                "source_bundles_sha256": hashlib.sha256(source).hexdigest(), "hmi_jar_sha256": actual,
                "run_hmi_sha256": hashlib.sha256(startup).hexdigest(),
                "files": {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in output.iterdir()
                          if p.name != "bridge.token"}}
    (output / "stage.json").write_text(json.dumps(manifest, indent=2) + "\n")
    return manifest


if __name__ == "__main__":
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--bundles", required=True)
    p.add_argument("--hmi-jar", required=True)
    p.add_argument("--bridge-jar", required=True)
    p.add_argument("--run-hmi", required=True)
    p.add_argument("--output", required=True)
    a = p.parse_args()
    print(json.dumps(stage(a.bundles, a.hmi_jar, a.bridge_jar, a.output, a.run_hmi), indent=2))

#!/bin/sh
set -eu
daynight_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
daynight_evidence="$daynight_root/../artifacts/day-night-analysis/extracted/agl-vw_hmi.ext4.sparse.vw_eu"
daynight_jdk=${DAYNIGHT_JDK:-${JAVA_HOME:?Set DAYNIGHT_JDK or JAVA_HOME}}
daynight_go=${DAYNIGHT_GO:-go}
daynight_hmi=${DAYNIGHT_HMI_JAR:-$daynight_evidence/hmi/jar/MIBHMI-STD-HCN1.1.134-bin.jar}
daynight_osgi=${DAYNIGHT_OSGI_JAR:-$daynight_evidence/hmi/lib/osgi.jar}
mkdir -p "$daynight_root/dist/classes" "$daynight_root/dist/test-classes" "$daynight_root/.build-go-cache"
export GOCACHE="$daynight_root/.build-go-cache"
export GOPROXY=off
cd "$daynight_root"
"$daynight_go" test -race ./...
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 "$daynight_go" build -trimpath -ldflags='-s -w' -o dist/daynight-linux-arm64 ./cmd/daynight
"$daynight_go" build -trimpath -o dist/daynight-local ./cmd/daynight
"$daynight_jdk/bin/javac" --release 8 -cp "$daynight_hmi:$daynight_osgi" -d dist/classes hmi/src/lab/vw/daynight/*.java
"$daynight_jdk/bin/jar" cf dist/vw-daynight-bridge.jar -C dist/classes .
"$daynight_jdk/bin/javac" --release 8 -cp dist/classes -d dist/test-classes hmi/test/lab/vw/daynight/*.java
"$daynight_jdk/bin/java" -cp dist/classes:dist/test-classes lab.vw.daynight.BridgeTest
python3 -m unittest discover -s scripts -p 'test_*.py'
shasum -a 256 dist/daynight-linux-arm64 dist/vw-daynight-bridge.jar > dist/SHA256SUMS
printf 'Built offline-tested experimental artifacts. Vehicle integration is not yet validated.\n'

#!/bin/sh
# One-shot offline rollback. Never starts HMI or reboots the headunit.
set -eu
if [ "$#" -ne 2 ]; then
 echo "Usage: sh restore.sh /absolute/backup-directory /absolute/update-tool" >&2
 exit 2
fi
case "$1" in /*) ;; *) echo "Absolute backup path required" >&2; exit 2;; esac
case "$2" in /*) ;; *) echo "Absolute update tool path required" >&2; exit 2;; esac
test -f "$1/manifest.json"
test -f "$1/before.jar"
test -x "$2"
systemctl mask --runtime vw-hmi.service
systemctl stop --no-block vw-hmi.service
attempt=0
while [ "$(systemctl show vw-hmi.service -p ActiveState)" != "ActiveState=inactive" ]; do
 attempt=$((attempt+1))
 if [ "$attempt" -ge 30 ]; then
  echo "HMI did not stop; restore refused. No JAR written." >&2
  exit 1
 fi
 sleep 1
done
"$2" restore "$1"
sync
sha256sum /vw/hmi/Resources/skin1/viewhandler.jar
echo "Previous version restored. Manually reboot the complete headunit; HMI stays stopped until reboot."

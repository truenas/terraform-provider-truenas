#!/usr/bin/env bash
# Put the UPS config back to "never configured" (driver/port empty) on a box
# where the hunt-test fixture set a driver (blazer_usb$625L / port auto).
# ups.update refuses empty driver/port, so this edits the config DB directly:
# boxes must be disposable.
#
# usage: restore-ups.sh <ip>
set -u
# shellcheck source=lib.sh
. "$(dirname "$0")/lib.sh"
if hunt_want_help "$@" || [ $# -ne 1 ]; then hunt_help "$0"; exit 0; fi
ip=$1
ssh -o ConnectTimeout=8 "root@$ip" python3 - <<'PY'
import sqlite3, subprocess, json
con = sqlite3.connect("/data/freenas-v1.db")
con.execute("update services_ups set ups_driver='', ups_port=''")
con.commit(); con.close()
c = json.loads(subprocess.check_output(["midclt", "call", "ups.config"]))
print("ups.config now: driver=%r port=%r" % (c["driver"], c["port"]))
PY

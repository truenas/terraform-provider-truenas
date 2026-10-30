#!/usr/bin/env bash
# Put sshd back the way it was on each box after a failed ssh_config test
# cleanup (echoing read-only host-key fields into ssh.update fails and
# strands sshd on the test port with the test's other values). Originals come
# from the box's own daily config-DB snapshot in the system dataset
# (services_ssh table); only writable keys are sent to ssh.update.
#
# usage: restore-ssh.sh <ip>...
# env (defaults): TFHUNT_SSH_PORT=2222   port sshd is currently stranded on
set -u
# shellcheck source=lib.sh
. "$(dirname "$0")/lib.sh"
if hunt_want_help "$@" || [ $# -lt 1 ]; then hunt_help "$0"; exit 0; fi
port=${TFHUNT_SSH_PORT:-2222}
for ip in "$@"; do
  echo "########## $ip ##########"
  ssh -o ConnectTimeout=8 -p "$port" "root@$ip" bash -s <<'REMOTE'
set -u
db=$(ls -t /var/db/system/configs-*/*/*.db 2>/dev/null | head -1)
echo "from: $db"
payload=$(python3 - "$db" <<'PY'
import sqlite3, sys, json
con = sqlite3.connect(sys.argv[1]); con.row_factory = sqlite3.Row
r = con.execute("select * from services_ssh").fetchone()
def lst(v):
    if v in (None, ""): return []
    try: return json.loads(v)
    except Exception: return [s for s in str(v).split(",") if s]
print(json.dumps({
    "tcpport": r["ssh_tcpport"],
    "passwordauth": bool(r["ssh_passwordauth"]),
    "kerberosauth": bool(r["ssh_kerberosauth"]),
    "tcpfwd": bool(r["ssh_tcpfwd"]),
    "compression": bool(r["ssh_compression"]),
    "options": r["ssh_options"] or "",
    "bindiface": lst(r["ssh_bindiface"]),
    "password_login_groups": lst(r["ssh_password_login_groups"]),
    "sftp_log_level": r["ssh_sftp_log_level"],
    "sftp_log_facility": r["ssh_sftp_log_facility"],
    "weak_ciphers": lst(r["ssh_weak_ciphers"]),
}))
PY
)
echo "restoring: $payload"
midclt call ssh.update "$payload" >/dev/null && echo "ssh.update OK" || echo "ssh.update FAILED"
REMOTE
  sleep 3
  echo -n "sshd on :22 now -> "; ssh -o ConnectTimeout=6 "root@$ip" 'echo ok' 2>&1 | head -1
  echo
done

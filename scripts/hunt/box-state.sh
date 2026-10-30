#!/usr/bin/env bash
# Per-box snapshot of hunt leftovers and restore state: datasets, users,
# groups, shares, cronjobs, iscsi, certs, api keys with hunt prefixes, plus
# sshd port / 2FA / FTP TLS / UPS driver and boot-environment state.
#
# usage: box-state.sh <ip>...
set -u
# shellcheck source=lib.sh
. "$(dirname "$0")/lib.sh"
if hunt_want_help "$@" || [ $# -lt 1 ]; then hunt_help "$0"; exit 0; fi
for ip in "$@"; do
  echo "########## $ip"
  ssh -o ConnectTimeout=8 "root@$ip" python3 - <<'PY'
import json, subprocess
def call(m, *a): return json.loads(subprocess.check_output(["midclt","call",m,*[json.dumps(x) for x in a]]))
pfx = ("tfhunt","tfupg","tfenum","tfimp","tfren","tfrepro","tfacc")
zfs = subprocess.check_output(["zfs","list","-H","-o","name"]).decode().split()
print("  datasets/zvols:", [d for d in zfs if any(p in d for p in pfx)])
print("  users:", [u["username"] for u in call("user.query") if u["username"].startswith(pfx)])
print("  groups:", [g["name"] for g in call("group.query") if g["name"].startswith(pfx)])
print("  smb shares:", [s["name"] for s in call("sharing.smb.query") if s["name"].startswith(pfx)])
print("  nfs shares:", [s["path"] for s in call("sharing.nfs.query") if any(p in s["path"] for p in pfx)])
print("  cronjobs:", [c["description"] for c in call("cronjob.query") if any(p in json.dumps(c) for p in pfx)])
print("  iscsi targets/extents:", [t["name"] for t in call("iscsi.target.query") if t["name"].startswith(pfx)], [e["name"] for e in call("iscsi.extent.query") if e["name"].startswith(pfx)])
print("  certs:", [c["name"] for c in call("certificate.query") if c["name"].startswith(pfx)])
print("  api keys:", [k["name"] for k in call("api_key.query") if k["name"].startswith(pfx)])
print("  sshd port:", call("ssh.config")["tcpport"], "| 2fa:", call("auth.twofactor.config")["enabled"], "| ftp tls:", call("ftp.config")["tls"], "| ups driver:", repr(call("ups.config")["driver"]))
bes = call("boot.environment.query"); print("  booted/activated BE:", [b["id"] for b in bes if b["active"]], [b["id"] for b in bes if b["activated"]], "| leftovers:", [b["id"] for b in bes if "tfhunt" in b["id"]])
PY
done

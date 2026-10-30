#!/usr/bin/env bash
# Run ON a TrueNAS box (build-ro-box.sh copies it there): build the
# READONLY_ADMIN redaction fixtures.
#   - user tfhuntro in truenas_readonly_administrators + API key tf-hunt-ro
#   - one secret-bearing object of each kind the TestHuntRedaction_* /
#     *readonly*_test.go data sources read, all named tfhuntro-*:
#     kerberos keytab, app registry, cloudflare ACME DNS authenticator,
#     Slack alert service, S3 cloudsync credentials, VM tfhuntrovm with a
#     DISPLAY device (ports 5951/5952).
#
# Prints "export ..." lines for env.ro.<ip>.sh on stdout; progress on stderr.
# Idempotent: objects are reused by name. The API key is minted once and
# shown once; an existing key is reset so the printed value is always valid.
#
# usage: TFHUNT_RO_PASSWORD=... TFHUNT_REG_PASSWORD=... ro-fixtures.sh [keytab_b64]
#
# env (required): TFHUNT_RO_PASSWORD   login password for the tfhuntro user
#                 TFHUNT_REG_PASSWORD  password of TFHUNT_REG_USER on the
#                                      registry started by registry-setup.sh
# env (defaults): TFHUNT_REG_USER=tfhuntreg TFHUNT_REG_PORT=9322
# The dummy secrets stored inside the fixture objects (ACME api key, Slack
# URL, S3 key, display password) are random per run; the tests only check
# that they come back redacted.
set -u
if [ "${1:-}" = -h ] || [ "${1:-}" = --help ]; then
  sed -n '2,/^[^#]/p' "$0" | grep '^#' | sed -E 's/^# ?//'; exit 0
fi
: "${TFHUNT_RO_PASSWORD:?set TFHUNT_RO_PASSWORD}"
: "${TFHUNT_REG_PASSWORD:?set TFHUNT_REG_PASSWORD}"
REG_USER=${TFHUNT_REG_USER:-tfhuntreg}; REG_PORT=${TFHUNT_REG_PORT:-9322}
KEYTAB_B64="${1:-}"

q() { midclt call "$@"; }
idof() { python3 -c 'import json,sys; r=json.load(sys.stdin); print(r[0]["id"] if isinstance(r,list) and r else (r.get("id","") if isinstance(r,dict) else ""))'; }
rnd() { tr -dc a-z0-9 </dev/urandom | head -c 20; }
jstr() { python3 -c 'import json,sys; print(json.dumps(sys.argv[1]))' "$1"; }

# --- read-only admin user + key
GID=$(q group.query '[["name","=","truenas_readonly_administrators"]]' | idof)
if [ -z "$GID" ]; then echo "no truenas_readonly_administrators group" >&2; q group.query '[["name","~","readonly"]]'; exit 1; fi
UID_=$(q user.query '[["username","=","tfhuntro"]]' | idof)
if [ -z "$UID_" ]; then
  UID_=$(q user.create "{\"username\":\"tfhuntro\",\"full_name\":\"tf hunt readonly\",\"group\":$GID,\"password\":$(jstr "$TFHUNT_RO_PASSWORD"),\"home\":\"/var/empty\",\"shell\":\"/usr/sbin/nologin\",\"smb\":false}" | idof)
fi
echo "# user tfhuntro id=$UID_ group=$GID" >&2
KEYID=$(q api_key.query '[["name","=","tf-hunt-ro"]]' | idof)
if [ -n "$KEYID" ]; then
  echo "# api key tf-hunt-ro already exists (id $KEYID); value not recoverable, resetting" >&2
  q api_key.update "$KEYID" '{"reset":true}' | python3 -c 'import json,sys; print("export TRUENAS_RO_API_KEY=%s" % json.load(sys.stdin)["key"])'
else
  q api_key.create '{"name":"tf-hunt-ro","username":"tfhuntro"}' | python3 -c 'import json,sys; print("export TRUENAS_RO_API_KEY=%s" % json.load(sys.stdin)["key"])'
fi
echo "export TRUENAS_RO_USERNAME=tfhuntro"

# --- fixtures
ensure() { # ensure <method-prefix> <name> <create-json>
  local id; id=$(q "$1.query" "[[\"name\",\"=\",\"$2\"]]" | idof)
  if [ -z "$id" ]; then id=$(q "$1.create" "$3" | idof); fi
  [ -z "$id" ] && echo "# FAILED $1 $2" >&2
  echo "$id"
}
if [ -n "$KEYTAB_B64" ]; then
  ensure kerberos.keytab tfhuntro-keytab "{\"name\":\"tfhuntro-keytab\",\"file\":\"$KEYTAB_B64\"}" >/dev/null
  echo "export TRUENAS_RO_KEYTAB=tfhuntro-keytab"
fi
# real basic-auth registry:2 on this box (registry-setup.sh); app.registry.create logs in to validate
REGID=$(ensure app.registry tfhuntro-registry "{\"name\":\"tfhuntro-registry\",\"username\":\"$REG_USER\",\"password\":$(jstr "$TFHUNT_REG_PASSWORD"),\"uri\":\"http://127.0.0.1:$REG_PORT/\"}")
echo "export TRUENAS_RO_REGISTRY=tfhuntro-registry"
echo "export TFHUNT_REG_ID=$REGID"
ensure acme.dns.authenticator tfhuntro-acme "{\"name\":\"tfhuntro-acme\",\"attributes\":{\"authenticator\":\"cloudflare\",\"cloudflare_email\":\"tfhunt@tfhunt.invalid\",\"api_key\":\"$(rnd)\"}}" >/dev/null
echo "export TRUENAS_RO_ACME=tfhuntro-acme"
ensure alertservice tfhuntro-alert "{\"name\":\"tfhuntro-alert\",\"attributes\":{\"type\":\"Slack\",\"url\":\"https://hooks.slack.com/services/T000/B000/$(rnd)\"},\"level\":\"WARNING\",\"enabled\":false}" >/dev/null
echo "export TRUENAS_RO_ALERT=tfhuntro-alert"
ensure cloudsync.credentials tfhuntro-s3 "{\"name\":\"tfhuntro-s3\",\"provider\":{\"type\":\"S3\",\"access_key_id\":\"tfhuntroaccess\",\"secret_access_key\":\"$(rnd)\"}}" >/dev/null
echo "export TRUENAS_RO_CLOUDSYNC=tfhuntro-s3"
VMID=$(q vm.query '[["name","=","tfhuntrovm"]]' | idof)
if [ -z "$VMID" ]; then VMID=$(q vm.create '{"name":"tfhuntrovm","memory":512,"autostart":false}' | idof); fi
DEVID=$(q vm.device.query "[[\"vm\",\"=\",$VMID],[\"attributes.dtype\",\"=\",\"DISPLAY\"]]" | idof)
if [ -z "$DEVID" ]; then DEVID=$(q vm.device.create "{\"vm\":$VMID,\"attributes\":{\"dtype\":\"DISPLAY\",\"password\":\"$(rnd)\",\"bind\":\"0.0.0.0\",\"web\":true,\"port\":5951,\"web_port\":5952}}" | idof); fi
echo "export TRUENAS_RO_VMDEVICE=$DEVID"

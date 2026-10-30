#!/usr/bin/env bash
# Build the registry + READONLY_ADMIN fixtures on one box over ssh and save
# the resulting env.ro.<ip>.sh into the env dir. Runs registry-setup.sh and
# ro-fixtures.sh on the box (copied to /tmp there).
#
# usage: build-ro-box.sh <ip>
#
# env (required): TFHUNT_RO_PASSWORD TFHUNT_REG_PASSWORD TFHUNT_REG_PASSWORD2
#   (the registry passwords must match TRUENAS_TEST_REGISTRY_PASSWORD(2) in
#   env.registry.sh, which the acceptance tests read)
# env (optional): TFHUNT_REG_USER TFHUNT_REG_USER2 TFHUNT_REG_PORT TFHUNT_REG_PORT2
#                 TRUENAS_DS_KEYTAB_B64 (or env.keytab.sh in the env dir): adds
#                 the kerberos keytab fixture
# Needs root ssh to the box. The passwords travel in the remote command line
# (visible in `ps` on the box for the duration): fine for disposable boxes.
set -u
# shellcheck source=lib.sh
. "$(dirname "$0")/lib.sh"
if hunt_want_help "$@" || [ $# -ne 1 ]; then hunt_help "$0"; exit 0; fi
ip=$1
hunt_require TFHUNT_RO_PASSWORD TFHUNT_REG_PASSWORD TFHUNT_REG_PASSWORD2 || exit 1
D=$(hunt_env_dir); mkdir -p "$D"
# shellcheck disable=SC1091
[ -f "$D/env.keytab.sh" ] && . "$D/env.keytab.sh"

here=$(dirname "$0")
scp -q "$here/registry-setup.sh" "$here/ro-fixtures.sh" "root@$ip:/tmp/" || exit 1
remote_env=$(printf 'export TFHUNT_RO_PASSWORD=%q TFHUNT_REG_PASSWORD=%q TFHUNT_REG_PASSWORD2=%q' \
  "$TFHUNT_RO_PASSWORD" "$TFHUNT_REG_PASSWORD" "$TFHUNT_REG_PASSWORD2")
for v in TFHUNT_REG_USER TFHUNT_REG_USER2 TFHUNT_REG_PORT TFHUNT_REG_PORT2; do
  [ -n "${!v:-}" ] && remote_env+=$(printf ' %s=%q' "$v" "${!v}")
done
# shellcheck disable=SC2029  # the passwords are meant to expand here, client side
ssh "root@$ip" "$remote_env; bash /tmp/registry-setup.sh" 2>&1 | tail -4
# shellcheck disable=SC2029
ssh "root@$ip" "$remote_env; bash /tmp/ro-fixtures.sh ${TRUENAS_DS_KEYTAB_B64:-}" \
  2> >(grep -v Traceback | grep -v '^ ' >&2) > "$D/env.ro.$ip.sh"
chmod 600 "$D/env.ro.$ip.sh"
echo "wrote $D/env.ro.$ip.sh:"
sed 's/\(API_KEY=\).*/\1<redacted>/' "$D/env.ro.$ip.sh"

#!/usr/bin/env bash
# Run ON a TrueNAS box (build-ro-box.sh copies it there): start two basic-auth
# docker registries (registry:2) as app.registry fixtures.
#
# app.registry.create does a real `docker login`, and docker only speaks plain
# HTTP to loopback registries, so each box runs its own and the test URI is
# http://127.0.0.1:<port>/. TrueNAS requires every app.registry uri to be
# unique, so two instances run: the first port holds the persistent
# READONLY_ADMIN fixture entry, the second is for the acceptance/hunt tests.
# Two accounts share one htpasswd: tests that rotate password_wo need a
# second valid login (registry:2 reloads htpasswd on change).
#
# usage: TFHUNT_REG_PASSWORD=... TFHUNT_REG_PASSWORD2=... registry-setup.sh
#
# env (required): TFHUNT_REG_PASSWORD TFHUNT_REG_PASSWORD2
# env (defaults): TFHUNT_REG_USER=tfhuntreg TFHUNT_REG_USER2=tfhuntreg2
#                 TFHUNT_REG_PORT=9322 TFHUNT_REG_PORT2=9323
# Data lives under /mnt/<docker pool>/tfhunt-registry.
set -u
if [ "${1:-}" = -h ] || [ "${1:-}" = --help ]; then
  sed -n '2,/^[^#]/p' "$0" | grep '^#' | sed -E 's/^# ?//'; exit 0
fi
: "${TFHUNT_REG_PASSWORD:?set TFHUNT_REG_PASSWORD}"
: "${TFHUNT_REG_PASSWORD2:?set TFHUNT_REG_PASSWORD2}"
U1=${TFHUNT_REG_USER:-tfhuntreg}; U2=${TFHUNT_REG_USER2:-tfhuntreg2}
P1=${TFHUNT_REG_PORT:-9322}; P2=${TFHUNT_REG_PORT2:-9323}

POOL=$(midclt call docker.config | python3 -c 'import json,sys; print(json.load(sys.stdin)["pool"])')
if [ -z "$POOL" ] || [ "$POOL" = None ]; then echo "docker pool not configured (docker.update pool=...)" >&2; exit 1; fi
D=/mnt/$POOL/tfhunt-registry
mkdir -p "$D/auth" "$D/data"
{ docker run --rm --entrypoint htpasswd httpd:2 -Bbn "$U1" "$TFHUNT_REG_PASSWORD" && \
  docker run --rm --entrypoint htpasswd httpd:2 -Bbn "$U2" "$TFHUNT_REG_PASSWORD2"; } > "$D/auth/htpasswd.new" || exit 1
mv "$D/auth/htpasswd.new" "$D/auth/htpasswd"

start_registry() { # <container name> <host port> <extra docker args...>
  local name=$1 port=$2; shift 2
  if ! docker ps -a --format '{{.Names}}' | grep -qx "$name"; then
    docker run -d --name "$name" --restart unless-stopped -p "$port:5000" \
      -v "$D/auth:/auth" "$@" \
      -e REGISTRY_AUTH=htpasswd -e REGISTRY_AUTH_HTPASSWD_REALM=tfhunt -e REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd \
      registry:2 || exit 1
  else
    docker start "$name" >/dev/null
  fi
}
start_registry tfhunt-registry "$P1" -v "$D/data:/var/lib/registry"
start_registry tfhunt-registry2 "$P2"

sleep 2
docker ps --filter name=tfhunt-registry --format '{{.Names}} {{.Status}} {{.Ports}}'
curl -s -o /dev/null -w 'anon=%{http_code}\n' "http://127.0.0.1:$P1/v2/"
curl -s -o /dev/null -w 'auth=%{http_code}\n' -u "$U1:$TFHUNT_REG_PASSWORD" "http://127.0.0.1:$P1/v2/"

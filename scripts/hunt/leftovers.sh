#!/usr/bin/env bash
# List acceptance/hunt test leftovers on a box (anything tf-acc/tfhunt/tfact
# named) so they can be deleted by hand. The intended persistent fixtures
# (tfhuntro user, tfhuntrovm, tfhuntro-registry, ...) show up too and are
# labelled.
#
# usage: leftovers.sh <ip>        (runs the queries on the box over ssh)
#        leftovers.sh --local     (run ON the box)
set -u
if [ "${1:-}" = -h ] || [ "${1:-}" = --help ] || [ $# -ne 1 ]; then
  sed -n '2,/^[^#]/p' "$0" | grep '^#' | sed -E 's/^# ?//'; exit 0
fi
if [ "$1" != --local ]; then
  ssh "root@$1" bash -s -- --local < "$0"
  exit $?
fi
py='import json,sys; [print("  ", x.get("id"), x.get("name") or x.get("username") or x.get("path")) for x in json.load(sys.stdin)]'
echo "## datasets"; midclt call pool.dataset.query '[["name","rin","tf"]]' '{"select":["id","name"]}' | python3 -c "$py" | grep -i 'tf-acc\|tfhunt\|tfact' | grep -v ix-apps
echo "## users (fixture: tfhuntro)"; midclt call user.query '[["username","^","tf"]]' | python3 -c "$py"
echo "## groups"; midclt call group.query '[["name","^","tf"]]' | python3 -c "$py"
echo "## vms (fixture: tfhuntrovm)"; midclt call vm.query '[["name","^","tf"]]' | python3 -c "$py"
echo "## iscsi auth"; midclt call iscsi.auth.query | python3 -c 'import json,sys; [print("  ", a["id"], a["tag"], a["user"], a["discovery_auth"]) for a in json.load(sys.stdin)]'
echo "## replication"; midclt call replication.query | python3 -c "$py"
echo "## keychain"; midclt call keychaincredential.query | python3 -c "$py"
echo "## shares nfs/smb"; midclt call sharing.nfs.query | python3 -c "$py"; midclt call sharing.smb.query | python3 -c "$py"
echo "## app registry (fixture: tfhuntro-registry)"; midclt call app.registry.query | python3 -c "$py"
echo "## cronjobs"; midclt call cronjob.query | python3 -c 'import json,sys; [print("  ", c["id"], c["description"]) for c in json.load(sys.stdin)]'

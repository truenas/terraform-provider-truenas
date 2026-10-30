#!/usr/bin/env bash
# Delete leftover hunt-test VMs (tfhuntvmd*) on a box; keeps the tfhuntrovm
# fixture. A failed vm_device test with ImportStatePersist leaves them behind
# ("This name already exists" on the next run).
#
# usage: clean-hunt-vms.sh <ip>
set -u
# shellcheck source=lib.sh
. "$(dirname "$0")/lib.sh"
if hunt_want_help "$@" || [ $# -ne 1 ]; then hunt_help "$0"; exit 0; fi
ssh "root@$1" "midclt call vm.query '[[\"name\",\"^\",\"tfhuntvmd\"]]' | python3 -c 'import json,sys; [print(v[\"id\"]) for v in json.load(sys.stdin)]' | while read -r id; do echo \"delete vm \$id\"; midclt call vm.delete \"\$id\" '{\"zvols\":false,\"force\":true}'; echo; done"

#!/usr/bin/env bash
# Dump a box's full API method schema (core.get_methods) to
# <work dir>/schema/methods-<ip>.json. Input for enum/enum-sweep.py and
# schema/drift.py.
#
# usage: dump-methods.sh <ip>...
set -u
# shellcheck source=lib.sh
. "$(dirname "$0")/lib.sh"
if hunt_want_help "$@" || [ $# -lt 1 ]; then hunt_help "$0"; exit 0; fi
out="$(hunt_work_dir)/schema"; mkdir -p "$out"
for ip in "$@"; do
  ssh -o ConnectTimeout=8 "root@$ip" midclt call core.get_methods > "$out/methods-$ip.json" || exit 1
  echo "$out/methods-$ip.json: $(python3 -c 'import json,sys; print(len(json.load(open(sys.argv[1]))))' "$out/methods-$ip.json") methods"
done

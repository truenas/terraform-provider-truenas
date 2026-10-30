#!/usr/bin/env bash
# Run acceptance tests against one TrueNAS box with every hunt fixture env
# sourced and every opt-in gate set, then print the end-of-run SKIPPED list.
#
# usage: run-box.sh <ip> <logfile> <go test args...>
#   e.g. run-box.sh 192.0.2.68 user.log ./internal/resources/user/ -run TestAcc
#        run-box.sh 192.0.2.68 all.log ./internal/...
#
# Reads env files from $TFHUNT_ENV_DIR (default <repo>/.hunt-env), see
# lib.sh / README.md. Sets TF_ACC=1 TRUENAS_DISRUPTIVE=1 and the
# TRUENAS_TEST_SSH_CONFIG / TRUENAS_TEST_SYSTEM_GENERAL /
# TRUENAS_TEST_TRUECOMMAND_ENABLE gates: the target must be disposable.
#
# RO=1   run the whole provider as the READONLY_ADMIN key from env.ro.<ip>.sh
#        (TestAccAppRegistry_readonlyRedacted and friends).
#
# Output: go test -v goes to <logfile>; stdout gets exit code, the
# PASS/FAIL/ok lines, a passed= count, and a SKIPPED block with one line per
# skipped test and the reason the test printed. Each line is tagged
# [accepted dead end] when the reason matches the ACCEPTED pattern below
# (features this lab cannot supply, and version gates on the box they skip
# on), else [UNBUILT FIXTURE / STALE GATE]. Any unexpected skip exits 3.
set -u
# shellcheck source=lib.sh
. "$(dirname "$0")/lib.sh"
if hunt_want_help "$@" || [ $# -lt 3 ]; then hunt_help "$0"; exit 0; fi

IP=$1; LOG=$2; shift 2
hunt_source_env "$IP" || exit 1

if [ "${RO:-}" = 1 ]; then
  hunt_require TRUENAS_RO_API_KEY TRUENAS_RO_USERNAME TFHUNT_REG_ID || exit 1
  export TRUENAS_API_KEY=$TRUENAS_RO_API_KEY TRUENAS_USERNAME=$TRUENAS_RO_USERNAME
  export TRUENAS_TEST_READONLY_REGISTRY_NAME=${TRUENAS_RO_REGISTRY:-tfhuntro-registry} TRUENAS_TEST_READONLY_REGISTRY_ID=$TFHUNT_REG_ID
fi
export TF_ACC=1 TRUENAS_DISRUPTIVE=1 TRUENAS_TEST_SSH_CONFIG=1 TRUENAS_TEST_SYSTEM_GENERAL=1 TRUENAS_TEST_TRUECOMMAND_ENABLE=1

cd "$(hunt_repo_root)" || exit 1
go test -count=1 -timeout 90m -p 1 -v "$@" > "$LOG" 2>&1
echo "exit=$?"
grep -E '^(--- FAIL|ok|FAIL|PASS)' "$LOG"
grep -c -- '--- PASS' "$LOG" | sed 's/^/passed=/'

# Every run ends with the skip list: test name + the reason the test printed.
# Accepted dead ends (lab cannot supply them; see README.md "Accepted dead
# ends") are marked; anything else is an unbuilt fixture or a stale gate and
# fails the run. Version gates ("below TrueNAS", "from TrueNAS 28.0", "needs
# TrueNAS < 26.0") are dead ends on the box they skip on.
ACCEPTED='failover|vmware|enclosure|tn_connect|TrueNASConnect|Enterprise|ds_auth|container|not available on this release|requires TrueNAS|accepted from TrueNAS|below TrueNAS|needs TrueNAS <|from TrueNAS [0-9]'
skip_lines() {
  grep -B1 -- '--- SKIP' "$LOG" | grep -v -- '^--$' | paste - - \
    | sed -E 's/^\s+[a-z_0-9]+\.go:[0-9]+: (.*)\t--- SKIP: (\S+).*/  \2: \1/'
}
echo "SKIPPED ($(grep -c -- '--- SKIP' "$LOG")):"
skip_lines | while IFS= read -r line; do
  if echo "$line" | grep -Eq "$ACCEPTED"; then
    echo "$line  [accepted dead end]"
  else
    echo "$line  [UNBUILT FIXTURE / STALE GATE]"
  fi
done
if grep -q -- '--- SKIP' "$LOG" && skip_lines | grep -Evq "$ACCEPTED"; then
  echo "UNEXPECTED SKIPS PRESENT"; exit 3
fi

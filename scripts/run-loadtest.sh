#!/usr/bin/env bash
# Run the TrueNAS provider load tests against a DISPOSABLE box.
#
# Usage:
#   scripts/run-loadtest.sh [all|client|tf|callsat|authburst|jobsat|mixed|sweep]
# Default target is "all".
#
# Configuration comes from the environment (no secrets live in this repo).
# Set these first, or put them in a git-ignored .load-test.env at the repo
# root (this script sources it if present):
#
#   TRUENAS_ENDPOINT   wss://<disposable-box>/api/current
#   TRUENAS_API_KEY    an API key on that box
#   TRUENAS_USERNAME   the key's owner (enables SCRAM on 26.0+)
#   TRUENAS_TEST_POOL  pool for tf-load- objects (default: tank)
#
# The box MUST be disposable: these tests create/destroy many objects and
# deliberately trip TrueNAS rate/concurrency limits. This script sets the
# TRUENAS_LOAD gate and points the allowed-endpoint guard at TRUENAS_ENDPOINT,
# so LoadCheck cannot run against any other box.
#
# Optional scale overrides (defaults in parentheses):
#   LOAD_AUTH_CONNS(40) LOAD_CALL_CONCURRENCY(50) LOAD_JOB_COUNT(30)
#   LOAD_RESOURCES(150) LOAD_MIXED_PER_TYPE(10) LOAD_PARALLELISM(20)
#   LOAD_CONCURRENT_APPLIES(6) LOAD_DURATION(10m)
#
# The tf/mixed targets need the `terraform` binary on PATH.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

# Optional local config, git-ignored (see .gitignore).
if [ -f .load-test.env ]; then
  # shellcheck disable=SC1091
  . ./.load-test.env
fi

: "${TRUENAS_ENDPOINT:?set TRUENAS_ENDPOINT (or add it to .load-test.env)}"
: "${TRUENAS_API_KEY:?set TRUENAS_API_KEY (or add it to .load-test.env)}"
: "${TRUENAS_USERNAME:?set TRUENAS_USERNAME (or add it to .load-test.env)}"

# Export so the make / go test child processes inherit them, whether they
# came from the environment or from a sourced .load-test.env without `export`.
export TRUENAS_ENDPOINT TRUENAS_API_KEY TRUENAS_USERNAME
export TF_ACC=1
export TRUENAS_LOAD=1
export TRUENAS_LOAD_ALLOWED_ENDPOINT="$TRUENAS_ENDPOINT"
export TRUENAS_TEST_POOL="${TRUENAS_TEST_POOL:-tank}"

TARGET="${1:-all}"
echo "Target: $TARGET   Box: $TRUENAS_ENDPOINT   Pool: $TRUENAS_TEST_POOL"

client_one() { go test ./internal/client/ -run "^$1\$" -v -count=1 -timeout 20m; }

case "$TARGET" in
  all)       make loadtest ;;
  client)    make loadtest-client ;;   # auth-burst + call-saturation + job-saturation
  tf)        make loadtest-tf ;;        # terraform apply / concurrent / sustained / mixed
  sweep)     make loadtest-sweep ;;     # delete stranded tf-load- objects
  callsat)   client_one TestLoad_CallSaturation ;;   # concurrency cap
  authburst) client_one TestLoad_AuthBurst ;;        # auth rate limit
  jobsat)    client_one TestLoad_JobSaturation ;;     # job-heavy load
  mixed)     go test ./test/load/ -run '^TestLoad_MixedResources$' -v -count=1 -timeout 30m ;;
  *) echo "unknown target: $TARGET (use all|client|tf|callsat|authburst|jobsat|mixed|sweep)" >&2; exit 1 ;;
esac

echo
echo "Reports (Markdown + JSON) under the package results/ dirs:"
find internal/client/results test/load/results -name 'load-report-*' 2>/dev/null | sort | tail -8 || true

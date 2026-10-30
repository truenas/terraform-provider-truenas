#!/usr/bin/env bash
# Summarize skipped tests across a set of `go test -v` logs: one line per
# distinct test+reason, followed by the logs it appeared in.
#
# usage: skips-report.sh <logs...>
#   e.g. skips-report.sh .hunt-work/all-*.log
set -u
# shellcheck source=lib.sh
. "$(dirname "$0")/lib.sh"
if hunt_want_help "$@" || [ $# -lt 1 ]; then hunt_help "$0"; exit 0; fi

for LOG in "$@"; do
  b=$(basename "$LOG")
  grep -B1 -- '--- SKIP' "$LOG" | grep -v -- '^--$' | paste - - \
    | sed -E 's/^\s+[a-z_0-9]+\.go:[0-9]+: (.*)\t--- SKIP: (\S+).*/\2\t\1/' \
    | sed "s#\$#\t$b#"
done | sort | awk -F'\t' '{k=$1"\t"$2; logs[k]=logs[k] (logs[k]?",":"") $3} END {for (k in logs) print k "\t[" logs[k] "]"}' | sort

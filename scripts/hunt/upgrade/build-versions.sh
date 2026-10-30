#!/usr/bin/env bash
# Build provider binaries for old release tags (via detached git worktrees)
# and for the current working tree, into
# <work dir>/upgrade/bin/<version>/terraform-provider-truenas.
# "current" is always built; tags are optional.
#
# usage: build-versions.sh [tag...]      e.g. build-versions.sh v1.5.24 v1.6.0
set -u
# shellcheck source-path=SCRIPTDIR
# shellcheck source=../lib.sh
. "$(dirname "$0")/../lib.sh"
if hunt_want_help "$@"; then hunt_help "$0"; exit 0; fi
UP="$(hunt_work_dir)/upgrade"
cd "$(hunt_repo_root)" || exit 1
for tag in "$@"; do
  wt="$UP/wt-$tag"
  [ -d "$wt" ] || git worktree add --detach "$wt" "$tag" >/dev/null 2>&1 || { echo "worktree $tag FAILED"; continue; }
  mkdir -p "$UP/bin/$tag"
  (cd "$wt" && go build -o "$UP/bin/$tag/terraform-provider-truenas" . ) && echo "built $tag" || echo "build $tag FAILED"
done
mkdir -p "$UP/bin/current"
go build -o "$UP/bin/current/terraform-provider-truenas" . && echo "built current ($(git rev-parse --short HEAD), $(git status --porcelain | wc -l) dirty files)" || echo "build current FAILED"
ls -la "$UP"/bin/*/terraform-provider-truenas

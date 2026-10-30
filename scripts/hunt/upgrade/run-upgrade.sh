#!/usr/bin/env bash
# Old-state upgrade probe: does state written by an OLD provider build
# plan clean under the CURRENT build?
#   1. apply main.tf with the old build (dev_overrides)
#   2. switch dev_overrides to the current build, `terraform plan -detailed-exitcode`
#      exit 0 = clean upgrade, 2 = diff (FINDING), 1 = error (FINDING)
#   3. destroy with the current build (fall back to the old one if that fails)
# Binaries come from build-versions.sh; env from env.<ip>.sh.
#
# usage: run-upgrade.sh <old-version> <ip>     e.g. run-upgrade.sh v1.6.0 192.0.2.68
# env (optional): TFHUNT_UPGRADE_TF  alternative .tf to probe (default main.tf here)
set -u
# shellcheck source-path=SCRIPTDIR
# shellcheck source=../lib.sh
. "$(dirname "$0")/../lib.sh"
if hunt_want_help "$@" || [ $# -ne 2 ]; then hunt_help "$0"; exit 0; fi
old=$1; ip=$2
UP="$(hunt_work_dir)/upgrade"
here=$(cd "$(dirname "$0")" && pwd)
hunt_source_env "$ip" || exit 1
hunt_require TRUENAS_ENDPOINT TRUENAS_API_KEY TRUENAS_TEST_POOL || exit 1
for v in "$old" current; do
  [ -x "$UP/bin/$v/terraform-provider-truenas" ] || { echo "no build for $v: run build-versions.sh $old"; exit 1; }
done
work="$UP/run-$old-$ip"; rm -rf "$work"; mkdir -p "$work"; cp "${TFHUNT_UPGRADE_TF:-$here/main.tf}" "$work/main.tf"
cd "$work" || exit 1
export TF_IN_AUTOMATION=1 TF_INPUT=0
export TF_VAR_endpoint="$TRUENAS_ENDPOINT" TF_VAR_api_key="$TRUENAS_API_KEY" TF_VAR_pool="$TRUENAS_TEST_POOL" TF_VAR_suffix="${old//[.v]/}$RANDOM"
TF_VAR_probe_password="Tf-$(tr -dc a-zA-Z0-9 </dev/urandom | head -c 16)-1"; export TF_VAR_probe_password

override() { # <version>
  cat > "$work/dev.tfrc" <<EOF
provider_installation {
  dev_overrides { "truenas/truenas" = "$UP/bin/$1" }
  direct {}
}
EOF
  export TF_CLI_CONFIG_FILE="$work/dev.tfrc"
}

echo "########## $old -> current on $ip (suffix $TF_VAR_suffix) ##########"
override "$old"
if ! terraform apply -auto-approve -no-color >"$work/apply-old.log" 2>&1; then
  echo "APPLY with $old FAILED:"; grep -E "Error|error" -A3 "$work/apply-old.log" | head -20
  override "$old"; terraform destroy -auto-approve -no-color >/dev/null 2>&1
  exit 1
fi
echo "applied with $old: $(grep -cE '^truenas_.*: Creation complete' "$work/apply-old.log") resources"

override current
terraform plan -no-color -detailed-exitcode >"$work/plan-current.log" 2>&1; code=$?
case $code in
  0) echo "UPGRADE PLAN: clean (empty)";;
  2) echo "UPGRADE PLAN: DIFF (finding)"; grep -E '^\s+[~+-/] |will be|must be replaced|forces replacement|^Plan:' "$work/plan-current.log" | head -60;;
  *) echo "UPGRADE PLAN: ERROR (finding)"; grep -E "Error" -A6 "$work/plan-current.log" | head -40;;
esac

override current
if ! terraform destroy -auto-approve -no-color >"$work/destroy.log" 2>&1; then
  echo "destroy with current FAILED, retrying with $old"; override "$old"
  terraform destroy -auto-approve -no-color >>"$work/destroy.log" 2>&1 || { echo "DESTROY FAILED: leftovers with suffix $TF_VAR_suffix"; grep -E "Error" -A3 "$work/destroy.log" | head -10; }
fi
echo "done: logs in $work"
exit "$code"

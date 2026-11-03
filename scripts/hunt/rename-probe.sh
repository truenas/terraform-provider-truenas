#!/usr/bin/env bash
# Data-survival probe for the in-place rename (hunt F8): blocks cannot be
# written to a zvol through the API, so this writes a signature over ssh,
# renames the zvol and a dataset with a file through Terraform (the current
# build, dev_overrides) and checks the signature, file, snapshot and guid
# afterwards. The old names must not back anything on the box (it creates its
# own under <pool>/tfrenprobe-<random>).
#
# usage: rename-probe.sh <ip>          e.g. rename-probe.sh 192.0.2.249
# needs: env.<ip>.sh (TRUENAS_ENDPOINT TRUENAS_API_KEY TRUENAS_TEST_POOL),
#        passwordless ssh as root to <ip>, terraform on PATH.
set -u
# shellcheck source=lib.sh
. "$(dirname "$0")/lib.sh"
if hunt_want_help "$@" || [ $# -ne 1 ]; then hunt_help "$0"; exit 0; fi
ip=$1
hunt_source_env "$ip" || exit 1
hunt_require TRUENAS_ENDPOINT TRUENAS_API_KEY TRUENAS_TEST_POOL || exit 1
pool=$TRUENAS_TEST_POOL
work="$(hunt_work_dir)/rename-probe-$ip"; rm -rf "$work"; mkdir -p "$work/bin"
cd "$(hunt_repo_root)" || exit 1
go build -o "$work/bin/terraform-provider-truenas" . || exit 1
base="$pool/tfrenprobe-$RANDOM"
ssh_() { ssh -o BatchMode=yes -o StrictHostKeyChecking=no "root@$ip" "$@"; }
guid() { ssh_ "zfs get -H -o value guid '$1'" 2>/dev/null; }

cat > "$work/dev.tfrc" <<EOF
provider_installation {
  dev_overrides { "truenas/truenas" = "$work/bin" }
  direct {}
}
EOF
export TF_CLI_CONFIG_FILE="$work/dev.tfrc" TF_IN_AUTOMATION=1 TF_INPUT=0
tf() { # <name>
  cat > "$work/main.tf" <<EOF
terraform {
  required_providers {
    truenas = { source = "truenas/truenas" }
  }
}
provider "truenas" {
  endpoint = "$TRUENAS_ENDPOINT"
  api_key  = "$TRUENAS_API_KEY"
  insecure = true
}
resource "truenas_dataset" "d" {
  name          = "$1"
  force_destroy = true
}
resource "truenas_zvol" "z" {
  name          = "$1/vol"
  volsize       = 16777216
  sparse        = true
  force_destroy = true
  depends_on    = [truenas_dataset.d]
}
EOF
}
cd "$work" || exit 1
tf "$base"
terraform apply -auto-approve -no-color >apply1.log 2>&1 || { echo "APPLY FAILED"; tail -20 apply1.log; exit 1; }
echo "created $base (+/vol)"
ssh_ "echo precious > /mnt/$base/keep.txt && printf 'TFRENPROBE-SIGNATURE' | dd of=/dev/zvol/$base/vol bs=4k seek=3 conv=notrunc status=none && zfs snapshot $base@keep && zfs snapshot $base/vol@keep" || { echo "fixture FAILED"; exit 1; }
g_ds=$(guid "$base"); g_zv=$(guid "$base/vol")
echo "signature written; guid ds=$g_ds zvol=$g_zv"

tf "$base-renamed"
terraform plan -no-color -detailed-exitcode >plan.log 2>&1; code=$?
if [ $code -ne 2 ] || grep -q "must be replaced" plan.log; then echo "PLAN not an in-place update (exit $code):"; grep -E "^\s*[~+-/]|replaced|Plan:" plan.log | head; fi
terraform apply -auto-approve -no-color >apply2.log 2>&1 || { echo "RENAME APPLY FAILED"; grep -A5 Error apply2.log | head -20; }
fail=0
chk() { if [ "$2" = "$3" ]; then echo "ok   $1: $2"; else echo "FAIL $1: got '$2' want '$3'"; fail=1; fi; }
chk "file moved"      "$(ssh_ "cat /mnt/$base-renamed/keep.txt" 2>/dev/null)" "precious"
chk "zvol signature"  "$(ssh_ "dd if=/dev/zvol/$base-renamed/vol bs=4k skip=3 count=1 status=none | tr -d '\0' | head -c 20" 2>/dev/null)" "TFRENPROBE-SIGNATURE"
chk "dataset guid"    "$(guid "$base-renamed")" "$g_ds"
chk "zvol guid"       "$(guid "$base-renamed/vol")" "$g_zv"
chk "snapshots moved" "$(ssh_ "zfs list -H -t snapshot -o name -r $base-renamed | sort | tr '\n' ' '")" "$base-renamed/vol@keep $base-renamed@keep "
chk "old name gone"   "$(ssh_ "zfs list -H -o name $base 2>&1 | grep -c 'does not exist'")" "1"
terraform plan -no-color -detailed-exitcode >plan2.log 2>&1; chk "empty plan after rename" "$?" "0"
terraform destroy -auto-approve -no-color >destroy.log 2>&1 || { echo "DESTROY FAILED: leftovers $base-renamed"; fail=1; }
[ $fail -eq 0 ] && echo "RENAME PROBE PASSED on $ip" || echo "RENAME PROBE FAILED on $ip (logs in $work)"
exit $fail

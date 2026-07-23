#!/bin/bash
# Run the hello-lxc example against the 26.0 box using a locally built
# provider (dev override — no registry install needed).
# Usage:
#   TRUENAS_ENDPOINT='wss://<box>/api/current' TRUENAS_API_KEY='<key>' bash run.sh [destroy]
set -euo pipefail

if [ -z "${TRUENAS_API_KEY:-}" ] || [ -z "${TRUENAS_ENDPOINT:-}" ]; then
  echo "Set both: TRUENAS_ENDPOINT='wss://<box>/api/current' TRUENAS_API_KEY='...' bash run.sh" >&2
  exit 1
fi

repo="$(cd "$(dirname "$0")/../.." && pwd)"
bindir="$repo/.dev-override"
mkdir -p "$bindir"

echo "Building provider..."
go -C "$repo" build -o "$bindir/terraform-provider-truenas" .

cat > "$bindir/dev.tfrc" <<EOF
provider_installation {
  dev_overrides {
    "truenas/truenas" = "$bindir"
  }
  direct {}
}
EOF

cd "$repo/examples/hello-lxc"
export TF_CLI_CONFIG_FILE="$bindir/dev.tfrc"
export TF_VAR_truenas_api_key="$TRUENAS_API_KEY"
export TF_VAR_truenas_endpoint="$TRUENAS_ENDPOINT"

if [ "${1:-}" = "destroy" ]; then
  terraform destroy
else
  terraform apply
fi

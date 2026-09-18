#!/usr/bin/env bash
# Generate SBOMs for terraform-provider-truenas in BOTH standard formats:
#   sbom/terraform-provider-truenas.spdx.json   (SPDX 2.3, JSON)
#   sbom/terraform-provider-truenas.cdx.json     (CycloneDX, JSON)
#
# The SBOM is built from the COMPILED provider binary, not a source-tree scan,
# so its component set is exactly what ships to users (the runtime module
# closure + Go stdlib) — a go.sum/source scan would over-report every module
# in the graph, including test- and docs-only tooling.
#
# Release artifacts receive equivalent per-binary SBOMs automatically via
# GoReleaser (see the `sboms:` block in .goreleaser.yml). This script is the
# on-demand local/CI equivalent. Regenerate with `make sbom`.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

if ! command -v syft >/dev/null 2>&1; then
  cat >&2 <<'EOF'
ERROR: syft not found on PATH. Install one of:
  go install github.com/anchore/syft/cmd/syft@latest
  curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh \
    | sh -s -- -b "$(go env GOPATH)/bin"
EOF
  exit 1
fi

VERSION="${VERSION:-dev}"
NAME="terraform-provider-truenas"
OUTDIR="sbom"
TMP="$(mktemp -d)"
BIN="$TMP/$NAME"
trap 'rm -rf "$TMP"' EXIT

echo "Building $NAME ($VERSION) for SBOM analysis..."
CGO_ENABLED=0 GOTOOLCHAIN=auto go build \
  -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$BIN" .

mkdir -p "$OUTDIR"
SPDX="$OUTDIR/$NAME.spdx.json"
CDX="$OUTDIR/$NAME.cdx.json"

echo "Scanning binary with syft (SPDX + CycloneDX)..."
syft "$BIN" -q -o "spdx-json=$SPDX" -o "cyclonedx-json=$CDX"

echo "Wrote:"
echo "  $SPDX"
echo "  $CDX"
python3 - "$CDX" <<'PY'
import json, sys
d = json.load(open(sys.argv[1]))
print(f"  CycloneDX components: {len(d.get('components', []))}")
PY

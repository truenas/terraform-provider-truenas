#!/usr/bin/env bash
# Build the browsable docs preview from the current docs/ and serve it.
#
# Usage:  scripts/serve-docs-preview.sh [port]
# Then open http://localhost:<port> (default 8000).
#
# The preview is generated into docs-preview/ (gitignored). It reflects the
# committed docs/ as-is; run `make generate` first if you want it rebuilt
# from the current schemas. Requires the `markdown` Python package.
set -euo pipefail

port="${1:-8000}"
repo="$(cd "$(dirname "$0")/.." && pwd)"

echo "Building docs preview..."
python3 "$repo/scripts/build-docs-preview.py"

echo "Serving http://localhost:$port  (Ctrl-C to stop)"
exec python3 -m http.server "$port" --directory "$repo/docs-preview"

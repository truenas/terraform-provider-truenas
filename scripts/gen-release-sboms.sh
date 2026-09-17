#!/usr/bin/env bash
# Generate per-binary SBOMs (SPDX + CycloneDX) from the GoReleaser dist and
# upload them to the GitHub release as extra assets.
#
# Why this is a separate step and not GoReleaser's `sboms:` pipe: that pipe
# adds every SBOM to SHA256SUMS, and the Terraform Registry rejects a
# checksums file that lists SBOMs ("missing files in request body"). Uploading
# them here keeps the SBOMs on the release but out of SHA256SUMS.
#
# Runs in CI after `goreleaser release --clean`. syft, jq and gh must be on
# PATH, and GITHUB_TOKEN must be set (GitHub-hosted runners provide jq + gh).
#
#   scripts/gen-release-sboms.sh v1.0.0
set -euo pipefail

TAG="${1:?usage: gen-release-sboms.sh <tag, e.g. v1.0.0>}"
VERSION="${TAG#v}"
DIST="${DIST:-dist}"
REPO="${GITHUB_REPOSITORY:-truenas/terraform-provider-truenas}"

for tool in syft jq gh; do
  command -v "$tool" >/dev/null || { echo "$tool not on PATH" >&2; exit 1; }
done
[ -f "$DIST/artifacts.json" ] || { echo "no $DIST/artifacts.json — run goreleaser first" >&2; exit 1; }

OUT="$(mktemp -d)"
trap 'rm -rf "$OUT"' EXIT

# Every built provider binary, with its target OS/arch, straight from the
# GoReleaser manifest (robust against dist/ directory-layout changes).
mapfile -t rows < <(jq -r '.[] | select(.type=="Binary") | [.path,.goos,.goarch] | @tsv' "$DIST/artifacts.json")
[ "${#rows[@]}" -gt 0 ] || { echo "no Binary artifacts in $DIST/artifacts.json" >&2; exit 1; }

files=()
for row in "${rows[@]}"; do
  IFS=$'\t' read -r path goos goarch <<<"$row"
  base="terraform-provider-truenas_v${VERSION}_${goos}_${goarch}"
  syft scan "$path" -q \
    -o "spdx-json=$OUT/$base.spdx.json" \
    -o "cyclonedx-json=$OUT/$base.cdx.json"
  files+=("$OUT/$base.spdx.json" "$OUT/$base.cdx.json")
done

echo "Generated ${#files[@]} SBOM files from ${#rows[@]} binaries; uploading to $REPO $TAG"
gh release upload "$TAG" "${files[@]}" --repo "$REPO" --clobber
echo "SBOMs attached to release $TAG (not present in SHA256SUMS)."

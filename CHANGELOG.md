# Changelog

All notable changes to this provider are documented in this file. The format
is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Attribute-coverage fill from a live-API field audit:
  - `truenas_smb_share`: nested `audit` block (`enable`, `watch_list`,
    `ignore_list`) for per-share audit logging.
  - `truenas_nfs_share`: `security` (SYS/KRB5/KRB5I/KRB5P) and
    `expose_snapshots`.
  - `truenas_iscsi_extent`: `filesize` for FILE-type extents.
  - `truenas_iscsi_target`: nested `iscsi_parameters` block (`queued_commands`).
- `truenas_replication_task`: SSH+NETCAT transport. `transport` now accepts
  `"SSH+NETCAT"` (unencrypted data channel over a netcat connection, authenticated
  over SSH) alongside the new `netcat_active_side`,
  `netcat_active_side_listen_address`, `netcat_active_side_port_min`,
  `netcat_active_side_port_max`, and `netcat_passive_side_connect_address`
  attributes.
- `truenas_nfs_share`: `mapall_user` and `mapall_group` attributes, mapping all
  NFS clients to a given user/group (mutually exclusive with `maproot_*`).
- Release, CI, and governance scaffolding: MPL-2.0 `LICENSE`, GoReleaser
  release pipeline and Terraform Registry manifest, GitHub Actions
  (build/vet/gofmt/lint/unit-test/docs-check/license-header-check on PRs;
  signed release on tags; manual self-hosted acceptance run), `SECURITY.md`,
  `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, and this changelog.
- SPDX license headers (`MPL-2.0`, TrueNAS) on all Go source files,
  with a `.copywrite.hcl` config and a CI check enforcing them.
- Supply-chain and governance files: `.github/CODEOWNERS`,
  `.github/dependabot.yml` (Go modules and Actions), issue templates, and a
  pull-request template.
- `VERSIONING.md`: the semantic-versioning, TrueNAS-release-compatibility,
  state-migration, and deprecation policy.

### Security
- Marked the SNMP `community` string and the VM device `attributes` blob
  (which can carry a DISPLAY console password) `Sensitive`, so neither
  appears in plan output or unmasked state.
- Documented that `insecure = true` also weakens authentication (an
  intercepting peer can force the plaintext-key fallback that SCRAM
  otherwise prevents).

<!--
Release process:
1. Move the Unreleased entries under a new "## [X.Y.Z] - YYYY-MM-DD" heading.
2. Commit, then tag: git tag vX.Y.Z && git push origin vX.Y.Z
3. The release workflow builds, signs, and publishes the archives the
   Terraform Registry ingests.
-->

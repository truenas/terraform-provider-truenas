# Changelog

All notable changes to this provider are documented in this file. The format
is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.8] - 2026-09-19

### Documentation
- Swept every resource example that consumes a dataset to reference the dataset
  resource rather than hardcoding its path or name, so a single `terraform
  apply` that manages the dataset and its consumers orders them correctly
  instead of racing (a hardcoded value gives Terraform no dependency edge and
  fails with a "path/parent not found" error on the first apply, succeeding
  only on the second). Path consumers (`truenas_filesystem_permissions`,
  `truenas_filesystem_acl`, `truenas_webshare`, `truenas_rsync_task`,
  `truenas_cloudsync_task`, `truenas_cloud_backup`) now use
  `truenas_dataset.<name>.mountpoint`; dataset-name consumers
  (`truenas_periodic_snapshot_task`, `truenas_snapshot`,
  `truenas_replication_task`, `truenas_vmware`) use `truenas_dataset.<name>.name`.
- Regenerated the `truenas_dataset` and `truenas_pool` reference docs, which
  v1.0.7 shipped without regenerating after their examples changed.

## [1.0.7] - 2026-09-19

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

### Documentation
- `truenas_pool` / `truenas_dataset` examples: when a configuration manages a
  pool and datasets together, the dataset name now references the pool
  (`name = "${truenas_pool.tank.name}/media"`) so Terraform orders the pool
  before its datasets instead of racing them (a hardcoded name gives no
  dependency edge and fails with a parent-not-found error on the first apply,
  succeeding only on the second). Also corrects the `truenas_pool` example's
  `topology` to the nested-attribute assignment form (`topology = { ... }`),
  which the block form (`topology { ... }`) is not valid for.

### Fixed
- `truenas_ipmi_lan`: reading a statically-addressed BMC LAN channel back right
  after a change no longer produces a spurious perpetual diff. The BMC LAN
  controller flaps `ip_address`/`subnet_mask` through `0.0.0.0` (under both a
  `static` and an `unspecified` source) for 10–15s while it settles after any
  `ipmi.lan.update`; the resource's Read and Import now wait out that transient
  when the channel is a configured static address, converging on any settled
  non-zero read (so a genuine out-of-band change is still detected as drift).
  Verified live end-to-end (apply + refresh + import) on a physical BMC.
- `truenas_network_config`: create/update no longer fails on a box that leaves
  `ipv6gateway` or a `nameserver2`/`nameserver3` slot empty (the common case).
  Empty gateway/nameserver fields were sent as JSON `null`, which TrueNAS
  rejects (`[EINVAL] ... Input should be ''`); they are now sent as `""`.
  Verified live (hostname set/restore) on a disposable box.
- `truenas_cloudsync_credentials`: a custom S3 `endpoint` (MinIO, SeaweedFS,
  Wasabi, Backblaze, and any other S3-compatible provider) no longer causes a
  perpetual diff. TrueNAS appends a trailing slash to the endpoint on
  read-back; the drift check now treats a trailing-slash-only difference as
  server normalization rather than a change. Verified live against a local
  S3-compatible endpoint.
- `truenas_pool`: pool creation and lifecycle now work end-to-end (verified on
  live drives). Fixes a cascade of bugs, none previously covered by a live
  create test:
  - a "Value Conversion Error" crash when a config omitted the `log` vdev
    (topology vdev lists now hold null/unknown).
  - `pool.create` payload mismatches: `autotrim` isn't a create field (now
    applied via a follow-up update), the topology spares key is `spares` (not
    `spare`), and cache vdevs use `type = "STRIPE"`.
  - deletion used a nonexistent `pool.delete` (now `pool.export` with
    `destroy`), and import failed for the numeric id (now imports by id or
    name).
  - reading a single-disk cache/log/spare vdev back (device is reported at the
    vdev top level with empty children; single-disk log normalizes
    `DISK`→`STRIPE`).

<!--
Release process:
1. Move the Unreleased entries under a new "## [X.Y.Z] - YYYY-MM-DD" heading.
2. Commit, then tag: git tag vX.Y.Z && git push origin vX.Y.Z
3. The release workflow builds, signs, and publishes the archives the
   Terraform Registry ingests.
-->

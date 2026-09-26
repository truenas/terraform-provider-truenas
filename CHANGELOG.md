# Changelog

All notable changes to this provider are documented in this file. The format
is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `truenas_dataset`: new optional `special_small_block_size` attribute, wrapping
  the ZFS `special_small_blocks` property on `pool.dataset.create`/`update`. It
  sets the threshold in bytes below which blocks are written to a pool's special
  allocation class vdev; `0` disables the behaviour. The attribute is a string
  holding a decimal integer or the literal `INHERIT` (case-insensitive), which
  `pool.dataset.create`/`update` take to mean "inherit from the parent"; a
  number is sent to the API as an integer, `0` included, and an HCL number such
  as `special_small_block_size = 16384` converts automatically (state holds
  `"16384"`). Omitting the attribute leaves the property inherited from the
  parent dataset, and `INHERIT` says so explicitly: it is for creating a dataset
  that inherits, or declaring one that already does. A size set on the dataset
  cannot be changed to `INHERIT`: the plan fails, since that one-word edit would
  silently move where the dataset's future small blocks are written. Removing
  the attribute keeps the last applied value. The read path keys off the
  property's `source` from `pool.dataset.get_instance` and records the value in
  state only when that source is `LOCAL`; anything else (inherited, default or
  received) is recorded as `INHERIT`, so an inherited or default value is never
  written back and an apply cannot silently convert an inherited property into a
  local one. It is null only when `get_instance` does not report the property at
  all, for a dataset type that does not carry it. An update sends `INHERIT` only
  when the property is not already `INHERIT` in state. Also exposed as a
  computed attribute on the `truenas_dataset` data source, with the same values.

## [1.1.0] - 2026-09-23

### Added
- **Terraform Actions** (Terraform 1.14+): trigger operational TrueNAS jobs from
  Terraform. Nine actions — `truenas_scrub_run`, `truenas_replication_run`,
  `truenas_cloudsync_run`, `truenas_snapshot_task_run`, `truenas_service_control`
  (verb START/STOP/RESTART/RELOAD), `truenas_app_start`, `truenas_app_stop`,
  `truenas_app_redeploy`, and `truenas_ui_restart`. Job-backed actions take an
  optional `wait` (default `true`); set `wait = false` to start the job and
  return immediately instead of blocking until it completes.
  `truenas_service_control` requires TrueNAS 26.0+.
- **List resources / `terraform query`** (Terraform 1.14+): every resource can
  be enumerated to discover objects that already exist on a TrueNAS system,
  independent of Terraform state — the discovery half of the import story. See
  `examples/list/` (including `discover-all.tfquery.hcl`, a full-system
  inventory).
- **Resource Identity** (Terraform 1.12+): every resource now has an identity
  schema and supports identity-based import, e.g.
  `import { to = truenas_pool.tank, identity = { id = 1 } }`. String-id import
  (`terraform import`) continues to work unchanged.

## [1.0.11] - 2026-09-21

### Changed
- `truenas_pool`: the provider now **refuses to ever plan a pool
  destroy/recreate** from a configuration change. `name` and `topology` are no
  longer `RequiresReplace`; instead a post-create change to either is rejected
  at plan time with an actionable error, because replacing a ZFS pool destroys
  all of its data and is essentially never a valid automatic outcome (the same
  stance as AWS `deletion_protection`/`force_destroy` and Terraform's
  `prevent_destroy`). A genuine topology change (grow, disk replace,
  add/remove cache/log/spare) is made in TrueNAS/`zpool` and reconciled with
  `terraform apply -refresh-only`; a deliberate teardown is still `terraform
  destroy`. Adding `lifecycle { prevent_destroy = true }` to pool resources is
  recommended as defense-in-depth (now shown in the example).

### Fixed
- `truenas_pool`: a pool with a **physically removed disk** — a pulled or failed
  data member, or a removed cache/log device — no longer plans a
  destroy/recreate. `pool.query` reports a `REMOVED`/`UNAVAIL` device with
  `disk`/`device` null (and the disk drops out of `disk.query`), but still
  carries an `unavail_disk` record with the disk's stable serial. The provider
  now recovers the member identity from that serial, so a serial-pinned config
  matches the removed member and the degraded pool plans no change. Verified
  live (member pulled from a mirror, pool imported, plans "No changes").
  Complements the hot-spare-activation fix in v1.0.10.

## [1.0.10] - 2026-09-21

### Fixed
- `truenas_pool`: a pool that has gone **degraded with a hot spare active** no
  longer plans a destroy/recreate. When a spare steps in for a faulted member,
  `pool.query` nests a `SPARE` vdev (original faulted disk + spare) inside the
  data vdev; the provider read the mirror's members as `[disk, ""]`, which
  differed from the configured membership and — because `topology` is
  `RequiresReplace` — planned a destroy/recreate of a degraded pool (a
  data-loss hazard). A nested `SPARE`/`REPLACING` child is now represented by
  its original member, so a degraded, spare-covered pool reads back with its
  configured membership and plans no change. Verified live: a real hot-spare
  activation on physical disks, imported with a serial-pinned config, plans
  "No changes."

## [1.0.9] - 2026-09-21

### Fixed
- `truenas_pool`: a pool no longer plans a destroy/recreate when a disk's
  kernel device name (`sdX`) changes across a reboot, and no longer perpetually
  diffs on the vdev `type` spelling (issue #9). Two root causes:
  - Topology disks could only be named by the volatile `sdX` device name, so a
    reboot renumber made `RequiresReplace` fire. `disks` now accepts a disk
    named by its stable **serial** (recommended), its TrueNAS identifier, a
    `/dev/disk/by-id` path, or an `sdX` name; at plan time the provider resolves
    the configured name and the one in state (via `disk.query`) to the same
    physical disk and suppresses the diff, so a config that pins disks by serial
    is renumber-proof. State reflects the live device name; the stability is in
    the plan, not a rewritten state. Existing `sdX` configs keep working.
  - A single-disk vdev read back as `type = "DISK"` but written in config as
    `"STRIPE"` (or vice-versa) perpetually diffed; `"DISK"` is now accepted as
    an alias for `"STRIPE"`. Computed pool attributes also carry
    `UseStateForUnknown` so a no-op plan no longer shows a spurious in-place
    update. Verified live end-to-end (create by serial, re-plan by sdX +
    `DISK` as a no-op, import) on real disks.

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

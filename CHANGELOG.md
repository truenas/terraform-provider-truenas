# Changelog

All notable changes to this provider are documented in this file. The format
is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.5.11] - 2026-10-06

### Fixed
- `truenas_dataset` / `truenas_zvol`: `compression = "ZSTD-FAST-1"` no longer
  fails with an inconsistent-result error after apply. ZFS stores the level-1
  fast tier as plain `zstd-fast`, so the value is now folded to that canonical
  form and reads back cleanly; every other level (`zstd-fast-10`, `zstd-5`,
  `gzip-9`, …) is unchanged.
- `truenas_dataset`: `acltype = "inherit"` now round-trips instead of failing
  with an inconsistent result. An inherited or default `acltype` reads back as
  `INHERIT` rather than resolving to the pool default (e.g. `posix`), matching
  the source-aware handling already used for `compression`.

### Testing
- Added an exhaustive value round-trip acceptance test
  (`internal/acctest`): for every value the TrueNAS API advertises as settable
  on an enum field, it applies a resource with that value and asserts the next
  plan is empty. This is the set-vs-read-back coverage behind the two fixes
  above; it runs across every creatable enum-bearing resource.

## [1.5.10] - 2026-10-06

### Documentation
- Regenerated the provider documentation so the Registry reflects the
  attributes added in 1.5.6–1.5.9 — the write-only secret attributes
  (`*_wo` / `*_wo_version`) and `truenas_zvol` encryption. No code changes;
  this release exists so the Registry's current docs match the shipped schema.

## [1.5.9] - 2026-10-06

### Added
- `truenas_zvol`: ZFS encryption support, matching `truenas_dataset` —
  `encryption`, `inherit_encryption`, `encryption_algorithm`,
  `encryption_generate_key`, write-only `encryption_passphrase` /
  `encryption_key`, and computed `key_format` / `locked`. Inherited encryption
  is reconciled on read, setting the encryption inputs on an imported zvol does
  not force replacement, `encryption` combined with `inherit_encryption = true`
  is rejected at plan time, and `encryption_algorithm` is not sent on TrueNAS
  27.0+ (removed there). Verified on 25.10, 26.0, and 28.0.

### Fixed
- `truenas_zvol`: `pool` no longer plans as "known after apply" on an in-place
  update (it derives from the RequiresReplace `name` and never changes) — parity
  with `truenas_dataset`.

## [1.5.8] - 2026-10-06

### Fixed
- `truenas_zvol` and `truenas_dataset`: setting `compression = "inherit"` failed
  with "Provider produced inconsistent result after apply" (the inherited value
  resolved to the parent's algorithm, e.g. `lz4`, instead of round-tripping as
  `inherit`). `compression` is now source-aware: it reports `inherit` when not
  set locally on the dataset/zvol, and the configured value round-trips. (#38)

## [1.5.7] - 2026-10-05

### Added
- `truenas_app`: `custom_compose_config_string_wo` (with
  `custom_compose_config_string_wo_version`) — a write-only overlay for the
  secret parts of a custom app's Compose. It is deep-merged into
  `custom_compose_config_string` when sending (including nested paths such as a
  service's `environment`) and never stored in state; on refresh the live
  Compose is projected onto only the keys in the base string, so the overlay's
  secret keys are not read back. Drift is still detected on the non-secret base.
  The plaintext `custom_compose_config_string` is unchanged. (#34)

## [1.5.6] - 2026-10-05

### Added
- Write-only alternatives that keep secrets out of state and saved plan files.
  Sensitive attributes are hidden from plan output but are still written to
  state; these new `*_wo` attributes (with a `*_wo_version` trigger) are read
  from configuration and never stored, and are not read back on refresh:
  `truenas_cloud_backup.password_wo` (#36), `truenas_cloudsync_credentials`
  `provider_secrets_wo` (merged over `provider_config`, #36),
  `truenas_acme_dns_authenticator.attributes_secrets_wo` (#37),
  `truenas_alert_service.attributes_secrets_wo`,
  `truenas_vm_device.attributes_secrets_wo`, `truenas_snmp_config.community_wo`,
  `truenas_truecommand_config.api_key_wo`, and `truenas_kerberos_keytab.file_wo`.
  The existing plaintext attributes keep working; set exactly one.

### Changed
- `truenas_certificate`: the private key for an ACME certificate is no longer
  stored in state (TrueNAS manages it; the certificate is referenced by id).
  CSR and imported certificates keep the private key, which the user needs. (#37)
- `truenas_system_advanced`: `anonstats_token` is now marked sensitive.

## [1.5.5] - 2026-10-05

### Changed
- `truenas_app`: `custom_compose_config_string` now detects drift for custom
  apps. It was write-only, so Compose edits made in the UI or via the API were
  invisible. On read it is reconciled from the live app configuration and
  compared semantically — formatting, comments, key order, and YAML-vs-JSON
  number spelling are not reported as drift, and the document round-trips
  (including on import). Large integers keep exact precision. The attribute is
  now marked sensitive: a Compose document may contain secrets, so it is not
  shown in plan or state output. Read failures are surfaced rather than treated
  as verified. (#34)

## [1.5.4] - 2026-10-03

### Changed
- `truenas_app`: `values` is now reconciled from the live app configuration on
  read, so configuration drift — a change made in the UI or via the API to a
  value you manage — is detected on the next plan. It was previously write-only
  and such drift was invisible. The live config is projected onto the keys you
  set: chart defaults you did not set and server-managed `ix_*` keys are not
  reported as drift. Note: a deployed app whose config has drifted from your
  Terraform configuration will show that drift on the first plan after
  upgrading. `custom_compose_config_string` (custom apps) remains write-only. (#33)

## [1.5.3] - 2026-10-02

### Fixed
- `truenas_dataset`: an existing dataset that inherits its encryption from the
  parent now reports `inherit_encryption` correctly on read (reconciled from the
  dataset's encryption root), so it round-trips and adding `inherit_encryption`
  to configuration no longer plans a spurious in-place update. (#32)
- `truenas_dataset`: setting `encryption` together with `inherit_encryption =
  true` is now rejected at plan time with a clear message, instead of failing
  during apply with "Provider produced inconsistent result after apply" — with
  inheritance the parent determines encryption, so an explicit `encryption`
  value is ambiguous. Remove `encryption`, or set `inherit_encryption = false`
  to manage encryption on the dataset. (#31)

## [1.5.2] - 2026-10-01

### Fixed
- `truenas_dataset`: an in-place update (e.g. changing `comments`) of a dataset
  created with `share_type` failed — `pool.dataset.update` rejects the
  create-only `share_type` field. It is no longer sent on update. (#25)
- `truenas_dataset`: an in-place update of a dataset created with
  `acltype = "posix"` could fail with an inconsistent-result error — resending
  the create-only `acltype` made the server set `aclmode`/`aclinherit` to
  `DISCARD`, changing an inherited value. `acltype` is no longer sent on
  update. (#26)
- `truenas_dataset`: updating a dataset in place no longer plans a replacement
  of resources that consume its `mountpoint` (such as `truenas_filesystem_acl`)
  — `mountpoint` and `pool` now keep their known values on update instead of
  becoming "known after apply". (#27)
- `truenas_app`: creating or updating an app no longer fails with "Provider
  produced inconsistent result after apply" when the app is still deploying —
  the provider now waits for the app to leave the transient `DEPLOYING` state
  before reading it back. (#28)
- `truenas_dataset`: importing an encrypted dataset and then setting
  `inherit_encryption` / `encryption_generate_key` in configuration no longer
  forces the dataset to be destroyed and recreated. (#29)
- `truenas_vm_device`: after importing a device, a plan no longer shows a
  spurious diff on `attributes` when the configuration already matches the
  device's live attributes. (#30)
- `truenas_snmp_config`: setting `loglevel` failed on TrueNAS 27.0 with
  `[EINVAL] snmp_update.loglevel: Extra inputs are not permitted` — 27.0 removed
  the field from `snmp.update`. It is no longer sent on 27.0+ (with a clear
  error if it is configured there) and is read back as null when absent; it
  continues to work on 25.10/26.0. Found by the full cross-version acceptance
  run.

### Added
- Test coverage auditors guarding the update/import lifecycle: a lifecycle-phase
  audit (every resource must exercise update and import), a create-vs-update
  `accepts` audit (a create-only API field mapping to a writable attribute must
  be `RequiresReplace` or stripped from the update payload), and an
  import-then-reapply acceptance helper that asserts an imported resource plans
  no changes. All six fixes above are covered by regression tests verified on
  TrueNAS 25.10, 26.0, and 27.0.

## [1.5.1] - 2026-09-29

### Fixed
- Data source reads for `truenas_user`, `truenas_cloudsync_task`, and
  `truenas_replication_task` failed with "mismatch between struct and object:
  Struct defines fields not found in object" — their data source models carried
  write-only fields absent from the data source schema (`home_mode` for user;
  `encryption_password` / `encryption_salt` for cloudsync; `encryption_key` for
  replication). `truenas_user` drops the stray field from its data source model;
  cloudsync and replication declare the fields as computed so the shared model
  matches. Adds a `truenas_user` data source acceptance test (verified on
  25.10, 26.0, 27.0) and a source guard (`TestDataSourceModelMatchesSchema`)
  that fails the build if any data source model has a field its schema omits.
  (#23)

## [1.5.0] - 2026-09-29

### Added
- `truenas_dataset`: encryption support at creation (#18) — `encryption`,
  `inherit_encryption`, `encryption_algorithm`, `encryption_generate_key`, and
  the write-only `encryption_passphrase` / `encryption_key`. The encryption
  state is read back through new computed `key_format` and `locked` attributes
  (alongside the existing `encrypted`). All encryption inputs are create-only:
  changing one recreates the dataset. Combined with the existing
  `truenas_dataset_lock` / `truenas_dataset_unlock` actions, this covers
  managing a dataset's encryption settings and lock state as code.
  `encryption_algorithm` is version-gated: TrueNAS 27.0 removed it from the
  create API (the algorithm is fixed server-side), so it is not sent there and
  is read back from the computed value. Verified live on 25.10.3.1, 26.0, and
  27.0 (passphrase and generated-key datasets).

### Fixed
- **Resource identity after update**: 81 resources declared an identity schema
  and set it on create/read/import but not on update, so any in-place update on
  a Terraform version that enforces resource identity failed with "The Terraform
  Provider unexpectedly returned no resource identity data after having no
  errors in the resource update" and left the resource tainted. Every affected
  resource's `Update` now sets its identity (mirroring its `Create`). A source
  guard (`TestResourceIdentitySetInUpdate`) fails the build if a new
  identity-bearing resource omits it, and a live regression test asserts the
  identity matches state after an update. (#20)
- `truenas_nfs_share`: a `networks` entry with host bits set (e.g.
  `192.168.100.10/24`) failed with "Provider produced inconsistent result after
  apply" because TrueNAS stores the network address (`192.168.100.0/24`). The
  `networks` elements now compare by network address, so the configured form is
  kept and the apply is consistent. (#13)
- `truenas_user`: creating a user with `smb = true` failed with "Provider
  produced inconsistent result after apply — .groups: new element … has
  appeared" because TrueNAS auto-adds the `builtin_users` group. Server-managed
  auxiliary groups the configuration did not request are now reconciled out of
  `groups`, so the apply is consistent; a group the configuration lists
  explicitly is still kept. (#14)

## [1.4.3] - 2026-09-29

### Fixed
- `truenas_certificate`: setting `digest_algorithm` on a server-generated
  certificate or CSR (`create_type = CERTIFICATE_CREATE_CSR`) failed the apply
  with "Provider produced inconsistent result after apply" — the API accepts
  `digest_algorithm` as a generation input but returns null for a CSR, and the
  read overwrote the configured value with that null. The read now keeps the
  configured value when the API omits it (and still reflects the API's value for
  signed certificates). Found by full-surface acceptance testing.
- `truenas_zvol`: setting `volblocksize` always failed create — the value was
  sent as an integer byte count, but `pool.dataset.create` requires a string
  enum (`"512"`, `"1K"` … `"128K"`). It is now converted, so `volblocksize`
  works.
- `truenas_zvol`: `sync` and `dedup` came back lower-cased on import (`ON` →
  `on`), causing an ImportStateVerify / plan mismatch against an upper-case
  config. They are now read from the API's source-aware `value` field (the
  canonical upper-case form), matching how `checksum` already worked.

### Removed
- `truenas_zvol`: the `special_small_block_size` attribute. `special_small_blocks`
  is a filesystem-only ZFS property; setting it on a volume always failed with
  "does not apply to datasets of this type", so the attribute was non-functional.
  (It remains on `truenas_dataset`, where it applies.)

## [1.4.2] - 2026-09-29

### Fixed
- `truenas_vm`: every in-place update failed with
  `[EINVAL] vm_update.bootloader_ovmf: Extra inputs are not permitted` /
  `enable_secure_boot: Extra inputs are not permitted`. These two attributes are
  accepted by `vm.create` but rejected by `vm.update`; the resource sent the
  same payload for both, so any change to a VM (memory, cores, autostart, …)
  was rejected and could not be applied. They are now dropped from the update
  payload and marked create-only (changing either recreates the VM). Verified
  end-to-end on 27.0: create with the full attribute set, update a subset, and
  attach disk/NIC/display devices, all with no plan drift.

## [1.4.1] - 2026-09-29

### Added
- Eight more **Terraform Actions** for dataset and VM operations that had no
  provider surface: `truenas_dataset_lock` / `truenas_dataset_unlock`
  (encrypted-dataset lock/unlock; unlock takes a passphrase or hex key),
  `truenas_dataset_promote` (promote a clone), `truenas_dataset_rename`,
  `truenas_dataset_set_quota` (user/group/dataset quotas), `truenas_vm_clone`,
  `truenas_vm_restart`, and `truenas_vm_reset` (`vm_reset` requires TrueNAS
  27.0+ and is version-gated). Verified end-to-end through a Terraform
  `action_trigger` on 25.10.3.1 and 27.0. `truenas_dataset_rename` needs
  `force = true` (TrueNAS performs no safety checks on a rename and refuses
  without it); its documentation says so.

## [1.4.0] - 2026-09-28

### Added
- Five snapshot **Terraform Actions** (Terraform 1.14+) covering the
  `pool.snapshot.*` operations the provider didn't expose:
  `truenas_snapshot_rollback` (revert a dataset to a snapshot),
  `truenas_snapshot_clone` (clone a snapshot into a new dataset),
  `truenas_snapshot_hold` / `truenas_snapshot_release` (deletion holds), and
  `truenas_snapshot_rename`. Live-verified against the API on TrueNAS 25.10.3.1.
- `truenas_pool`: `deduplication` and `checksum` — set the pool's root-dataset
  ZFS properties at creation (also on the data source). `pool.query` returns
  null for them, so they are read back source-aware from the pool's root dataset
  (`pool.dataset.get_instance`), and changes are applied to the root dataset via
  `pool.dataset.update`. (Not redundant with `truenas_dataset` — the root
  dataset is created by the pool and isn't independently manageable without
  import.) (coverage audit)

## [1.3.1] - 2026-09-28

### Fixed
- `truenas_user`: the `webshare` attribute added in v1.3.0 was sent
  unconditionally, but the field only exists on TrueNAS 26.0+. On 25.10 this
  made every `user.create`/`user.update` fail with "Extra inputs are not
  permitted", breaking the resource entirely. `webshare` is now version-gated:
  it is dropped from the payload below TrueNAS 26.0, so `truenas_user` works on
  25.10 again (setting `webshare` there has no effect; the attribute
  documents the 26.0 requirement). Live-verified on 25.10.3.1, 26.0, and 27.0.

## [1.3.0] - 2026-09-28

### Added
- `truenas_cloudsync_task`: `transfers` (parallel file transfers), `follow_symlinks`,
  and `create_empty_src_dirs` (also on the data source). The crypt group
  (`encryption`/`filename_encryption` with write-only `encryption_password`/`salt`)
  and `bwlimit` (nested) remain to be modeled. (GH coverage audit)
- `truenas_replication_task`: eight send-stream and behaviour options —
  `compressed`, `embed`, `large_block` (ZFS send `-c`/`-e`/`-L`),
  `allow_from_scratch`, `hold_pending_snapshots`, `only_matching_schedule`,
  `logging_level`, and `properties_exclude` (also on the data source). The
  encryption group (`encryption`/`encryption_key`/…) and the nested
  `restrict_schedule`/`lifetimes`/`properties_override` remain to be modeled.
  (GH coverage audit)
- `truenas_group`: `users` — the list of user IDs (`truenas_user.id`) that are
  members of the group (also on the data source). Omitting it leaves existing
  membership unchanged; it is guarded so an unset value never wipes members.
  (GH coverage audit)
- `truenas_user`: `webshare` (grant web-file-share access; read back and
  drift-detected) and `home_mode` (octal home-directory permission mode). The
  API accepts `home_mode` but never returns it, so it is modeled as a
  write-only attribute — it is applied on create/update but not read back or
  drift-detected. (GH coverage audit)
- `truenas_vm`: thirteen hardware/boot/CPU options — `machine_type`,
  `arch_type`, `bootloader_ovmf`, `command_line_args`, `cpuset`, `nodeset`,
  `enable_secure_boot`, `trusted_platform_module`, `pin_vcpus`, `hide_from_msr`,
  `hyperv_enlightenments`, `enable_cpu_topology_extension`, and
  `suspend_on_snapshot` (also on the data source). These are plain
  Optional+Computed settings. TrueNAS enforces cross-field rules server-side
  (e.g. `arch_type` is required with `machine_type`; `enable_secure_boot` needs
  a compatible `machine_type`; `cpuset` must cover the vCPU count for
  `pin_vcpus`), surfaced as apply-time errors. (GH-16)
- `truenas_zvol`: seven ZFS tuning properties applicable to volumes —
  `checksum`, `readonly`, `snapdev`, `copies`, `special_small_block_size`,
  `reservation`, and `refreservation` (also on the data source), using the same
  source-aware read as `truenas_dataset` (null when inherited/default).
  Filesystem-only properties (recordsize, atime, exec, snapdir, aclmode, quota)
  are intentionally excluded — they do not apply to a block device. (GH-16)
- `truenas_dataset`: twelve ZFS tuning properties — `aclmode`, `atime`, `exec`,
  `readonly`, `sync`, `checksum`, `snapdir`, `dedup`, `recordsize`, `copies`,
  `special_small_block_size`, and `refreservation` (also exposed as computed
  attributes on the data source). Each is source-aware: it reads back null when
  the property is inherited from the parent or left at its ZFS default, so an
  inherited value is never written into state and re-sent, and an apply cannot
  silently convert an inherited property into a local one. (`sync`/`dedup` match
  the existing `truenas_zvol` attribute names.) Note: because an inherited
  property reads back as unset, reverting a locally-set value to inherited
  cannot be expressed by removing it from the configuration — change it out of
  band and refresh. (GH-16)
- `truenas_dataset`: `xattr` (extended-attribute storage mode: SA / ON / OFF),
  exposed **read-only** (also on the data source). TrueNAS returns it from
  `get_instance` but does not accept it in the writable `create`/`update` API
  (verified live on 25.10 and 27.0), so it can be read and drift-observed but
  not set. With this, the original dataset ZFS-property request is fully
  addressed — the other twelve are writable above. (GH-16)

## [1.2.1] - 2026-09-28

### Added
- `truenas_smb_share`: plan-time validation now rejects an `options` field that
  is not valid for the share's `purpose` (e.g. `recyclebin` on a
  `TIMEMACHINE_SHARE`), pointing at the offending attribute and listing the
  valid options for that purpose. Previously such a field was silently dropped.
  (GH-21 follow-up)

### Fixed
- `truenas_smb_share` **data source**: now exposes the `options` object, matching
  the resource. Previously the data source carried only the flat legacy
  attributes, so purpose-specific settings (e.g. a `TIMEMACHINE_SHARE`'s
  `auto_dataset_creation`) could not be read for a share not managed by the same
  configuration. (GH-21 follow-up)

## [1.2.0] - 2026-09-28

### Added
- `truenas_smb_share_acl`: new resource (and matching data source) managing an
  SMB share's share-level ACL via `sharing.smb.setacl` / `getacl`, keyed by
  `share_name`. Each entry sets `ae_perm` (FULL/CHANGE/READ), `ae_type`
  (ALLOWED/DENIED) and one principal selector — `ae_who_sid`, `ae_who_id`
  (`{id_type, id}`), or `ae_who_str`. Follows the same write-what-you-said
  modeling as `truenas_filesystem_acl` (the server resolves the other principal
  selectors on write, but only what you configured is kept in state, so there is
  no spurious drift). `terraform destroy` resets the share ACL to the TrueNAS
  default (`everyone@ FULL ALLOWED`) with a warning, since a share always has a
  share ACL. This closes the last SMB API coverage gap — share-level ACLs were
  previously unmanageable by the provider.
- `truenas_smb_share`: a typed `options` object exposing the full
  purpose-specific SMB settings (the discriminated `options` union on TrueNAS
  26.0+/27.0), for **every** purpose — e.g. `TIMEMACHINE_SHARE`'s
  `auto_dataset_creation` / `auto_snapshot` / `dataset_naming_schema`,
  `DEFAULT_SHARE`/`MULTIPROTOCOL_SHARE`/etc. `hostsallow` / `hostsdeny` /
  `aapl_name_mangling`, `TIME_LOCKED_SHARE`'s `grace_period`,
  `PRIVATE_DATASETS_SHARE`'s `auto_quota`, and `EXTERNAL_SHARE`'s `remote_path`.
  Only the fields valid for the chosen `purpose` are sent; the rest read back
  null. The flat legacy attributes continue to work for `LEGACY_SHARE` and are
  used as a fallback when the matching `options` field is unset. (GH-21)

### Fixed
- `truenas_smb_share`: `options` are now read back for **all** purposes, so
  drift in purpose-specific settings is detected instead of being invisible,
  and `Create`/`Update` no longer overwrite `options` with just the purpose —
  which could silently reset settings such as a Time Machine share's
  `auto_dataset_creation` on an unrelated apply. (GH-21)

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

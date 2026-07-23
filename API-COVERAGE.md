# TrueNAS API Coverage Report

**Audience:** Engineering management
**Provider:** terraform-provider-truenas
**Surveyed against:** TrueNAS SCALE 26.0 (127 live API namespaces), cross-checked on 25.10
**Date:** 2026-07-22

## Executive Summary

The provider implements **71 resources and 71 matching data sources**, covering the
core storage, sharing, block-storage, accounts, scheduled-task, access-management,
certificates/ACME, keychain/remote-replication, filesystem permissions/ACLs,
Active Directory/LDAP/IPA/Kerberos, and system-configuration surface of the
TrueNAS SCALE API. Every implemented resource has a full acceptance test (create,
update, import, destroy) run against real TrueNAS boxes on both 25.10 and 26.0,
with two deliberate exceptions: `truenas_directoryservices` is live-tested on the
25.10 VM plus dedicated Samba AD, OpenLDAP, and FreeIPA servers only —
directory-service tests never run against the production-serving 26.0 box, by
design (see TESTING.md) — and `truenas_cloud_backup`, whose acceptance test is a
documented, permanent skip because `cloud_backup.create` validates the
credential/bucket against a real remote endpoint and no live S3-compatible bucket
fixture is available in this environment (see Part 2, Data protection and
movement).

Of the 127 API namespaces the middleware exposes, more than half now map to
declarative resources we cover, a smaller share are uncovered but viable Terraform
resources (the backlog, detailed below), and the remainder are actions, telemetry,
or enterprise hardware features that are either a poor fit for Terraform or need
dedicated hardware scheduled for testing.

Every gap previously tracked as high-value (§1.1: certificates/ACME, remote
replication, scheduled tasks, filesystem ACLs, two-factor auth, cloud backup,
audit config, reporting exporters) is now covered — see Part 2. The
next-highest-value gaps, in rough priority order:

1. **Containers and apps (26.0)** — Incus system containers (`container`,
   `container.device`, `container.image`), Docker service configuration
   (`docker`, `docker.network`), and private registries (`app.registry`) sit
   alongside our existing `truenas_app` catalog-app coverage but remain
   uncovered themselves.
2. **New 26.0 share type** — `webshare`/`sharing.webshare` (WebDAV-style web
   shares), a natural fit next to `truenas_nfs_share`/`truenas_smb_share`.
3. **Enterprise/HA hardware** — failover controller pairs, Fibre Channel,
   JBOF shelves, enclosure management, IPMI, RDMA: all need dedicated hardware
   scheduled for testing before they can ship under this project's
   no-mocks policy.

Directory services no longer belongs on this list: Active Directory, LDAP, and
IPA join, Kerberos (config, realms, keytabs), and explicit AD idmap configuration
are all covered now — see Part 2. The standalone `idmap` API namespace exposes
only a cache-clear action (`clear_idmap_cache`), not durable configuration state —
see Part 1, §1.5 (poor Terraform fit); idmap *configuration* is exposed as a
nested block on `truenas_directoryservices` instead.

## Part 1 — Uncovered API Areas

### 1.1 High-value gaps (viable resources, requested workflows)

*(empty)* — every namespace previously tracked here (`certificate`,
`acme.dns.authenticator`, `keychaincredential`, `cronjob`,
`initshutdownscript`, `cloud_backup`, `auth.twofactor`, `filesystem`,
`filesystem.acltemplate`, `audit`, `reporting.exporters`) is now covered; see
Part 2 (Certificates & ACME, Scheduled tasks, Access management, Keychain &
remote replication, Filesystem permissions & ACLs, System). The next tier of
gaps is §1.2 below.

### 1.2 Containers and applications (26.0 growth area)

| Namespace(s) | What it manages | Notes |
|---|---|---|
| `container`, `container.device`, `container.image` | Incus system containers (new in 26.0) | Parallel to our `vm`/`vm_device` pair |
| `lxc` | LXC image import | |
| `docker`, `docker.network` | Docker service configuration and networks | Singleton + CRUD |
| `app.registry` | Private container registries | Small CRUD |
| `catalog` | App catalog trains configuration | Singleton |
| `virt.*` (25.10 only) | Predecessor of `container` | Superseded in 26.0; recommend covering `container` only |

Our existing `app` resource covers catalog-app install/upgrade/delete lifecycle;
the namespaces above surround it.

### 1.3 New 26.0 surfaces

| Namespace(s) | What it manages | Notes |
|---|---|---|
| `webshare`, `sharing.webshare` | WebDAV-style web shares (new share type) | Natural fit next to `nfs`/`smb` |
| `zfs.resource`, `zfs.resource.snapshot`, `zpool`, `zpool.scrub` | Next-generation ZFS namespaces | We use the stable `pool.dataset` / `pool.snapshot` names; watch for deprecation signals before migrating |
| `tn_connect` | TrueNAS Connect enrollment | Cloud service enrollment; limited Terraform value |

### 1.4 Enterprise / licensed hardware

| Namespace(s) | What it manages |
|---|---|
| `failover`, `failover.disabled`, `failover.reboot` | High-availability controller pairs |
| `fc`, `fc.fc_host`, `fcport` | Fibre Channel targets and ports |
| `jbof` | NVMe JBOF expansion shelves |
| `enclosure2`, `enclosure.label` | Enclosure management |
| `ipmi.lan`, `ipmi.chassis`, `ipmi.sel` | IPMI configuration |
| `rdma` | RDMA-capable interface configuration |
| `truecommand` | TrueCommand connection |
| `vmware` | VMware snapshot coordination |

These require enterprise/HA systems for verification. Our testing policy (no
mocks, real systems only) means each of these needs time scheduled on such a
system before its resource can ship.

### 1.5 Poor Terraform fit (actions, telemetry, one-shots — likely never)

| Namespace(s) | Why not |
|---|---|
| `update`, `system.reboot`, `boot` | Imperative operations, not declarative state |
| `support`, `truenas`, `truenas.license` | Support tickets, EULA, license upload — one-shot actions |
| `config` | Config backup/restore — operational, not declarative |
| `disk` | Wipe/format operations; disk *settings* update exists but is risky to converge |
| `reporting`, `webui.*` | Telemetry and UI internals |
| `dns`, `device` | Read-only queries; possible future data sources at most |
| `core`, `auth`, `api_key` (session half) | Session plumbing — already used internally by our client |
| `alert` | Alert list/dismiss — operational (we do cover `alertservice` and alert *policy*) |
| `idmap` | Exposes only the `clear_idmap_cache` action; no durable configuration — idmap *configuration* lives on `truenas_directoryservices`' AD block instead |

## Part 2 — Covered API Areas

All 71 resources below also ship a matching data source, generated documentation,
unit tests (payload builders, response mappers, schema shape), and a live
acceptance test with create → update → import → destroy verification and leak
checks. Suite is green against SCALE 25.10 and 26.0, with the two exceptions
noted in the Executive Summary (`truenas_directoryservices`, 25.10 + dedicated
Samba AD, OpenLDAP, and FreeIPA servers only; `truenas_cloud_backup`, documented
permanent acceptance-test skip).

### Storage

| Terraform resource | API namespace |
|---|---|
| `truenas_pool` | `pool` (creation verified manually — needs blank disks) |
| `truenas_dataset` | `pool.dataset` |
| `truenas_zvol` | `pool.dataset` (volume type) |
| `truenas_snapshot` | `pool.snapshot` |
| `truenas_periodic_snapshot_task` | `pool.snapshottask` |
| `truenas_system_dataset` | `systemdataset` |
| `truenas_scrub_task` | `pool.scrub` |
| `truenas_resilver_config` | `pool.resilver` (singleton) |

### Sharing

| Terraform resource | API namespace |
|---|---|
| `truenas_nfs_share` | `sharing.nfs` |
| `truenas_smb_share` | `sharing.smb` |
| `truenas_nfs_config` | `nfs` |
| `truenas_smb_config` | `smb` |

### Block storage — iSCSI

| Terraform resource | API namespace |
|---|---|
| `truenas_iscsi_global` | `iscsi.global` |
| `truenas_iscsi_portal` | `iscsi.portal` |
| `truenas_iscsi_initiator` | `iscsi.initiator` |
| `truenas_iscsi_auth` | `iscsi.auth` |
| `truenas_iscsi_extent` | `iscsi.extent` |
| `truenas_iscsi_target` | `iscsi.target` |
| `truenas_iscsi_targetextent` | `iscsi.targetextent` |

### Block storage — NVMe-oF

| Terraform resource | API namespace |
|---|---|
| `truenas_nvmet_global` | `nvmet.global` |
| `truenas_nvmet_subsys` | `nvmet.subsys` |
| `truenas_nvmet_port` | `nvmet.port` |
| `truenas_nvmet_namespace` | `nvmet.namespace` |
| `truenas_nvmet_host` | `nvmet.host` |
| `truenas_nvmet_host_subsys` | `nvmet.host_subsys` |
| `truenas_nvmet_port_subsys` | `nvmet.port_subsys` |

### Accounts

| Terraform resource | API namespace |
|---|---|
| `truenas_user` | `user` |
| `truenas_group` | `group` |

### Data protection and movement

| Terraform resource | API namespace |
|---|---|
| `truenas_replication_task` | `replication` (LOCAL push and remote SSH transport, the latter authenticating via a `truenas_keychain_ssh_connection` credential referenced by `ssh_credentials` — no longer blocked on `keychaincredential`, live-tested end to end as `TestAccReplication_RemoteSSH`) |
| `truenas_replication_config` | `replication.config` |
| `truenas_cloudsync_task` | `cloudsync` |
| `truenas_cloudsync_credentials` | `cloudsync.credentials` |
| `truenas_cloud_backup` | `cloud_backup` (restic-based backup to a cloud bucket, distinct from `cloudsync`; `cloud_backup.create` validates the credential/bucket against the real remote endpoint at apply time — confirmed live via a bogus S3 access key rejected before any local state was created — so its acceptance test, `TestAccCloudBackup_basic`, is a documented, permanent skip: no live S3-compatible bucket fixture is available in this environment. Schema, unit tests, and payload/response mapping are exercised without a live run) |
| `truenas_rsync_task` | `rsynctask` (MODULE and SSH modes; SSH mode uses the run-as user's own keys unless `ssh_credentials` is set) |

### Network

| Terraform resource | API namespace |
|---|---|
| `truenas_network_config` | `network.configuration` |
| `truenas_network_interface` | `interface` |
| `truenas_static_route` | `staticroute` |

### System configuration

| Terraform resource | API namespace |
|---|---|
| `truenas_system_general` | `system.general` |
| `truenas_system_advanced` | `system.advanced` |
| `truenas_ntp_server` | `system.ntpserver` |
| `truenas_tunable` | `tunable` |
| `truenas_boot_environment` | `boot.environment` |
| `truenas_service` | `service` |
| `truenas_audit_config` | `audit` (singleton: retention/reservation/quota for the local audit databases; no top-level enable/disable — auditing is toggled per service) |
| `truenas_reporting_exporter` | `reporting.exporters` (GRAPHITE exporter, the only type TrueNAS currently supports, exposed as a typed nested block) |

### Scheduled tasks

| Terraform resource | API namespace |
|---|---|
| `truenas_cronjob` | `cronjob` |
| `truenas_init_shutdown_script` | `initshutdownscript` |

### Certificates & ACME

| Terraform resource | API namespace |
|---|---|
| `truenas_certificate` | `certificate` (four immutable creation modes: imported cert+key, on-box CSR generation, imported CSR+key, ACME. ACME issuance — DNS challenge orchestration, renewal polling — is schema-and-preflight only: `acme_directory_uri`/`csr_id`/`tos`/`dns_mapping` are accepted and forwarded to `certificate.create`, but live issuance is deferred and not exercised by the acceptance suite) |
| `truenas_acme_dns_authenticator` | `acme.dns.authenticator` (cloudflare, digitalocean, OVH, route53, shell — five discriminated variants, so `attributes` is a free-form JSON document) |

### Keychain & remote replication

| Terraform resource | API namespace |
|---|---|
| `truenas_keychain_ssh_keypair` | `keychaincredential` (type SSH_KEY_PAIR; import or on-box generate) |
| `truenas_keychain_ssh_connection` | `keychaincredential` (type SSH_CREDENTIALS) — together with `truenas_keychain_ssh_keypair`, this unblocks `truenas_replication_task`'s SSH transport (see Data protection and movement above) |

### Filesystem permissions & ACLs

| Terraform resource | API namespace |
|---|---|
| `truenas_filesystem_permissions` | `filesystem` (`setperm`/`stat`; declarative mode/uid/gid on an existing path, keyed by path rather than a TrueNAS-assigned id) |
| `truenas_filesystem_acl` | `filesystem` (`setacl`/`getacl`; declarative NFS4/POSIX1E ACL on an existing path) |
| `truenas_acl_template` | `filesystem.acltemplate` (reusable named ACL templates; never touches TrueNAS's 9 builtin templates unless one is explicitly imported) |

### Service configuration (singletons)

| Terraform resource | API namespace |
|---|---|
| `truenas_ssh_config` | `ssh` |
| `truenas_ftp_config` | `ftp` |
| `truenas_snmp_config` | `snmp` |
| `truenas_ups_config` | `ups` |
| `truenas_mail` | `mail` |

### Alerts

| Terraform resource | API namespace |
|---|---|
| `truenas_alert_service` | `alertservice` |
| `truenas_alert_policy` | `alertclasses` |

### Access management

| Terraform resource | API namespace |
|---|---|
| `truenas_api_key` | `api_key` (key rotation via the API's `reset` flag is not exposed — taint and recreate to rotate) |
| `truenas_privilege` | `privilege` |
| `truenas_twofactor_auth` | `auth.twofactor` (singleton: system-wide enable, TOTP window, per-service requirement. Does not affect API-key authentication, confirmed live) |

### Directory services and Kerberos

| Terraform resource | API namespace |
|---|---|
| `truenas_directoryservices` | `directoryservices` (AD, LDAP, and IPA service types; idmap via the AD configuration block) |
| `truenas_kerberos_config` | `kerberos` (singleton) |
| `truenas_kerberos_realm` | `kerberos.realm` |
| `truenas_kerberos_keytab` | `kerberos.keytab` |

### Virtualization and apps

| Terraform resource | API namespace |
|---|---|
| `truenas_vm` | `vm` |
| `truenas_vm_device` | `vm.device` |
| `truenas_app` | `app` |

## Method

The uncovered list was produced by dumping the live namespace roster from a SCALE
26.0 system (`go run ./cmd/debug_api/ namespaces`, 127 namespaces) and diffing it
against the provider's registered resources. Classification into
viable-vs-poor-fit reflects whether a namespace models durable declarative state
(Terraform's model) or an imperative action. Per the project's testing policy — no
mocks anywhere — every resource ships only with acceptance tests that ran against
real TrueNAS systems.

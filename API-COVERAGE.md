# TrueNAS API Coverage Report

**Audience:** Engineering management
**Provider:** terraform-provider-truenas
**Surveyed against:** TrueNAS SCALE 26.0 (127 live API namespaces), cross-checked on 25.10
**Date:** 2026-07-22

## Executive Summary

The provider implements **58 resources and 58 matching data sources**, covering the
core storage, sharing, block-storage, accounts, scheduled-task, access-management,
Active Directory/LDAP/IPA/Kerberos, and system-configuration surface of the
TrueNAS SCALE API. Every implemented resource has a full acceptance test (create,
update, import, destroy) run against real TrueNAS boxes on both 25.10 and 26.0,
with one deliberate exception: `truenas_directoryservices` is live-tested on the
25.10 VM plus dedicated Samba AD, OpenLDAP, and FreeIPA servers only —
directory-service tests never run against the production-serving 26.0 box, by
design (see TESTING.md).

Of the 127 API namespaces the middleware exposes, close to half now map to
declarative resources we cover, a smaller share are uncovered but viable Terraform
resources (the backlog, detailed below), and the remainder are actions, telemetry,
or enterprise hardware features that are either a poor fit for Terraform or need
dedicated hardware scheduled for testing.

The highest-value gaps, in rough priority order:

1. **Certificates** — no certificate or ACME management; blocks TLS automation.
2. **Remote replication** — the replication resource supports local push only;
   remote targets require the uncovered `keychaincredential` namespace (SSH
   keypairs and connections).
3. **Scheduled tasks** — cron jobs and boot/shutdown scripts remain uncovered
   (rsync tasks and pool scrub/resilver schedules are now covered — see Part 2).
4. **Filesystem ACLs** — `filesystem`, `filesystem.acltemplate`: POSIX/NFSv4 ACLs
   and permission templates, frequently requested alongside dataset + share
   management.

Directory services no longer belongs on this list: Active Directory, LDAP, and
IPA join, Kerberos (config, realms, keytabs), and explicit AD idmap configuration
are all covered now — see Part 2. The standalone `idmap` API namespace exposes
only a cache-clear action (`clear_idmap_cache`), not durable configuration state —
see Part 1, §1.5 (poor Terraform fit); idmap *configuration* is exposed as a
nested block on `truenas_directoryservices` instead.

## Part 1 — Uncovered API Areas

### 1.1 High-value gaps (viable resources, requested workflows)

| Namespace(s) | What it manages | Notes |
|---|---|---|
| `certificate`, `acme.dns.authenticator` | TLS certificates, CSRs, ACME (Let's Encrypt) DNS validation | Needed before `system_general` UI-certificate management is useful end-to-end |
| `keychaincredential` | SSH keypairs and SSH connections | Prerequisite for remote replication; our `replication` resource is local-only today because of this gap. `rsync_task` (see Part 2) covers SSH-mode rsync without it, using the run-as user's own keys |
| `cronjob` | Scheduled shell commands | Straightforward CRUD |
| `initshutdownscript` | Boot/shutdown hook scripts | Straightforward CRUD |
| `cloud_backup` | TrueCloud (Storj) backup tasks | Same shape as our existing `cloudsync` resource |
| `auth.twofactor` | Two-factor authentication policy | Singleton config |
| `filesystem`, `filesystem.acltemplate` | POSIX/NFSv4 ACLs, permissions, ACL templates | Frequently requested alongside dataset + share management |
| `audit` | Audit subsystem configuration | Singleton config |
| `reporting.exporters` | Metrics export (e.g. Graphite) | Small CRUD surface |

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

All 58 resources below also ship a matching data source, generated documentation,
unit tests (payload builders, response mappers, schema shape), and a live
acceptance test with create → update → import → destroy verification and leak
checks. Suite is green against SCALE 25.10 and 26.0, with the single exception
noted in the Executive Summary (`truenas_directoryservices`, 25.10 + dedicated
Samba AD, OpenLDAP, and FreeIPA servers only).

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
| `truenas_replication_task` | `replication` (local push; remote blocked on `keychaincredential`, see Part 1) |
| `truenas_replication_config` | `replication.config` |
| `truenas_cloudsync_task` | `cloudsync` |
| `truenas_cloudsync_credentials` | `cloudsync.credentials` |
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

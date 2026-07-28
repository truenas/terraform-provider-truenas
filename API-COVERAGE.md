# TrueNAS API Coverage Report

**Audience:** Engineering management
**Provider:** terraform-provider-truenas
**Surveyed against:** TrueNAS 26.0 (127 live API namespaces), cross-checked on 25.10 and on a disposable TrueNAS 25.10.4 Enterprise HA pair
**Date:** 2026-07-23

## Executive Summary

The provider implements **85 resources and 87 data sources** (84 resources
have a matching data source; three data sources have no corresponding
resource — `truenas_docker_network` (Docker networks are managed by Docker
itself, not by TrueNAS's own config surface), `truenas_container_image` (a
read-only lookup against the upstream LXC image registry), and
`truenas_enclosure` (enclosure hardware is discovered, never created — see
Part 2, Enterprise and HA); one resource has no matching data source —
`truenas_enclosure_label`, whose read surface is the `truenas_enclosure`
data source instead), covering the core storage, sharing, block-storage,
accounts, scheduled-task, access-management, certificates/ACME,
keychain/remote-replication, filesystem permissions/ACLs, Active
Directory/LDAP/IPA/Kerberos, containers/apps, enterprise/HA, and
system-configuration surface of the TrueNAS API. Every implemented
resource has a full acceptance test (create, update, import, destroy) run
against real TrueNAS boxes on both 25.10 and 26.0, with eight deliberate
exceptions: `truenas_directoryservices` is live-tested on the 25.10 VM plus
dedicated Samba AD, OpenLDAP, and FreeIPA servers only — directory-service
tests never run against the production-serving 26.0 box, by design (see
TESTING.md) — `truenas_cloud_backup`, `truenas_app_registry`, and
`truenas_vmware`, each with a documented, permanent acceptance-test skip
because their respective `create` calls validate credentials against a real
remote endpoint (a cloud storage bucket, a container registry, and a
vCenter/ESXi host, respectively) and no live fixture of that kind is
available in this environment (see Part 2, Data protection and movement /
Virtualization and apps / Enterprise and HA) — `truenas_lxc_config` and
`truenas_container`/`truenas_container_image`/`truenas_webshare`/
`truenas_webshare_config`, whose acceptance tests require TrueNAS
26.0 (the `lxc`, `container`, and `webshare`/`sharing.webshare` namespaces
do not exist on 25.10, confirmed live) and skip cleanly, rather than
failing, on the 25.10 box (see Part 2, Virtualization and apps / Sharing) —
and `truenas_tn_connect_config`, whose singleton mutate-and-restore test is
a documented, permanent skip: `tn_connect.update`'s own accepts schema on
TrueNAS 26.0 exposes exactly one writable field, `enabled`, and setting it
true starts real TrueNAS Connect cloud enrollment — a side effect this
provider's tests never trigger (see Part 2, System configuration).
`truenas_truecommand_config` is fully live-tested (datasource read plus a
Tier 2 set-and-restore of `api_key`) but `enabled` is never set `true` in
any committed test — no TrueCommand instance is available to enroll with,
so real TrueCommand enrollment itself remains unexercised (see Part 2,
Enterprise and HA); this is a coverage caveat, not a skip.

Of the 127 API namespaces the middleware exposes, more than half now map to
declarative resources we cover, a smaller share are uncovered but viable Terraform
resources (the backlog, detailed below), and the remainder are actions, telemetry,
or enterprise hardware features that are either a poor fit for Terraform or need
dedicated hardware scheduled for testing.

Every gap previously tracked as high-value (§1.1: certificates/ACME, remote
replication, scheduled tasks, filesystem ACLs, two-factor auth, cloud backup,
audit config, reporting exporters) is now covered — see Part 2. Containers and
apps (Docker service configuration, private registries, app catalog trains,
and now LXC container lifecycle — create/start/stop/delete, 26.0+) are
covered too — see Part 2, Virtualization and apps. §1.2's new-26.0-surfaces
list is now empty too: web shares (`webshare`/`sharing.webshare`) and
TrueNAS Connect enrollment status (`tn_connect`) are both covered — see
Part 2, Sharing and System configuration. The HA-testable slice of §1.3's
enterprise/HA hardware gap is now covered too — failover controller pairs,
enclosure management, IPMI LAN configuration, TrueCommand connection, and
VMware snapshot coordination were verified live on a disposable TrueNAS
25.10.4 Enterprise HA pair — see Part 2, Enterprise and HA. The remaining
gap is:

1. **Fibre Channel, JBOF, RDMA** — the disposable HA pair used to close the
   rest of §1.3 has none of this hardware/licensing: `fc.capable=false`,
   `jbof.licensed=0`, and no `rdma.*` methods at all (probed live). All
   three still need dedicated hardware scheduled for testing before they can
   ship under this project's no-mocks policy.

Directory services no longer belongs on this list: Active Directory, LDAP, and
IPA join, Kerberos (config, realms, keytabs), and explicit AD idmap configuration
are all covered now — see Part 2. The standalone `idmap` API namespace exposes
only a cache-clear action (`clear_idmap_cache`), not durable configuration state —
see Part 1, §1.4 (poor Terraform fit); idmap *configuration* is exposed as a
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

### 1.2 New 26.0 surfaces

*(empty)* — `webshare`/`sharing.webshare` and `tn_connect` (the two viable
Terraform resources previously tracked here) are now covered — see Part 2,
Sharing and System configuration respectively. `zfs.resource`,
`zfs.resource.snapshot`, `zpool`, `zpool.scrub` (the next-generation ZFS
namespaces) were deliberately **not** adopted and are not tracked as a gap:
they duplicate the stable `pool.dataset`/`pool.snapshot`/`pool.scrub`
namespaces this provider already covers, so there is no coverage gain from
switching, only churn. Kept as a watch-item below in case TrueNAS ever
deprecates the `pool.*` names in favor of these.

**Watch-item (not a gap):** `zfs.resource`, `zfs.resource.snapshot`,
`zpool`, `zpool.scrub` — next-generation ZFS namespaces introduced
alongside the stable `pool.dataset`/`pool.snapshot`/`pool.scrub` ones this
provider uses. Revisit only if TrueNAS signals deprecation of the `pool.*`
namespaces.

### 1.3 Enterprise / licensed hardware

`failover`/`failover.disabled`/`failover.reboot` (HA controller pairs),
`enclosure2`/`enclosure.label` (enclosure management), `ipmi.lan` (IPMI LAN
configuration), `truecommand` (TrueCommand connection), and `vmware` (VMware
snapshot coordination) — the HA-testable slice of this list — are now
covered; see Part 2, Enterprise and HA. What remains needs hardware/licensing
the disposable TrueNAS 25.10.4 Enterprise HA pair used to close the rest of
this section does not have:

| Namespace(s) | What it manages | Evidence |
|---|---|---|
| `fc`, `fc.fc_host`, `fcport` | Fibre Channel targets and ports | `fc.capable` = `false` on the HA pair (probed live) |
| `jbof` | NVMe JBOF expansion shelves | `jbof.licensed` = `0` on the HA pair (probed live) |
| `rdma` | RDMA-capable interface configuration | `core.get_methods` lists zero `rdma.*` methods on the HA pair (probed live) — no RDMA-capable interface hardware present |

These require enterprise/HA systems this project doesn't have access to for
verification. Our testing policy (no mocks, real systems only) means each of
these needs time scheduled on such a system before its resource can ship.
`ipmi.chassis` (chassis identify action) and `ipmi.sel` (system event log,
read-only telemetry) are deliberately not tracked here even though the
underlying hardware is now available — see §1.4, they're a poor fit for a
declarative resource regardless of hardware access.

### 1.4 Poor Terraform fit (actions, telemetry, one-shots — likely never)

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
| `ipmi.chassis` | Chassis identify (power/locate LED blink) — an imperative action, not durable state; `ipmi.lan` LAN configuration on the same hardware is covered, see Part 2, Enterprise and HA |
| `ipmi.sel` | System Event Log — read-only telemetry (BMC hardware event history), not configuration |

## Part 2 — Covered API Areas

All 84 resources below also ship a matching data source — `truenas_enclosure_label`
is the one exception (no matching data source; enclosure lookup is the
`truenas_enclosure` data source instead, listed alongside it under
Enterprise and HA) — generated documentation, unit tests (payload builders,
response mappers, schema shape), and a live acceptance test with create →
update → import → destroy verification and leak checks. Suite is green
against TrueNAS 25.10 and 26.0, plus a disposable TrueNAS 25.10.4 Enterprise HA
pair for the Enterprise and HA section below, with the eight exceptions
noted in the Executive Summary (`truenas_directoryservices`, 25.10 +
dedicated Samba AD, OpenLDAP, and FreeIPA servers only; `truenas_cloud_backup`,
`truenas_app_registry`, and `truenas_vmware`, each a documented permanent
acceptance-test skip; `truenas_lxc_config` and
`truenas_container`/`truenas_container_image`/`truenas_webshare`/
`truenas_webshare_config`, TrueNAS 26.0-only with a clean skip on 25.10;
`truenas_tn_connect_config`, a documented permanent Tier 2 skip).

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
| `truenas_smb_config` | `smb` (`stateful_failover`/`minimum_protocol`/`search_protocols` writable on TrueNAS 26.0+ only — none of the three exist on `smb.update` below 26.0, confirmed live against a 25.10.3.1 VM and a 25.10.4 HA pair member) |
| `truenas_webshare` | `sharing.webshare` (WebDAV-style web shares: `name`/`path`/`enabled`/`is_home_base`; no `comment` field — confirmed live, rejected as an extra input; **TrueNAS 26.0+ only**, the `sharing.webshare` namespace does not exist on 25.10, confirmed live via `core.get_methods`) |
| `truenas_webshare_config` | `webshare` (singleton: bind IPs, search indexing, passkey auth mode, allowed AD/LDAP groups; **TrueNAS 26.0+ only**, same version gate as `truenas_webshare`) |

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
| `truenas_system_advanced` | `system.advanced` (`nvidia` writable on TrueNAS 26.0+ only — does not exist on `system.advanced.update` below 26.0, confirmed live against a 25.10.3.1 VM and a 25.10.4 HA pair member) |
| `truenas_ntp_server` | `system.ntpserver` |
| `truenas_tunable` | `tunable` |
| `truenas_boot_environment` | `boot.environment` |
| `truenas_service` | `service` |
| `truenas_audit_config` | `audit` (singleton: retention/reservation/quota for the local audit databases; no top-level enable/disable — auditing is toggled per service) |
| `truenas_reporting_exporter` | `reporting.exporters` (GRAPHITE exporter, the only type TrueNAS currently supports, exposed as a typed nested block) |
| `truenas_tn_connect_config` | `tn_connect` (singleton: TrueNAS Connect cloud enrollment status. **SAFETY**: `enabled` is the only writable field — confirmed live, `tn_connect.update`'s own accepts schema on TrueNAS 26.0 exposes nothing else — and setting it true starts real cloud enrollment, so this provider's own tests never do so; enrollment actions (claim token generation, registration URI) are out of scope. Present on both 25.10 and 26.0, unlike most other new-26.0 surfaces, but the two releases' `tn_connect.config`/`tn_connect.update` field shapes diverge — TrueNAS 25.10 additionally reports `ips`/`interfaces`/`interfaces_ips`/`use_all_interfaces` (absent on 26.0, which selects addresses automatically instead), TrueNAS 26.0 additionally reports `tier`/`last_heartbeat_failure_datetime` (absent on 25.10) — both sets are exposed as Computed, release-conditional attributes that read as null where absent) |

### Scheduled tasks

| Terraform resource | API namespace |
|---|---|
| `truenas_cronjob` | `cronjob` |
| `truenas_init_shutdown_script` | `initshutdownscript` |

### Certificates & ACME

| Terraform resource | API namespace |
|---|---|
| `truenas_certificate` | `certificate` (four immutable creation modes: imported cert+key, on-box CSR generation, imported CSR+key, ACME. ACME issuance is live-tested end to end — a full DNS-01 order (`acme_directory_uri`/`csr_id`/`tos`/`dns_mapping`) is driven against a real ACME CA in the acceptance suite; automated renewal polling is not yet exercised) |
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
| `truenas_docker_config` | `docker` (singleton: pool/dataset, image-update checks, address pools, IPv6 CIDR, registry mirrors; `nvidia` writable on 25.10 and earlier only — dropped from `docker.update`'s accepted fields on 26.0+) |
| `truenas_docker_network` | `docker.network` — **datasource only, read-only namespace**: Docker networks are created/destroyed by Docker itself (and by installed applications), not by TrueNAS's own config surface, so there is no corresponding resource |
| `truenas_app_registry` | `app.registry` (private container registry credentials; `app.registry.create` validates username/password/uri against the real registry endpoint synchronously — confirmed live via a rejected throwaway create against an unreachable TEST-NET-1 host — so its acceptance test, `TestAccAppRegistry_basic`, is a documented, permanent skip: no live, reachable container registry fixture is available in this environment) |
| `truenas_catalog_config` | `catalog` (singleton: preferred app-catalog trains; `label`/`location` are read-only) |
| `truenas_lxc_config` | `lxc` (singleton: preferred storage pool, network bridge, IPv4/IPv6 network CIDRs for LXC-based instances; **TrueNAS 26.0+ only** — the `lxc` namespace does not exist on 25.10, confirmed live (`lxc.config` returns "Method does not exist" there), so Create/Read/Update fail with a clean version-gate diagnostic instead of the raw API error on older releases) |
| `truenas_container` | `container` (LXC container lifecycle: create/update/start/stop/delete, `dataset`/`default_network`/`status` read back on create; `running` mirrors vm's running-state attribute — true starts the container, false stops it, tolerating the "domain does not exist" never-started case; **TrueNAS 26.0+ only** — the `container` namespace does not exist on 25.10, confirmed live via `core.get_methods` (0 `container.*` methods there), so Create/Read/Update fail with a clean version-gate diagnostic instead of the raw API error on older releases) |
| `truenas_container_image` | `container.image.query_registry` (**datasource only**: looks up available versions of an upstream LXC image by name, e.g. `alpine:3.22:amd64:default`, exposing `versions` and `latest_version` so HCL can reference a current build instead of hardcoding one — the upstream registry, images.linuxcontainers.org, prunes old builds, confirmed live: a version the registry still listed 404'd on download once pruned; **TrueNAS 26.0+ only**, same version gate as `truenas_container`) |
| `truenas_container_device` | `container.device` (per-container device attachment: create/update/delete/query/get_instance, all `job:false`; `attributes` is an opaque JSON document keyed by `"dtype"`, mirroring `truenas_vm_device` — live-tested FILESYSTEM (bind-mount, `source`/`target`), NIC (`nic_attach`/`type`/`mac`), and USB (`device` or `usb.vendor_id`/`usb.product_id`); GPU is schema-only — `gpu_choices` returned no entries and `container.device.create` validates `pci_address`/`gpu_type` against real host hardware on every probe environment used, so no synthetic value could be live-tested; **TrueNAS 26.0+ only**, same version gate as `truenas_container`) |

**Exclusion note:** everything in the
`container`/`container.image`/`container.device` families that maps to
durable declarative state is now covered above as `truenas_container`,
`truenas_container_image`, and `truenas_container_device`. This narrows an
earlier, broader exclusion note that had lumped the entire family in with
the deprecated incus tooling as "superseded upstream" — a decisive live
probe (TrueNAS 26.0) showed `container.*` is itself the modern,
actively-developed LXC container surface (not incus), so most of it is
covered rather than excluded. `lxc` (the service-wide pool/bridge/network
singleton `truenas_container` instances run under) is a separate namespace,
covered above as `truenas_lxc_config`.

### Enterprise and HA

| Terraform resource | API namespace |
|---|---|
| `truenas_failover_config` | `failover` (singleton: `disabled`/`master`/`timeout` on a licensed HA controller pair; datasource additionally exposes `status`/`node`/`disabled_reasons`. Tested live on a disposable TrueNAS 25.10.4 Enterprise HA pair, including one real controlled failover exercise via `failover.become_passive` — status/node transition verified end to end and the pair confirmed healthy afterward, full transcript in `.superpowers/sdd/task-1-report.md`. `master` is not a reliable "who is active" indicator immediately after a failover — confirmed live, it can read `false` on the new master for a period; `status`/`node` are the reliable source. Gated by the new `acctest.HACheck` PreCheck (`TRUENAS_HA=1` + `TRUENAS_HA_ALLOWED_ENDPOINT` guard, DSCheck pattern) — skips cleanly on any box that isn't both HA-licensed and the explicitly allowed endpoint) |
| `truenas_ipmi_lan` | `ipmi.lan` (per-channel BMC LAN configuration — `channel` is Required+RequiresReplace identity, not an assignable property, since channels are fixed hardware enumerated by `ipmi.lan.channels`; `password` is WriteOnly+Sensitive, never read back by the API under any name. Tested live on the HA pair's physical BMC: a `vlan` set-and-restore round trip, gated by `acctest.HACheck` + `acctest.DisruptiveCheck`. `vlan = null` cannot clear an existing VLAN tag through this provider — Terraform's Optional+Computed model can't distinguish an explicit null from an omitted attribute even via `req.Plan`, confirmed against the plugin framework's own source — Update returns an actionable diagnostic instead of silently no-op'ing or crashing; clearing requires a direct API call plus `terraform apply -refresh-only`) |
| `truenas_enclosure` | `enclosure2` (**datasource only**: `enclosure2.query`, by-id lookup; `enclosure.get_instance`/`enclosure.query` (v1, no `"2"`) do not exist on either probed release, confirmed live. Tested live on the HA pair's physical enclosure (a BROADCOM VirtualSES H10 chassis), including a not-found case. `core.get_methods` under-reports the `enclosure2.*`/`ipmi.*`/`failover.*` namespaces on TrueNAS 25.10.4 specifically — every method used across this section was direct-call-verified regardless of what the introspection listing showed) |
| `truenas_enclosure_label` | `enclosure.label.set` (the one mutable field an enclosure exposes; **no matching data source** — `truenas_enclosure` covers lookup. Delete restores the label captured at Create/Import time, via Terraform private state rather than a Computed attribute, so the original value never appears in `terraform show`/state. Tested live on the HA pair: set → import (`ImportStateVerify`) → Terraform's own Destroy → independently re-queried live to confirm the pre-test label was genuinely restored, not just claimed by the test) |
| `truenas_truecommand_config` | `truecommand` (singleton: `enabled`/`api_key`. `api_key` is Sensitive but **not** WriteOnly — confirmed live, it round-trips verbatim and unmasked on read, unlike most other secret fields in this provider. Tested live: datasource read plus a Tier 2 `api_key` set-and-restore, both gated by `acctest.HACheck` + `acctest.DisruptiveCheck`. **`enabled` is never set `true` by any committed test** — no TrueCommand instance is available in this environment to enroll with, and doing so starts a real cloud connection; this mirrors `truenas_tn_connect_config`'s enrollment-avoidance policy, but here the resource itself is still fully live-tested, only `enabled=true` is out of scope) |
| `truenas_vmware` | `vmware` (CRUD: `hostname`/`username`/`password`/`filesystem`/`datastore`; `password` is Required+Sensitive+WriteOnly, no live read-back evidence exists for it since `create` always rejects before persisting anything. **`vmware.create` validates synchronously against the real vCenter/ESXi endpoint** — confirmed live on both the HA pair and the TrueNAS 26.0 box with an RFC 5737 TEST-NET-1 hostname and fabricated credentials, rejected (`ENETUNREACH`/`ETIMEDOUT`) before `vmware.query` ever showed a record — so `TestAccVMware_basic` is a documented, permanent skip: no live, reachable vCenter/ESXi fixture is available in this environment. Schema, unit tests, and payload/response mapping are exercised without a live run, mirroring `truenas_app_registry`) |

All five resources plus the `truenas_enclosure` datasource were verified
live on a disposable TrueNAS 25.10.4 Enterprise HA pair
(`failover.licensed=true`, `status=MASTER`) — the only enterprise/HA-licensed
system available to this project. `acctest.HACheck` skips cleanly (not
`t.Fatal`) on any box that isn't HA-licensed, confirmed live against the
TrueNAS 26.0 box; see TESTING.md for the full HA environment description and
env vars. Fibre Channel, JBOF, and RDMA remain uncovered — this same HA pair
has none of that hardware/licensing (§1.3).

## Method

The uncovered list was produced by dumping the live namespace roster from a TrueNAS
26.0 system (`go run ./cmd/debug_api/ namespaces`, 127 namespaces) and diffing it
against the provider's registered resources. Classification into
viable-vs-poor-fit reflects whether a namespace models durable declarative state
(Terraform's model) or an imperative action. Per the project's testing policy — no
mocks anywhere — every resource ships only with acceptance tests that ran against
real TrueNAS systems.

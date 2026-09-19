# Test Suite

This provider is tested at two levels: **unit tests** (no TrueNAS required)
and **live tests** that run against a real TrueNAS box.
There are no mock servers anywhere — not in the acceptance suite and not
in the client package. Unit tests are pure-function tests (payload
builders, response mappers, parsers, error classification); everything
that talks to a server talks to a real one. Client-level live tests
(`internal/client`, `TestLive*`) cover the transport itself — auth
mechanisms including SCRAM, error mapping, job polling bailout, and
reconnect-after-drop — and are gated on the same env vars as the
acceptance suite, skipping cleanly when unset. The full suite runs green
against TrueNAS 25.10 and 26.0.

## Quick start

```sh
# Unit tests — no TrueNAS needed
make test

# Live acceptance, safe tier (creates/destroys only its own objects)
export TRUENAS_API_KEY="1-xxxx..."
make testacc-safe

# Live acceptance including singleton set-and-restore tests
make testacc-disruptive
```

## Environment variables

| Variable | Default | Meaning |
|---|---|---|
| `TF_ACC` | unset | Master gate: acceptance tests skip without `TF_ACC=1` |
| `TRUENAS_ENDPOINT` | — (required) | Target box, e.g. `wss://truenas.example.com/api/current` |
| `TRUENAS_API_KEY` | — | Auth (or `TRUENAS_USERNAME` + `TRUENAS_PASSWORD`) |
| `TRUENAS_TEST_POOL` | `tank` | Pool under which test datasets/zvols are created |
| `TRUENAS_DISRUPTIVE` | unset | Enables Tier 2 (singleton set-and-restore) |
| `TRUENAS_HA` | unset | Enables Enterprise HA / failover, IPMI LAN, and enclosure acceptance tests (see HA / Enterprise test environment below) |
| `TRUENAS_HA_ALLOWED_ENDPOINT` | — | Required whenever `TRUENAS_HA=1`; must equal `TRUENAS_ENDPOINT` exactly, or the HA tests `t.Fatal` instead of running — same DSCheck-pattern guard as `TRUENAS_DS_ALLOWED_ENDPOINT`, against accidentally hitting a non-disposable HA pair |
| `TRUENAS_APPS` | unset | Enables the app lifecycle test (pulls container images) |
| `TRUENAS_ACME` | unset | Enables the live ACME issuance test `TestAccCertificate_acmeIssuance` (drives a real DNS-01 order against a Pebble ACME CA); skips otherwise |
| `TRUENAS_ACME_DIRECTORY` | — | Required whenever `TRUENAS_ACME=1`; ACME directory URL, e.g. `https://pebble.example.com:14000/dir` (a trailing slash is added if absent — load-bearing for account reuse across re-runs, see MIDDLEWARE-FINDINGS.md) |
| `TRUENAS_ACME_CHALLTESTSRV` | — | Required whenever `TRUENAS_ACME=1`; pebble-challtestsrv management HTTP base URL, e.g. `http://pebble.example.com:8055` (the DNS-01 shell script POSTs `/set-txt` and `/clear-txt` here) |
| `TRUENAS_ACME_CA_PEM` | — | Required whenever `TRUENAS_ACME=1`; path to (or literal content of) the PEM the ACME **directory endpoint's TLS** is signed by — for Pebble's default setup the self-signed directory cert itself (`openssl s_client -connect <pebble>:14000`), NOT the issuance root from `:15000/roots/0`. Imported into the trusted store so the ACME client trusts the directory |
| `TRUENAS_DS` | unset | Enables directory-services tests against the Samba AD DC (see below) |
| `TRUENAS_DS_DOMAIN` | — | AD realm, e.g. `EXAMPLE.LAN` |
| `TRUENAS_DS_USER` | — | AD admin username, e.g. `Administrator` |
| `TRUENAS_DS_PASSWORD` | — | AD admin password |
| `TRUENAS_DS_ALLOWED_ENDPOINT` | — | Required whenever `TRUENAS_DS=1`; must equal `TRUENAS_ENDPOINT` exactly, or the DS tests `t.Fatal` instead of running — a guard against accidentally domain-joining a shared or production box |
| `TRUENAS_DS_KEYTAB_B64` | unset | Base64 of a real Kerberos keytab (e.g. from `samba-tool domain exportkeytab` on the DC); enables `TestAccKerberosKeytab_basic`, skips otherwise |
| `TRUENAS_DS_LDAP_URL` | — | Generic-LDAP test VM URL, e.g. `ldap://ldap.example.com` (or `ldaps://ldap.example.com` for TLS) |
| `TRUENAS_DS_LDAP_BASEDN` | — | LDAP base DN, e.g. `dc=example,dc=lan` |
| `TRUENAS_DS_LDAP_BINDDN` | — | LDAP bind DN, e.g. `cn=admin,dc=example,dc=lan` |
| `TRUENAS_DS_LDAP_BINDPW` | — | LDAP bind password |
| `TRUENAS_DS_IPA_TARGET` | — | FreeIPA server hostname, e.g. `ipa.example.lan` |
| `TRUENAS_DS_IPA_DOMAIN` | — | FreeIPA domain, e.g. `example.lan` (realm `EXAMPLE.LAN`) |
| `TRUENAS_DS_IPA_PASSWORD` | — | FreeIPA `admin` password |

Helpers in `internal/acctest`: `PreCheck` (TF_ACC + credentials),
`DisruptiveCheck` (adds `TRUENAS_DISRUPTIVE=1`), `HACheck` (adds
`TRUENAS_HA=1` + the `TRUENAS_HA_ALLOWED_ENDPOINT` fatal-guard, DSCheck
pattern, then a live `failover.licensed` probe that **skips** — not
`t.Fatal`s — when false, since an unlicensed box, e.g. the TrueNAS 26.0 box,
is a valid non-HA test target, not a safety violation), `AppsCheck`
(`TRUENAS_APPS=1`), `ACMECheck` (adds `TRUENAS_ACME=1` plus a `t.Fatal` if any
of `TRUENAS_ACME_DIRECTORY` / `TRUENAS_ACME_CHALLTESTSRV` / `TRUENAS_ACME_CA_PEM`
is missing, so the ACME test is portable and self-skips cleanly without the
Pebble infrastructure), `Endpoint()`, `TestPool()`, `RandName(prefix)`
(crypto-random `prefix-xxxxxxxx` names), `RandNQN()` (valid RFC-4122
uuid-style NVMe host NQNs), `Client()` (shared live API client for
CheckDestroy/fixture queries), `ProviderConfig()` (HCL provider block),
`RestoreCall(ctx, method, params...)` — used by the `audit_config` and
`twofactor_auth` Tier-2 tests for both the pre-change read and the
`t.Cleanup`-registered restore write, through the shared `acctest.Client()`
connection, with the same transient-failure retry and reconnect behavior as
`client.CallRead`. That connection sits idle for the whole duration of a
Tier-2 test's Terraform steps (which run over the provider's own, separate
connection) and has occasionally gone stale by the time `t.Cleanup` fires —
a real, observed flake, not a resource defect — so retrying the restore
across transport drops matters even though the Terraform steps themselves
already succeeded.

## Unit tests (1103 functions across 90 packages)

Every resource package carries unit tests for:

- **Payload builders** — exact wire keys; unset Optional+Computed fields
  omitted (never sent as zero values); nullable three-ways
  (null/unknown→omit, explicit zero→JSON null, value→value); create-only
  keys absent from update payloads.
- **Response mappers** — nil pointers → null or documented sentinels;
  embedded-object dual-decodes (`{"id": N}` / bare int / null→error);
  write-only secrets never populated from responses.
- **Schema shape** — Required/Optional/Computed per attribute, plan
  modifiers (RequiresReplace, UseStateForUnknown), Sensitive flags.
- **Datasource schema↔struct match** — a reflection test asserting the
  datasource model's `tfsdk` tags exactly match the datasource schema
  attribute set. This catches a class of bug that is invisible to all other
  unit tests (a mismatch makes the datasource fail on every real Read).
- The `client` package tests the WebSocket protocol against an in-process
  test server: calls, errors, context cancellation, auth, and the CallJob
  job-polling loop including its no-job bail-out.

## Acceptance suite (125 test functions, 88 packages)

### Tier 1 — safe (`make testacc-safe`)

Every test creates its own objects (names prefixed `tf-acc-` with random
suffixes), runs a full lifecycle, and verifies deletion against the API:

1. **Create** + attribute checks (fixtures like parent datasets are created
   in the same config via resource references)
2. **Update** in place, verifying the changed attribute
3. **ImportState** with `ImportStateVerify` (write-only fields listed in
   `ImportStateVerifyIgnore`: passwords, CHAP secrets, DH-CHAP keys,
   `force`, `group_create`, provider configs…)
4. **Destroy** + `CheckDestroy` querying the live API by unique name/ID and
   failing if the object survived

Coverage highlights:

- **Storage**: pool (datasource; creation needs blank disks and is a
  documented manual test), dataset, zvol (resize-up), snapshot
  (`dataset@name` import), periodic_snapshot task, scrub task (per-pool,
  cron schedule). `TestAccScrubTask_basic` self-skips when the target pool
  already has a scrub schedule (TrueNAS allows only one per pool), so it
  commonly shows SKIP in sweeps; full CRUD was verified manually on 25.10
  (see task report).
- **Shares**: NFS and SMB shares on own dataset fixtures (SMB exercises the
  TrueNAS 26.0 `LEGACY_SHARE` purpose/options mapping)
- **iSCSI end-to-end** (`iscsi_targetextent.TestAccISCSIEndToEnd`): portal →
  initiator → CHAP auth (wired into the target group) → zvol → extent →
  target → LUN-0 association, plus per-resource basics in each package
- **NVMe-oF end-to-end** (`nvmet_port_subsys.TestAccNVMeTEndToEnd`): subsys →
  disabled port on :14420 → zvol → namespace → host → host/port
  associations, plus per-resource basics
- **Accounts**: user (primary group via `group_create`, write-only
  password), group (sudo commands)
- **Access management**: api_key (create-once plaintext key, expiry),
  privilege (local/DS group role grants)
- **Directory services and Kerberos**: kerberos realm (KDC/admin-server
  lists), kerberos keytab (`TRUENAS_DS_KEYTAB_B64`-gated, real keytab
  exported from the Samba AD DC), directoryservices Active Directory join
  with explicit idmap (builtin + idmap_domain RID), LDAP join (RFC2307,
  seeded-user visibility check), and IPA join (`TRUENAS_DS=1`-gated, see
  below — three service types, three dedicated directory servers)
- **Data movement**: rsync task (MODULE and SSH modes, cron schedule),
  replication task (LOCAL push with dataset fixtures, plus `TestAccReplication_RemoteSSH`
  exercising transport = SSH end to end with a self-provisioned NOPASSWD sudo
  user and a `truenas_keychain_ssh_connection` credential); `cloud_backup` is
  the one resource in this area with no Tier-1 test — see Never-run tier
  below
- **Certificates & ACME**: certificate (CERTIFICATE_CREATE_IMPORTED and
  CERTIFICATE_CREATE_CSR paths), acme_dns_authenticator (cloudflare variant
  with a syntactically valid but fake token — `acme.dns.authenticator.create`
  does not validate credentials against the real DNS provider for any
  variant except shell, which uniquely performs a local file-existence
  check, so cloudflare is used to keep the test self-contained)
- **Keychain**: keychain_ssh_keypair (both generate=true and imported-key
  paths), keychain_ssh_connection (wired to a keypair fixture)
- **Filesystem permissions and ACLs**: filesystem_permissions (mode/uid/gid
  on a dataset fixture live-tested; `recursive` appears only in
  ImportStateVerifyIgnore, not exercised), filesystem_acl (NFS4 entries
  live-tested; POSIX1E entries, including the named-entry-requires-a-MASK
  rule, are unit-tested plus live probes, not run through the acceptance
  test itself), acl_template (NFS4 templates live-tested; POSIX1E
  templates unit-tested only)
- **Scheduled tasks**: cronjob (schedule block live-tested; stdout/stderr
  flags are not set by the test), init_shutdown_script (COMMAND type at
  POSTINIT live-tested; SCRIPT type and the other timings, PREINIT/
  SHUTDOWN, are unit-tested only)
- **Misc**: static route (TEST-NET-2), NTP server (TEST-NET-3 + `force`),
  alert service (Mail attributes JSON), tunable (delete restores the
  captured `orig_value`), boot environment (clones the active BE, never
  activates), cloudsync credentials (26.0 provider/attributes split),
  service (toggles the stopped `ftp` service and restores), VM (stopped,
  alphanumeric name) and VM DISPLAY device (SPICE + password + distinct
  ports)
- **App** (`TRUENAS_APPS=1` only): syncthing catalog app lifecycle
- **Virtualization and apps, continued**: docker_config (datasource-only Tier
  1 test — the resource itself is Tier 2, see below), docker_network
  (datasource lookup of the built-in `"bridge"` network; self-skips when
  Docker is unconfigured on the target box — `docker.config`'s `pool` is
  null), catalog_config (datasource, includes `catalog.trains`); app_registry
  has no Tier-1 test — see Never-run tier below; lxc_config (datasource-only
  Tier 1 test — the resource itself is Tier 2, see below; **TrueNAS 26.0+
  only** — self-skips cleanly on 25.10 via a version precheck, the `lxc`
  namespace does not exist there, confirmed live); container
  (`TestAccContainer_basic`: image datasource lookup → create running=false
  → update description → start and verify RUNNING → stop → ImportState
  (`image` in `ImportStateVerifyIgnore` — not recoverable from the read-back
  API) → destroy + CheckDestroy by name; RandName `tf-acc-` container on the
  explicit test pool, autostart always false, own object only — same
  container/VM safety convention as `vm`); container_image
  (`TestAccContainerImageDataSource_basic` + `_notFound`: datasource lookup
  of `alpine:3.22:amd64:default`'s `latest_version`/`versions`, plus a clean
  not-found diagnostic for a nonexistent image name — never writes).
  **container and container_image are TrueNAS 26.0+ only** — self-skip cleanly
  on 25.10 via a version precheck, the `container` namespace does not exist
  there (0 `container.*` methods, confirmed live via `core.get_methods`).
  container_device (`TestAccContainerDevice_basic`: RandName container +
  dataset fixtures → attach a FILESYSTEM device (`source` = the dataset's
  mountpoint, `target` = `/data`) → update `target` in place → ImportState →
  destroy + CheckDestroy that both the device and the container are gone;
  never starts the container, so attach/detach only ever rewrites libvirt
  domain XML on disk, never a live bind-mount). **TrueNAS 26.0+ only** —
  self-skips cleanly on 25.10 via a version precheck, the `container.device`
  namespace does not exist there (0 `container.device.*` methods, confirmed
  live via `core.get_methods`). NIC and USB device types were live-tested
  (create/delete round trip against `truenasbr0` and the box's own
  `usb_choices` entry respectively) but are not exercised by the acceptance
  test itself, which stays on FILESYSTEM per the design spec; GPU is
  schema-only (no GPU hardware on the probe box — `gpu_choices` empty, and
  `container.device.create` validates `pci_address`/`gpu_type` against real
  host inventory, confirmed live with a fabricated PCI address).
- **Webshare**: `webshare` (`TestAccWebshare_basic`: create on a RandName
  dataset fixture → update `enabled` in place (this resource's cosmetic
  update field — `sharing.webshare` has no `comment` field at all, confirmed
  live) → ImportState → destroy + CheckDestroy by name); `webshare_config`
  (datasource-only Tier 1 test — the resource itself is Tier 2, see below).
  **Both are TrueNAS 26.0+ only** — self-skip cleanly on 25.10 via a version
  precheck, the `webshare`/`sharing.webshare` namespaces do not exist there
  (0 matching methods, confirmed live via `core.get_methods`).
- **TrueNAS Connect**: `tn_connect_config` (`TestAccTnConnectConfigDataSource_basic`,
  datasource-only Tier 1 test — the resource itself has no Tier 2 test at
  all, see Never-run tier below). Unlike `webshare`/`lxc_config`/
  `container_device`, this namespace needs **no version gate**: probed
  live, `tn_connect.config`/`tn_connect.update` are present on both TrueNAS
  25.10 and 26.0, so the test runs unconditionally on either release.
- **HA / Enterprise** (`TRUENAS_HA=1`-gated, see HA / Enterprise test
  environment below): `failover_config` (`TestAccFailoverConfigDataSource_basic`
  — reads `disabled`/`master`/`timeout` plus the datasource's
  `status`/`node`/`disabled_reasons`), `ipmi_lan`
  (`TestAccIPMILanDataSource_basic` — reads the physical BMC's channel 1
  LAN configuration), `enclosure` (`TestAccEnclosureDataSource_basic` +
  `_notFound` — reads the physical enclosure by id, plus a clean not-found
  diagnostic for a bogus id), `enclosure_label`
  (`TestAccEnclosureLabel_setAndRestore` — full lifecycle: set a `RandName`
  label → import → Terraform's own Destroy → independently re-verified live
  that the original label was restored; gated by `HACheck` only, no
  `DisruptiveCheck` — a cosmetic label carries none of the other HA tests'
  risk), `truecommand_config` (`TestAccTrueCommandConfigDataSource_basic` —
  Tier 1 read). `vmware` has no Tier 1 test — see Never-run tier below.

Safety rules baked into the tests — they must never touch the box's live
objects: iSCSI portal/target id=1, extent id=2; NVMe-oF subsys/port/
namespace/port_subsys id=1 (a box may have live block-storage clients); the management NIC
`enp7s0`; the three stock Debian NTP servers; the active boot environment.
Test portals bind the box IP (0.0.0.0 collides with the live portal on
26.0); NVMe test ports use 14420/14421 disabled.

### Tier 2 — disruptive-lite (`make testacc-disruptive`)

Singleton configs have no create/delete; these tests mutate one cosmetic
field and restore it:

1. Read the current live config via the API **before** any change
2. `t.Cleanup` registers an API-level restore that runs even on failure
3. Terraform applies the change, verifies, then re-applies the original
   value (so update-back is exercised too)

| Package | Field toggled |
|---|---|
| `ftp_config` | `banner` |
| `snmp_config` | `location` |
| `smb_config` | `description` |
| `nfs_config` | `v4_domain` |
| `system_advanced` | `motd` |
| `alert_policy` | one class override (full class map carried and restored) |
| `iscsi_global` | `pool_avail_threshold` (advisory alert threshold) |
| `nvmet_global` | `xport_referral` |
| `replication_config` | `max_parallel_replication_tasks` (+1, restore) |
| `kerberos_config` | `appdefaults_aux` |
| `resilver_config` | `enabled` |
| `mail` | `fromname` — **self-skips if mail is unconfigured** (`fromemail` empty: the API requires it on every update, so the unconfigured state could not be restored) |
| `ups_config` | `description` — **self-skips if UPS is unconfigured** (empty `driver`/`port`, same reasoning) |
| `audit_config` | `quota_fill_warning` |
| `twofactor_auth` | `window` — **SAFETY**: the test never toggles `enabled`, only `window`, to avoid locking out password-based logins mid-run |
| `docker_config` | `enable_image_updates` — **self-skips if Docker is unconfigured** (`docker.config`'s `pool` is null: confirmed on the 25.10 test VM, which has never had Docker set up) |
| `catalog_config` | `preferred_trains` (removes and restores `"community"`) — probed live to succeed regardless of whether Docker/apps is configured (unlike `docker_config`, `catalog.update` does not require a pool), so this test has no self-skip condition and ran on both boxes |
| `lxc_config` | `v4_network` — **TrueNAS 26.0+ only** (self-skips on 25.10, `lxc` namespace absent, confirmed live); **never** sets/reads back `preferred_pool` (same docker-pool safety rule as `docker_config`'s `pool`); additionally self-skips if `lxc.config`'s `preferred_pool` is ever non-null on the target box (LXC in use) — decisive probe against the production 26.0 box found it null, so the test ran there |
| `webshare_config` | `search` — **TrueNAS 26.0+ only** (self-skips on 25.10, `webshare` namespace absent, confirmed live); probed live to be a genuine partial update (unlike `mail`/`ups_config`, `webshare.update` does not require other fields on every call), so this test has no unconfigured-service self-skip condition in practice — the self-skip guard is kept anyway as a defensive mirror of the `mail`/`ups_config` precedent |
| `failover_config` | `timeout` — **`TRUENAS_HA=1`-gated** (`HACheck` + `DisruptiveCheck`); `disabled`/`master` are never exercised by any committed test (a `master` mismatch on the live master node is a real failover trigger — see the real-failover exercise note below) |
| `ipmi_lan` | `vlan` — **`TRUENAS_HA=1`-gated**; restore polls `ipmi.lan.query` until the BMC's LAN controller settles (observed real settle-time behavior, not a bug), and is registered via `t.Cleanup` before the mutating apply; `password`/`apply_remote` are never exercised by any committed test |
| `truecommand_config` | `api_key` — **`TRUENAS_HA=1`-gated**; decisively probed live to round-trip verbatim with `enabled` left `false` and no outbound connection attempted; `enabled` is never set `true` by any committed test (no TrueCommand instance to enroll with) |

### Never-run tier

Unconditionally skipped write tests, each with an in-file rationale and
instructions for manual runs against a disposable box — these can cut off
management access or migrate system state:

- `network_config` (hostname/DNS/gateways)
- `system_general` (management UI address/ports/certificate)
- `ssh_config` (management SSH)
- `system_dataset` (pool migration job)
- `network_interface` bridge creation (global commit/checkin cycle);
  its enp7s0 **datasource** test is read-only and does run
- `pool` creation (requires dedicated blank disks)
- `cloud_backup` — `cloud_backup.create` validates the credential/bucket
  against the real remote endpoint at apply time (confirmed live: a bogus
  S3 access key was rejected before any local state was created); no live
  S3-compatible bucket fixture is available in this environment, so
  `TestAccCloudBackup_basic` is a documented, permanent skip. See the
  in-file doc comment for the decisive probe evidence and instructions to
  enable it against an environment with real cloud storage credentials.
- `app_registry` — `app.registry.create` validates the supplied
  username/password/uri against the real container registry endpoint
  synchronously (confirmed live on TrueNAS 26.0: a throwaway create with
  fabricated credentials against an unreachable TEST-NET-1 host was rejected
  with "Invalid credentials for registry" before anything was persisted); no
  live, reachable container registry fixture is available in this
  environment, so `TestAccAppRegistry_basic` is a documented, permanent
  skip (unconditional `t.Skip`, no `TF_ACC` gate needed). See the in-file
  doc comment for the decisive probe evidence and instructions to enable it
  against an environment with a real container registry.
- `tn_connect_config` Tier 2 set-and-restore — `tn_connect.update`'s own
  `core.get_methods` accepts schema exposes **exactly one** writable
  property on TrueNAS 26.0: `enabled`. There is no cosmetic/informational
  field this resource could safely toggle without touching `enabled`
  (starting real TrueNAS Connect cloud enrollment — forbidden
  unconditionally, see the resource's schema description), so
  `TestAccTnConnectConfig_setAndRestore` is a documented, permanent skip
  (unconditional `t.Skip`, no `TF_ACC` gate needed), mirroring
  `cloud_backup`/`app_registry` above. Independently decisive: the
  production 26.0 box is already enrolled (`enabled=true`, `tier=FOUNDATION`)
  — a real account, not a disposable fixture. TrueNAS 25.10's
  `tn_connect.update` does accept a richer schema (also `ips`, `interfaces`,
  `use_all_interfaces`), and a supplementary live probe there confirmed an
  `ips`-only update leaves `enabled` untouched, but this resource
  deliberately does not expose those fields as writable on any release
  (see the in-file doc comment on `TestAccTnConnectConfig_setAndRestore`,
  which carries the full transcript inline).
- `vmware` — `vmware.create` validates `hostname`/`username`/`password`
  against the real vCenter/ESXi endpoint synchronously (confirmed live on
  BOTH the HA pair and the TrueNAS 26.0 box: a throwaway create with an RFC
  5737 TEST-NET-1 hostname and fabricated credentials was rejected —
  `ENETUNREACH` on the HA pair, `ETIMEDOUT` on 26.0 — before `vmware.query`
  ever showed a record on either box); no live, reachable vCenter/ESXi
  fixture is available in this environment, so `TestAccVMware_basic` is a
  documented, permanent skip (unconditional `t.Skip`, no `TF_ACC` gate
  needed), mirroring `app_registry`/`cloud_backup` above. See the in-file
  doc comment for the decisive probe evidence and instructions to enable it
  against an environment with a real vCenter/ESXi host.

## HA / Enterprise test environment

Tests gated on `TRUENAS_HA=1` (`failover_config`, `ipmi_lan`, `enclosure`,
`enclosure_label`; `truecommand_config`/`vmware` are not HA-gated — they run
on any box) need a licensed Enterprise HA controller pair with a BMC/IPMI
channel and a supported enclosure.

- **Env vars**: `TRUENAS_HA=1` plus `TRUENAS_HA_ALLOWED_ENDPOINT` set to
  exactly `TRUENAS_ENDPOINT` — enforced by `acctest.HACheck`'s DSCheck-pattern
  guard, `t.Fatal` on mismatch or omission, so an HA suite can never point at
  the wrong box. `TRUENAS_DISRUPTIVE=1` is additionally required for the
  Tier 2 set-and-restore tests (`failover_config` `timeout`, `ipmi_lan`
  `vlan`, `truecommand_config` `api_key`).
- **Skip behavior on a non-HA box**: `HACheck` probes `failover.licensed`
  live and `t.Skip`s (not `t.Fatal`s) when false, so the HA-gated set runs
  cleanly against an unlicensed box with zero mutating calls.
- **Use a disposable pair.** These tests can trigger real failover events
  (`failover.become_passive`) and can swap the master/backup assignment.
  Never point them at a production or shared HA pair. Revoke any API key
  created for the run afterward.

## Directory-services test environment

Tests gated on `TRUENAS_DS=1` (Active Directory join/idmap/etc.) need a real
domain controller — for example a disposable Samba `samba-ad-dc` VM. Point the
TrueNAS box under test at the DC and supply the realm credentials:

- **Env vars**: `TRUENAS_DS_DOMAIN` (realm, e.g. `EXAMPLE.LAN`),
  `TRUENAS_DS_USER` (e.g. `Administrator`), `TRUENAS_DS_PASSWORD`.
- **Prerequisite**: the TrueNAS box under test must have its nameserver
  pointed at the DC so it can resolve the realm's SRV/A records — the
  acceptance run flips this via `network.configuration.update` and restores
  the box's original nameserver afterward, pass or fail.
- **Disposable-box guard**: `DSCheck` (`internal/acctest`) requires
  `TRUENAS_DS_ALLOWED_ENDPOINT` to be set and to exactly match
  `TRUENAS_ENDPOINT` whenever `TRUENAS_DS=1` — it `t.Fatal`s rather than
  skipping if the guard is missing or mismatched, since an unintended AD join
  is far more disruptive than a skipped test.
- **Keytab fixture**: `TestAccKerberosKeytab_basic` additionally needs
  `TRUENAS_DS_KEYTAB_B64`, base64 of a real keytab (e.g. `samba-tool domain
  exportkeytab /tmp/tfacc.keytab --principal=Administrator@EXAMPLE.LAN` then
  `base64 -w0 /tmp/tfacc.keytab`). The test self-skips when unset, so plain
  Tier-1 sweeps stay green without a DC.

### Generic LDAP test server (RFC2307)

Plain (non-AD) LDAP directory-service tests need an OpenLDAP (`slapd`) server
seeded with RFC2307 posix data (posixAccount/posixGroup under an `ou=People`
/ `ou=Group` tree):

- **Env vars**: `TRUENAS_DS_LDAP_URL` (`ldap://host` or `ldaps://host`),
  `TRUENAS_DS_LDAP_BASEDN` (e.g. `dc=example,dc=lan`),
  `TRUENAS_DS_LDAP_BINDDN` (e.g. `cn=admin,dc=example,dc=lan`),
  `TRUENAS_DS_LDAP_BINDPW`.
- **TLS**: with a self-signed server cert, clients must tolerate it
  (`LDAPTLS_REQCERT=allow` for `ldapsearch`, or set
  `OPT_X_TLS_REQUIRE_CERT`/`OPT_X_TLS_NEWCTX` globally *before*
  `ldap.initialize()` for python-ldap — those options are process-global, not
  per-connection).

### FreeIPA test server

FreeIPA (Kerberos + LDAP + DNS) tests need an `ipa-server` whose own FQDN is
its hostname and which serves an authoritative DNS zone for its domain
(install with `ipa-server-install --setup-dns`):

- **Env vars**: `TRUENAS_DS_IPA_TARGET` (server FQDN, e.g.
  `ipa.example.lan`), `TRUENAS_DS_IPA_DOMAIN` (e.g. `example.lan`, realm
  `EXAMPLE.LAN`), `TRUENAS_DS_IPA_PASSWORD` (the `admin` password).

Keep all directory-service credentials out of the repo — supply them through
the environment at run time.

## Operational notes

- **Run packages sequentially.** TrueNAS rate-limits authentication
  (~20/min); every Terraform plan/apply opens a fresh connection. The
  provider retries rate-limited auth with backoff (5/10/20/30s), but
  parallel suites will still trip it. The make targets and the batch
  scripts sleep between packages.
  Token caching was prototyped and rejected: on TrueNAS 26.0,
  `auth.login_with_token` draws from the same rate bucket as key login,
  and a generated token authenticates exactly one new session (see
  `cmd/token_probe` for the experiment and results). Fewer, larger applies
  and paced test runs are the only real levers. The auth rate limit applies
  identically to the HA box — pace HA-gated runs the same way.
- **Runtimes**: most packages 20–40s; the full safe tier ~25 minutes with
  pauses.
- **Leftovers**: a failed run can strand `tf-acc-*` objects. Find them with
  `go run ./cmd/debug_api/ <section>` queries and delete via the UI or API;
  every test object carries the prefix.
- **`cmd/debug_api`** is the discovery/diagnosis tool used to build this
  suite: `methods <prefix>` dumps live method schemas (check the `"job"`
  flag), `namespaces` lists the API surface, and the `smbprobe`/`csprobe`
  sections create-inspect-delete throwaway objects to capture real response
  shapes.

## Writing a new acceptance test

1. Copy the closest sibling (`dataset` for simple CRUD,
   `iscsi_targetextent` for multi-resource wiring, `snmp_config` for
   singleton set-and-restore).
2. `acctest.RandName` every object; fixtures under `acctest.TestPool()`.
3. Full contract: Create+checks → Update → ImportState(Verify+ignores) →
   CheckDestroy with a real API query that would catch a leak.
4. Verify wire assumptions against the live box first
   (`go run ./cmd/debug_api/ methods <ns>.create`) — the API's `"job"`
   flags and field sets differ between TrueNAS releases, and every mismatch
   this suite found came from trusting stale shapes.
5. Never reference existing objects on the target box; if the resource is a
   singleton, use `DisruptiveCheck` + set-and-restore, and self-skip when
   the singleton is unconfigured.

# Test Suite

This provider is tested at two levels: **unit tests** (no TrueNAS required)
and **live tests** that run against a real TrueNAS SCALE box.
There are no mock servers anywhere — not in the acceptance suite and not
in the client package. Unit tests are pure-function tests (payload
builders, response mappers, parsers, error classification); everything
that talks to a server talks to a real one. Client-level live tests
(`internal/client`, `TestLive*`) cover the transport itself — auth
mechanisms including SCRAM, error mapping, job polling bailout, and
reconnect-after-drop — and are gated on the same env vars as the
acceptance suite, skipping cleanly when unset. The full suite runs green
against TrueNAS SCALE 25.10 and 26.0.

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
| `TRUENAS_APPS` | unset | Enables the app lifecycle test (pulls container images) |
| `TRUENAS_DS` | unset | Enables directory-services tests against the Samba AD DC (see below) |
| `TRUENAS_DS_DOMAIN` | — | AD realm, e.g. `TFTEST.LAN` |
| `TRUENAS_DS_USER` | — | AD admin username, e.g. `Administrator` |
| `TRUENAS_DS_PASSWORD` | — | AD admin password |
| `TRUENAS_DS_ALLOWED_ENDPOINT` | — | Required whenever `TRUENAS_DS=1`; must equal `TRUENAS_ENDPOINT` exactly, or the DS tests `t.Fatal` instead of running — a guard against accidentally domain-joining a shared or production box |
| `TRUENAS_DS_KEYTAB_B64` | unset | Base64 of a real Kerberos keytab (e.g. from `samba-tool domain exportkeytab` on the DC); enables `TestAccKerberosKeytab_basic`, skips otherwise |
| `TRUENAS_DS_LDAP_URL` | — | Generic-LDAP test VM URL, e.g. `ldap://192.168.1.251` (or `ldaps://192.168.1.251` for TLS) |
| `TRUENAS_DS_LDAP_BASEDN` | — | LDAP base DN, e.g. `dc=tftest-ldap,dc=lan` |
| `TRUENAS_DS_LDAP_BINDDN` | — | LDAP bind DN, e.g. `cn=admin,dc=tftest-ldap,dc=lan` |
| `TRUENAS_DS_LDAP_BINDPW` | — | LDAP bind password |
| `TRUENAS_DS_IPA_TARGET` | — | FreeIPA server hostname, e.g. `ipa.tfipa.lan` |
| `TRUENAS_DS_IPA_DOMAIN` | — | FreeIPA domain, e.g. `tfipa.lan` (realm `TFIPA.LAN`) |
| `TRUENAS_DS_IPA_PASSWORD` | — | FreeIPA `admin` password |

Helpers in `internal/acctest`: `PreCheck` (TF_ACC + credentials),
`DisruptiveCheck` (adds `TRUENAS_DISRUPTIVE=1`), `AppsCheck`
(`TRUENAS_APPS=1`), `Endpoint()`, `TestPool()`, `RandName(prefix)`
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

## Unit tests (1070 functions across 85 packages)

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

## Acceptance suite (119 test functions, 84 packages)

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
  SCALE 26.0 `LEGACY_SHARE` purpose/options mapping)
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
  Tier 1 test — the resource itself is Tier 2, see below; **SCALE 26.0+
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
  **container and container_image are SCALE 26.0+ only** — self-skip cleanly
  on 25.10 via a version precheck, the `container` namespace does not exist
  there (0 `container.*` methods, confirmed live via `core.get_methods`).
  container_device (`TestAccContainerDevice_basic`: RandName container +
  dataset fixtures → attach a FILESYSTEM device (`source` = the dataset's
  mountpoint, `target` = `/data`) → update `target` in place → ImportState →
  destroy + CheckDestroy that both the device and the container are gone;
  never starts the container, so attach/detach only ever rewrites libvirt
  domain XML on disk, never a live bind-mount). **SCALE 26.0+ only** —
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
  **Both are SCALE 26.0+ only** — self-skip cleanly on 25.10 via a version
  precheck, the `webshare`/`sharing.webshare` namespaces do not exist there
  (0 matching methods, confirmed live via `core.get_methods`).
- **TrueNAS Connect**: `tn_connect_config` (`TestAccTnConnectConfigDataSource_basic`,
  datasource-only Tier 1 test — the resource itself has no Tier 2 test at
  all, see Never-run tier below). Unlike `webshare`/`lxc_config`/
  `container_device`, this namespace needs **no version gate**: probed
  live, `tn_connect.config`/`tn_connect.update` are present on both SCALE
  25.10 and 26.0, so the test runs unconditionally on either release.

Safety rules baked into the tests — they must never touch the box's live
objects: iSCSI portal/target id=1, extent id=2; NVMe-oF subsys/port/
namespace/port_subsys id=1 (serving Proxmox storage); the management NIC
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
| `lxc_config` | `v4_network` — **SCALE 26.0+ only** (self-skips on 25.10, `lxc` namespace absent, confirmed live); **never** sets/reads back `preferred_pool` (same docker-pool safety rule as `docker_config`'s `pool`); additionally self-skips if `lxc.config`'s `preferred_pool` is ever non-null on the target box (LXC in use) — decisive probe against the production 26.0 box found it null, so the test ran there |
| `webshare_config` | `search` — **SCALE 26.0+ only** (self-skips on 25.10, `webshare` namespace absent, confirmed live); probed live to be a genuine partial update (unlike `mail`/`ups_config`, `webshare.update` does not require other fields on every call), so this test has no unconfigured-service self-skip condition in practice — the self-skip guard is kept anyway as a defensive mirror of the `mail`/`ups_config` precedent |

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
  synchronously (confirmed live on SCALE 26.0: a throwaway create with
  fabricated credentials against an unreachable TEST-NET-1 host was rejected
  with "Invalid credentials for registry" before anything was persisted); no
  live, reachable container registry fixture is available in this
  environment, so `TestAccAppRegistry_basic` is a documented, permanent
  skip (unconditional `t.Skip`, no `TF_ACC` gate needed). See the in-file
  doc comment for the decisive probe evidence and instructions to enable it
  against an environment with a real container registry.
- `tn_connect_config` Tier 2 set-and-restore — `tn_connect.update`'s own
  `core.get_methods` accepts schema exposes **exactly one** writable
  property on SCALE 26.0: `enabled`. There is no cosmetic/informational
  field this resource could safely toggle without touching `enabled`
  (starting real TrueNAS Connect cloud enrollment — forbidden
  unconditionally, see the resource's schema description), so
  `TestAccTnConnectConfig_setAndRestore` is a documented, permanent skip
  (unconditional `t.Skip`, no `TF_ACC` gate needed), mirroring
  `cloud_backup`/`app_registry` above. Independently decisive: the
  production 26.0 box is already enrolled (`enabled=true`, `tier=FOUNDATION`)
  — a real account, not a disposable fixture. SCALE 25.10's
  `tn_connect.update` does accept a richer schema (also `ips`, `interfaces`,
  `use_all_interfaces`), and a supplementary live probe there confirmed an
  `ips`-only update leaves `enabled` untouched, but this resource
  deliberately does not expose those fields as writable on any release
  (see the in-file doc comment on `TestAccTnConnectConfig_setAndRestore`
  and `.superpowers/sdd/task-3-report.md` for the full transcript).

## Directory-services test environment

Tests gated on `TRUENAS_DS=1` (Active Directory join/idmap/etc.) need a real
domain controller. A dedicated one exists purely for this:

- **VM**: id 210 on Proxmox node `pve`, named `tftest-dc`, static IP
  `192.168.1.250/24`, Debian 13 (trixie) + Samba 4 (`samba-ad-dc`), 2 vCPU /
  2 GB RAM / 20 G disk. Login: `ssh root@192.168.1.250` (root's authorized
  key is the same one used for the other `pve`-hosted test VMs). The
  TrueNAS 25.10 test VM (id 110, `192.168.1.249`) is a separate, unrelated
  VM — it must stay running and untouched by DC changes.
- **Realm**: `TFTEST.LAN`, NetBIOS domain `TFTEST`. The DC's own resolver
  is pinned to `127.0.0.1` (Samba's internal DNS backend) with search
  domain `tftest.lan`; `/etc/resolv.conf` on the DC is `chattr +i`-locked
  so nothing rewrites it back to a DHCP/cloud-init value.
- **Credentials**: the generated `Administrator` password lives only on
  `pve`, at `/root/tftest-dc-admin.pass` (root-only, `chmod 600`) — it is
  intentionally not recorded in this repo. Export it locally as
  `TRUENAS_DS_PASSWORD` when running DS tests, alongside
  `TRUENAS_DS_DOMAIN=TFTEST.LAN` and `TRUENAS_DS_USER=Administrator`.
- **Prerequisite for a DS run**: the TrueNAS box under test must have its
  nameserver pointed at the DC (`192.168.1.250`) so it can resolve the
  realm's SRV/A records — the acceptance run flips this via
  `midclt call network.configuration.update` and must restore the box's
  original nameserver afterward, pass or fail. Outside of a DS run the
  TrueNAS box's nameserver is left at its normal value; do not point it at
  the DC as a standing change.
- **Disposable-VM guard**: `DSCheck` (`internal/acctest`) requires
  `TRUENAS_DS_ALLOWED_ENDPOINT` to be set and to exactly match
  `TRUENAS_ENDPOINT` whenever `TRUENAS_DS=1` — it `t.Fatal`s rather than
  skipping if the guard is missing or mismatched, since an unintended AD
  join is far more disruptive than a skipped test.
- **Keytab fixture**: `TestAccKerberosKeytab_basic` additionally needs
  `TRUENAS_DS_KEYTAB_B64`, a base64-encoded real keytab. Generate one on
  the DC: `samba-tool domain exportkeytab /tmp/tfacc.keytab
  --principal=Administrator@TFTEST.LAN` then `base64 -w0 /tmp/tfacc.keytab`.
  The test self-skips (does not fail) when this var is unset, so plain
  Tier-1 sweeps stay green without touching the DC.

### Generic LDAP test VM (RFC2307)

A second dedicated VM covers plain (non-AD) LDAP directory service testing:

- **VM**: id 211 on `pve`, named `tftest-ldap`, static IP `192.168.1.251/24`,
  Debian 13 (trixie) + OpenLDAP (`slapd`), 1 vCPU / 1 GB RAM / 10 G disk.
  Login: `ssh root@192.168.1.251` via `pve` (same authorized key as the
  other `pve`-hosted test VMs — there is no direct route to this VM's
  subnet from outside `pve`, so proxy through it: `ssh root@pve ssh
  root@192.168.1.251 ...`).
- **Directory**: base DN `dc=tftest-ldap,dc=lan`, admin bind DN
  `cn=admin,dc=tftest-ldap,dc=lan`. RFC2307 data seeded under
  `ou=People`/`ou=Group`: `tfuser1` (uidNumber 21001), `tfuser2` (uidNumber
  21002) as posixAccount+inetOrgPerson, primary group `tfgroup` (gidNumber
  21100, posixGroup).
- **TLS**: self-signed cert (`/etc/ldap/ssl/ldap.{crt,key}`, CN
  `tftest-ldap.lan`). Both `ldaps://` (636) and StartTLS on `ldap://` (389)
  work; clients must tolerate the self-signed cert (`LDAPTLS_REQCERT=allow`
  for `ldapsearch`, or set `OPT_X_TLS_REQUIRE_CERT`/`OPT_X_TLS_NEWCTX`
  globally *before* `ldap.initialize()` for python-ldap clients — those
  options are process-global, not per-connection).
- **Credentials**: the generated `cn=admin` password lives only on `pve`,
  at `/root/tftest-ldap-admin.pass` (root-only, `chmod 600`) — not recorded
  in this repo. Export it as `TRUENAS_DS_LDAP_BINDPW`, alongside
  `TRUENAS_DS_LDAP_URL=ldap://192.168.1.251` (or `ldaps://192.168.1.251`),
  `TRUENAS_DS_LDAP_BASEDN=dc=tftest-ldap,dc=lan`, and
  `TRUENAS_DS_LDAP_BINDDN=cn=admin,dc=tftest-ldap,dc=lan`.
- **Verification**: `ldapsearch` against all three access modes (plain,
  StartTLS, ldaps) from `pve` (the reachable vantage point standing in for
  "workstation" — this environment's sandbox has no route to the VM subnet)
  and from the TrueNAS 25.10 test VM. The TrueNAS box has no `ldapsearch`/
  `ldap-utils` (package management is disabled on TrueNAS appliances), but
  it does ship `python3-ldap` as a middleware dependency, which works fine
  for ad hoc verification via a short script.

### FreeIPA test VM

A third VM covers FreeIPA (identity management combining Kerberos, LDAP,
and DNS):

- **VM**: id 212 on `pve`, named `tftest-ipa`, static IP `192.168.1.252/24`,
  Rocky Linux 9 + `ipa-server`, 2 vCPU / 4 GB RAM / 20 G disk. The Proxmox
  VM name is `tftest-ipa`, but the OS hostname is pinned to `ipa.tfipa.lan`
  (`hostnamectl set-hostname` + a matching `/etc/hosts` entry) since
  FreeIPA requires the server's own FQDN as its hostname. Login: `ssh
  root@192.168.1.252` via `pve`, same as the LDAP VM above.
- **Realm**: `TFIPA.LAN`, domain `tfipa.lan`. Installed unattended via
  `ipa-server-install --realm=TFIPA.LAN --domain=tfipa.lan
  --hostname=ipa.tfipa.lan --setup-dns --no-forwarders --no-ntp
  --unattended` with generated Directory Manager and `admin` passwords.
  `--setup-dns --no-forwarders` gives the VM its own authoritative DNS zone
  for `tfipa.lan` (including SRV records) without forwarding unrelated
  queries upstream.
- **Credentials**: the generated Directory Manager and `admin` passwords
  live only on `pve`, at `/root/tftest-ipa-admin.pass` (root-only, `chmod
  600`) — not recorded in this repo. Export the `admin` password as
  `TRUENAS_DS_IPA_PASSWORD`, alongside `TRUENAS_DS_IPA_TARGET=ipa.tfipa.lan`
  and `TRUENAS_DS_IPA_DOMAIN=tfipa.lan`.
- **Verification**: `kinit admin@TFIPA.LAN` + `ipa user-find` on the VM;
  `host -t SRV _ldap._tcp.tfipa.lan 127.0.0.1` and `host -t SRV
  _kerberos._tcp.tfipa.lan 127.0.0.1` confirm the DNS zone's SRV records;
  the same SRV lookups were repeated from the TrueNAS 25.10 test VM
  querying `192.168.1.252` directly, confirming FreeIPA's DNS answers
  off-box. The TrueNAS box's own nameserver is left untouched by this
  verification — pointing it at `192.168.1.252` as a standing change is a
  later task's concern, not this one.

### Networking quirk shared by all `pve` directory-service VMs

This Proxmox network segment's IPv4 path does not carry working DNS/general
package-mirror egress for VMs on it (the cloud-init-assigned `nameserver
192.168.1.1` resolves nothing usable), but IPv6 egress works via SLAAC.
Debian cloud images resolve fine as-is (mirrors have AAAA records). Rocky
9's genericcloud image needed `/etc/resolv.conf` pointed at an IPv6
resolver (`2606:4700:4700::1111` / `2606:4700:4700::1001`) before `dnf`
would succeed — apply this early on any new Rocky/Alma VM on this segment,
before running `dnf` or `ipa-server-install`.

## Operational notes

- **Run packages sequentially.** TrueNAS rate-limits authentication
  (~20/min); every Terraform plan/apply opens a fresh connection. The
  provider retries rate-limited auth with backoff (5/10/20/30s), but
  parallel suites will still trip it. The make targets and the batch
  scripts sleep between packages.
  Token caching was prototyped and rejected: on SCALE 26.0,
  `auth.login_with_token` draws from the same rate bucket as key login,
  and a generated token authenticates exactly one new session (see
  `cmd/token_probe` for the experiment and results). Fewer, larger applies
  and paced test runs are the only real levers.
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
   flags and field sets differ between SCALE releases, and every mismatch
   this suite found came from trusting stale shapes.
5. Never reference existing objects on the target box; if the resource is a
   singleton, use `DisruptiveCheck` + set-and-restore, and self-skip when
   the singleton is unconfigured.

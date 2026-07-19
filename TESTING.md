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

Helpers in `internal/acctest`: `PreCheck` (TF_ACC + credentials),
`DisruptiveCheck` (adds `TRUENAS_DISRUPTIVE=1`), `AppsCheck`
(`TRUENAS_APPS=1`), `Endpoint()`, `TestPool()`, `RandName(prefix)`
(crypto-random `prefix-xxxxxxxx` names), `RandNQN()` (valid RFC-4122
uuid-style NVMe host NQNs), `Client()` (shared live API client for
CheckDestroy/fixture queries), `ProviderConfig()` (HCL provider block).

## Unit tests (~527 functions across 51 packages)

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

## Acceptance suite (67 test functions, 49 packages)

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
  (`dataset@name` import), periodic_snapshot task
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
- **Misc**: static route (TEST-NET-2), NTP server (TEST-NET-3 + `force`),
  alert service (Mail attributes JSON), tunable (delete restores the
  captured `orig_value`), boot environment (clones the active BE, never
  activates), cloudsync credentials (26.0 provider/attributes split),
  replication task (LOCAL push with dataset fixtures), service (toggles the
  stopped `ftp` service and restores), VM (stopped, alphanumeric name) and
  VM DISPLAY device (SPICE + password + distinct ports)
- **App** (`TRUENAS_APPS=1` only): syncthing catalog app lifecycle

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
| `mail` | `fromname` — **self-skips if mail is unconfigured** (`fromemail` empty: the API requires it on every update, so the unconfigured state could not be restored) |
| `ups_config` | `description` — **self-skips if UPS is unconfigured** (empty `driver`/`port`, same reasoning) |

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

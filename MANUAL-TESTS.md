# terraform-provider-truenas — Manual Test Catalog

**Audience:** QA engineers executing the provider by hand
**Companion documents:** `TEST-PLAN.md` (strategy, environments, coverage
matrix, safety rules), `TESTING.md` (automated-suite operator reference),
`API-COVERAGE.md` (coverage vs. the middleware API surface)

---

## How to use this catalog

Each case is a self-contained manual procedure a QA engineer can run without
reading the codebase. Cases are grouped by domain and numbered with a stable
`MT-<DOMAIN>-NNN` id.

Every resource case follows the same shape:

- **Resource** — the Terraform resource/datasource under test
- **Type** — CRUD, Singleton set-and-restore, Datasource, Integration,
  Manual-only, or Manual-only **DANGEROUS**
- **Environment** — which system role (see TEST-PLAN.md §4), any release gate
  (e.g. 26.0+), any special test gate
- **Preconditions** — fixtures, generated keys/certs, safety notes
- **Config** — a complete HCL block (secrets via variables; object names
  prefixed `tf-acc-manual-`; TEST-NET addresses where an address is needed)
- **Steps** — `terraform` commands plus verification through both the UI and
  the API (`midclt call <method> ...`), an update, an import, a destroy, and a
  post-destroy check
- **Expected results** — concrete pass criteria
- **Cleanup** — how to remove leftovers if a step fails

### Running any case

```sh
# Build the provider and point Terraform at it (no registry install):
go build -o ./bin/terraform-provider-truenas .
cat > dev.tfrc <<EOF
provider_installation {
  dev_overrides { "truenas/truenas" = "$PWD/bin" }
  direct {}
}
EOF
export TF_CLI_CONFIG_FILE=$PWD/dev.tfrc
export TF_VAR_truenas_endpoint='wss://<box>/api/current'
export TF_VAR_truenas_api_key='<id>-<secret>'
# then, in the case's config directory:
terraform plan && terraform apply
```

`terraform init` is neither needed nor wanted under a dev override.

### Conventions and standing cautions

- **No plan-time validation of enums/ranges.** The provider registers no
  Terraform-level value validators; a bad enum or out-of-range value is
  rejected at *apply* time as an API `[EINVAL]`, not blocked at plan time.
  Cases that exercise bad input expect an apply-time error.
- **"Write-only" wording.** Some fields described elsewhere as "write-only"
  (e.g. dataset `share_type`, zvol `sparse`, snapshot `recursive`) are
  ordinary `Optional` attributes — they remain visible in the state file at
  their last-configured value. Genuine protocol WriteOnly fields (passwords,
  bind passwords, CHAP/registry secrets, SSH/keytab-adjacent secrets) never
  appear in state and are listed in each case's `ImportStateVerifyIgnore`
  note. A few "secrets" are returned intact by the API and are modeled
  Sensitive-but-persisted (certificate private keys, kerberos keytab `file`,
  SSH private keys) — their cases verify the value round-trips.
- **Safety.** On any production-serving system, run **only** the cases marked
  safe for it; never touch pre-existing objects (iSCSI portal/target/extent
  id=1, NVMe subsys/port/namespace id=1, the active boot environment, the UI
  certificate id=1, builtin ACL templates and privileges, the management NIC,
  existing shares/containers/registries). Cases marked **DANGEROUS** can cut
  management access, migrate system state, join a domain, or start cloud
  enrollment — run them only on a fully disposable box, and read the case's
  ⚠ Safety block first.
- **Pacing.** TrueNAS rate-limits authentication (~20 logins/60s/IP). Run
  cases sequentially; if several fail in a burst with `[EBUSY] Rate Limit
  Exceeded` or `not connected`, that is the limiter, not the provider —
  re-run the affected cases individually, paced.

---

## Table of contents

- **Section 0 — Provider configuration and authentication** (`MT-PROVIDER`)
- **Section 1 — Storage and shares** (`MT-STORAGE`, 20 cases)
- **Section 2 — Block storage: iSCSI and NVMe-oF** (`MT-BLOCK`, 18 cases)
- **Section 3 — Accounts, access, and directory services** (`MT-IDENTITY`, 13 cases)
- **Section 4 — Certificates, keychain, replication, and tasks** (`MT-TASKS`, 22 cases)
- **Section 5 — Filesystem, apps, and containers** (`MT-APPS`, 21 cases)
- **Section 6 — Network, system, services, and alerts** (`MT-SYSTEM`, 26 cases)
- **Section 7 — Virtualization and HA / Enterprise** (`MT-HA`, 16 cases)

Total: 136 resource cases plus the provider/authentication suite.

---

## 0. Provider Configuration and Authentication

Source: `internal/provider/provider.go` (schema, `Configure`),
`internal/client/client.go`, `auth.go`, `scram.go`, `tls.go`, and `SCRAM.md`.

### 0.1 Endpoint forms

| # | Case | Steps | Expect |
|---|---|---|---|
| 0.1.1 | Native `wss://` endpoint | `endpoint = "wss://<host>/api/current"`, valid `api_key`. `terraform apply` on a trivial data source (e.g. `truenas_user` lookup of an existing user). | Connects and authenticates; no endpoint rewrite. |
| 0.1.2 | Legacy `/websocket` endpoint auto-rewrite | `endpoint = "wss://<host>/websocket"`. Apply as above. | Provider rewrites to `wss://<host>/api/current` transparently (per `strings.CutSuffix(endpoint, "/websocket")` in `provider.go`); connects successfully. Confirm via server-side session/audit log that the connection landed on `/api/current`, not `/websocket`. |
| 0.1.3 | Missing endpoint | Omit `endpoint` and unset `TRUENAS_ENDPOINT`. Apply. | Error "Missing endpoint": "Set endpoint in provider config or TRUENAS_ENDPOINT env var." Plan/apply aborts before any network call. |
| 0.1.4 | Endpoint via env var | Omit `endpoint` in HCL; set `TRUENAS_ENDPOINT=wss://<host>/api/current`. Apply. | Connects using the env value. |
| 0.1.5 | HCL value takes precedence over env | Set both `endpoint` in HCL (valid host) and `TRUENAS_ENDPOINT` (different, invalid host). Apply. | Uses the HCL value (`envOrVal` only falls back to env when the HCL value is empty/null/unknown). |
| 0.1.6 | Unreachable endpoint | `endpoint = "wss://192.0.2.1/api/current"` (TEST-NET-1, non-routable). Apply. | Error "Connection failed": "Cannot connect to TrueNAS at <endpoint>: ..." after the dial times out (`HandshakeTimeout: 10s`). No hang. |

### 0.2 api_key auth

| # | Case | Steps | Expect |
|---|---|---|---|
| 0.2.1 | Valid api_key, no username (plain path) | `api_key = "<id>-<secret>"`, no `username`/`password`. Apply. | Connects via `auth.login_with_api_key` (no SCRAM probe attempted since `username` is empty — see `AuthAPIKeyAuto`). |
| 0.2.2 | api_key via env var | Omit `api_key` in HCL; set `TRUENAS_API_KEY`. Apply. | Authenticates using the env value. |
| 0.2.3 | Malformed api_key (no `-` separator) | `api_key = "notavalidkey"`. Apply. | On a SCRAM-capable server with username set: `splitAPIKey` error "invalid API key format (want <id>-<secret>)" surfaces as the auth failure. Without username (plain path): server rejects with an auth error ("authentication rejected: invalid API key"). Either way, apply fails cleanly, no panic. |
| 0.2.4 | Revoked/deleted api_key | Create an API key via `truenas_api_key`, note its value, delete the resource (revoking it on the server), then configure the provider with that stale key value. Apply. | Connection fails with an authentication-rejected error; no partial state changes. |

### 0.3 username+password auth

| # | Case | Steps | Expect |
|---|---|---|---|
| 0.3.1 | Valid username+password | `username`/`password` set, no `api_key`. Apply. | Connects via `auth.login`. |
| 0.3.2 | username without password | Set `username` only. Apply. | Error "Missing auth": "Provide api_key or username+password." (password-only is also insufficient — `hasPassword` requires both.) |
| 0.3.3 | password without username | Set `password` only. Apply. | Same "Missing auth" error. |
| 0.3.4 | Wrong password | Set valid `username`, wrong `password`. Apply. | Error text includes "authentication rejected: invalid username or password" (from `AuthPassword`), wrapped under "Connection failed". Verify the raw password is never echoed in the error/plan output (schema marks `password` Sensitive). |
| 0.3.5 | username+password via env vars | Omit both in HCL; set `TRUENAS_USERNAME`/`TRUENAS_PASSWORD`. Apply. | Authenticates using env values. |

### 0.4 username+api_key (SCRAM on 26.0+, plain fallback on 25.10)

| # | Case | Steps | Expect |
|---|---|---|---|
| 0.4.1 | SCRAM path on TrueNAS 26.0+ | Against a 26.0+ test box: set `api_key` + `username` (key owner). Apply, and independently capture the exchange — either the middleware auth/audit log or `auth.sessions` — while the provider authenticates. | `auth.mechanism_choices` is called pre-auth and returns a list including `SCRAM`; the client runs the full `auth.login_ex` SCRAM exchange (`CLIENT_FIRST_MESSAGE`/`CLIENT_FINAL_MESSAGE`) rather than `auth.login_with_api_key`. The resulting session in `auth.sessions` (or equivalent audit entry) shows the SCRAM mechanism, not `API_KEY_PLAIN`. Raw key never appears on the wire (verify via packet capture or server-side auth log if available — only nonces/proofs are sent). |
| 0.4.2 | Plain fallback on 25.10 | Against a 25.10.x test box: same config (`api_key` + `username`). Apply. | `auth.mechanism_choices` fails pre-auth (25.10's known `'NoneType' object has no attribute 'may_create_auth_token'` behavior per `SCRAM.md`); the client falls back to `auth.login_with_api_key`. Apply still succeeds. `auth.sessions`/audit log shows a plain API-key login, not SCRAM. |
| 0.4.3 | SCRAM wrong credentials rejected (mutual auth) | On a 26.0+ box, corrupt the key secret (same key ID, wrong secret) with `username` set. Apply. | SCRAM exchange fails at proof verification server-side; connection fails with an auth error. No fallback to plain login is attempted (no mechanism downgrade — verify via audit log that only one login attempt, SCRAM, was recorded). |
| 0.4.4 | SCRAM correct key, wrong username | On a 26.0+ box, use a valid key but a `username` that is not the key's actual owner. Apply. | SCRAM identity `<username>:<api_key_id>` doesn't match the key record server-side; exchange rejected; apply fails. |
| 0.4.5 | api_key set, username empty string | `api_key` set, `username = ""` (or omitted). Apply on a 26.0+ box. | No SCRAM probe attempted (`AuthAPIKeyAuto` requires non-empty username); falls straight to plain `auth.login_with_api_key`. Confirms plain path is available as an explicit operator choice even on a SCRAM-capable server. |

### 0.5 Conflict validation (config errors, no network call)

| # | Case | Steps | Expect |
|---|---|---|---|
| 0.5.1 | api_key + password both set | Set `api_key`, `username`, and `password` all together. `terraform plan`. | Error "Conflicting auth": "Provide api_key OR username+password, not both. (username alone may accompany api_key to enable SCRAM.)" No connection attempted. |
| 0.5.2 | Neither api_key nor username+password | Leave all four unset/empty, no env vars. `terraform plan`. | Error "Missing auth" as in 0.3.2. |
| 0.5.3 | insecure + ca_cert both set | `insecure = true` and `ca_cert = "/path/to/ca.pem"` together. `terraform plan`. | Error "Conflicting TLS config": "Set insecure or ca_cert, not both." No connection attempted. |

### 0.6 Insecure TLS flag

| # | Case | Steps | Expect |
|---|---|---|---|
| 0.6.1 | `insecure = true` against self-signed cert | Point at a TrueNAS box with a self-signed/expired certificate; `insecure = true`, valid auth. Apply. | Connects successfully; `BuildTLSConfig` sets `InsecureSkipVerify: true`. |
| 0.6.2 | `insecure` unset against self-signed cert | Same box, `insecure` omitted (default false), no `ca_cert`. Apply. | TLS handshake fails; "Connection failed" error surfaces the underlying x509 verification failure (e.g. "certificate signed by unknown authority"). |
| 0.6.3 | `insecure = false` explicit | Explicitly `insecure = false` against a box with a valid, CA-trusted certificate. Apply. | Connects normally (explicit false behaves identically to omitted). |

### 0.7 Custom CA path

| # | Case | Steps | Expect |
|---|---|---|---|
| 0.7.1 | Valid CA file, matching server cert | Generate/obtain the CA that signed the test box's TLS cert, save to a local file, set `ca_cert = "<path>"`. Apply. | `BuildTLSConfig` reads and parses the PEM, connects successfully via the pinned CA (`RootCAs` pool). |
| 0.7.2 | ca_cert path does not exist | `ca_cert = "/nonexistent/path/ca.pem"`. Apply. | Error "TLS config error": "reading CA certificate /nonexistent/path/ca.pem: ...". Fails before any dial attempt. |
| 0.7.3 | ca_cert file with invalid/empty PEM content | Point `ca_cert` at a file containing non-PEM garbage (or an empty file). Apply. | Error "TLS config error": "no valid certificates found in <path>". |
| 0.7.4 | ca_cert that does not match server's actual CA | Valid PEM CA cert, but not the one that issued the server's certificate. Apply. | TLS handshake fails (cert chain doesn't validate against the pinned CA); "Connection failed" surfaces the x509 error. |

### 0.8 Wrong credentials error text

| # | Case | Steps | Expect |
|---|---|---|---|
| 0.8.1 | Wrong api_key, plain path | Invalid but well-formed `api_key` (`999999-bogussecret`), no username. Apply. | Error chain includes "auth.login_with_api_key: ..." wrapping "authentication rejected: invalid API key" (from `AuthAPIKey`), surfaced under the provider's "Connection failed" diagnostic. |
| 0.8.2 | Wrong username/password | As in 0.3.4. | "authentication rejected: invalid username or password". |
| 0.8.3 | Error text excludes secrets | For 0.8.1 and 0.8.2, inspect the full `terraform apply` output/diagnostics text. | Neither the API key secret nor the password value appears anywhere in the error text (schema marks both `Sensitive`; auth error messages are generic per SCRAM.md's "No secret in error paths" claim). |

### 0.9 Rate-limit behavior (burst logins)

Source: `internal/client/auth.go` (`IsRateLimited`, `WithRetry`, `authRetryBackoff`).

| # | Case | Steps | Expect |
|---|---|---|---|
| 0.9.1 | Burst of provider connections trips rate limit | Script ~25+ sequential `terraform apply` runs (or parallel acceptance-test-style connections) against the same TrueNAS box within 60 seconds, each opening a fresh provider connection with valid credentials. | TrueNAS returns an EBUSY "Rate Limit Exceeded" error (API error code 16) on some attempts; the provider's `WithRetry` catches it via `IsRateLimited` (checks `APIError.Code == 16`, else a case-insensitive "rate limit" substring match) and retries with backoff `5s, 10s, 20s, 30s`. Applies that hit the limit succeed after a delay rather than failing outright, as long as the limit clears within the retry budget (~65s total). |
| 0.9.2 | Rate limit exhausts all retries | Sustain a high enough login rate that the limit does not clear within the ~65s retry budget (4 retries). | After the final retry attempt, the rate-limit error is returned as-is (still wrapped as "Connection failed"); confirm the error text still identifies it as a rate-limit condition (EBUSY / code 16), not a generic failure, so an operator can distinguish it from a real credential problem. |
| 0.9.3 | Non-rate-limit error is not retried | Trigger a genuine auth failure (wrong password) under normal (non-bursty) conditions. Apply. | Fails immediately on the first attempt — `WithRetry` returns as soon as `IsRateLimited(err)` is false; no multi-second delay before the error surfaces. |
| 0.9.4 | Rate limit during SCRAM exchange | On a 26.0+ box, burst enough `username`+`api_key` (SCRAM) provider connections to trip the limit. | Rate limiting applies at the `auth.login_ex`/session layer the same as plain auth; `WithRetry` wraps the whole `authFn` (including the SCRAM multi-step exchange), so retries re-run the entire exchange from scratch each attempt, not mid-exchange. |

---


---

## Storage and Shares

All configs below assume this provider block (adjust `endpoint` for your target box; `TEST-NET` addresses
per RFC 5737 are used wherever a demonstration IP/CIDR is needed):

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 0.1"
    }
  }
}

provider "truenas" {
  endpoint = "wss://<target-host>/api/current"
  api_key  = var.truenas_api_key
  insecure = true
}

variable "truenas_api_key" {
  description = "TrueNAS API key"
  sensitive   = true
}
```

All resource-under-test names use the `tf-acc-manual-` prefix so leftovers are easy to find and distinguish
from the Go acceptance-test suite's own `tf-acc-` fixtures. Every case operates only on objects it creates
itself — never rename, delete, or otherwise touch a pre-existing pool/dataset/share/schedule found on the
target box.

---

### 1. truenas_pool

**MT-STORAGE-001 — Manual pool creation procedure (documented, not automated)**

- **Resource:** truenas_pool
- **Type:** Manual-only (documented)
- **Environment:** primary test box only; requires at least 2 unused/blank disks with no existing partitions or ZFS labels. Never run against a box whose disks are in production use — this is destructive to whatever is on the target disks.
- **Preconditions:**
  - Identify blank disk device names via the TrueNAS UI (Storage > Disks) or `midclt call disk.query '[]'` and confirm each candidate disk's `pool` field is `null` (not already a pool member).
  - No `truenas_pool` resource block should ever be applied against disks that already host data — the schema forces replacement on `name`/`topology` changes, so any drift here destroys and recreates the pool.
- **Config:** (topology.data is a list of `{type, disks}`; `name` and `topology` both force replacement)
  ```hcl
  resource "truenas_pool" "test" {
    name = "tf-acc-manual-pool"
    topology = {
      data = [{
        type  = "MIRROR"
        disks = ["sdX", "sdY"]   # replace with real blank disk device names, e.g. from `midclt call disk.query '[]'`
      }]
    }
    autotrim = false
  }
  ```
- **Steps:**
  1. Replace `sdX`/`sdY` in the config with real blank disk device names obtained from the UI or API.
  2. `terraform init`
  3. `terraform plan` — review carefully; confirm only the two intended disks appear under `topology.data.0.disks`.
  4. `terraform apply`
  5. Verify in UI: Storage > pool list shows `tf-acc-manual-pool`, status `ONLINE`, topology shows a 2-disk mirror vdev.
  6. Verify via API: `midclt call pool.query '[["name", "=", "tf-acc-manual-pool"]]'` — confirm `status` is `ONLINE`, `healthy` is `true`, and `topology.data[0].type` is `MIRROR`.
  7. Update in place: change `autotrim = true`, `terraform apply`, confirm plan shows only an in-place update (not a replace).
  8. Verify: `midclt call pool.query '[["name","=","tf-acc-manual-pool"]]'` shows `autotrim.parsed` (or equivalent) as `true`.
  9. `terraform import truenas_pool.test tf-acc-manual-pool` into a fresh state and confirm no diff (`terraform plan` shows no changes).
  10. `terraform destroy` — this exports/destroys the pool. Confirm the destroy plan only targets `truenas_pool.test`.
  11. Verify in UI the pool no longer appears in Storage > pool list.
  12. Verify via API: `midclt call pool.query '[["name", "=", "tf-acc-manual-pool"]]'` returns an empty list.
- **Expected results:**
  - Step 4: apply succeeds, pool created with 2-disk mirror.
  - Step 6: pool healthy and online via API.
  - Step 7-8: autotrim toggle applies without replacing the pool (no disk reformat).
  - Step 9: import produces an empty diff.
  - Step 12: pool fully removed from the system; disks return to unused state.
- **Cleanup:** If apply/destroy fails partway, use the UI's Storage > pool > Export/Disconnect (with "Destroy data on this pool" checked) to remove `tf-acc-manual-pool`, or `midclt call pool.export '[<pool_id>, {"destroy": true}]'`. Query leftovers with `midclt call pool.query '[["name","~","^tf-acc-manual"]]'`.

**MT-STORAGE-002 — Pool datasource lookup**

- **Resource:** truenas_pool (datasource)
- **Type:** Datasource
- **Environment:** any; requires an existing pool (the box's normal primary pool, e.g. `tank`)
- **Preconditions:** Note the existing pool name to query; this test only reads, never modifies.
- **Config:**
  ```hcl
  data "truenas_pool" "t" {
    name = "tank"   # replace with the target box's actual pool name
  }

  output "pool_status" {
    value = data.truenas_pool.t.status
  }
  ```
- **Steps:**
  1. `terraform init`
  2. `terraform plan` — confirm the plan shows a data source read only, no resources created.
  3. `terraform apply`
  4. Check the `pool_status` output matches the pool's actual status.
  5. Cross-check via API: `midclt call pool.query '[["name","=","tank"]]'` and compare `status`, `healthy`, `guid`, `path`, `size`, `free`, `allocated` fields against the Terraform state (`terraform show`).
- **Expected results:** All datasource attributes (`topology`, `autotrim`, `guid`, `status`, `healthy`, `path`, `size`, `free`, `allocated`) match the live API values.
- **Cleanup:** None — read-only, no objects created. `terraform destroy` removes only the datasource from state (no API side effect).

---

### 2. truenas_dataset

**MT-STORAGE-003 — Dataset create, update, import, destroy**

- **Resource:** truenas_dataset
- **Type:** CRUD
- **Environment:** any; requires a pool named per `TRUENAS_TEST_POOL` convention (default `tank`)
- **Preconditions:** none beyond a usable pool; this creates a new child dataset under it.
- **Config:**
  ```hcl
  resource "truenas_dataset" "test" {
    name        = "tank/tf-acc-manual-ds"
    compression = "lz4"
    comments    = "initial comment"
  }
  ```
- **Steps:**
  1. `terraform init`
  2. `terraform apply`
  3. Verify in UI: Datasets tree shows `tf-acc-manual-ds` under `tank`, compression `LZ4`.
  4. Verify via API: `midclt call pool.dataset.query '[["id","=","tank/tf-acc-manual-ds"]]'` — confirm `compression.value` is `LZ4` and `comments.value` is `initial comment`.
  5. Update: change `comments = "updated comment"`, `terraform apply`. Confirm plan shows an in-place update (no replace).
  6. Verify: `midclt call pool.dataset.query '[["id","=","tank/tf-acc-manual-ds"]]'` shows `comments.value` = `updated comment`.
  7. `terraform import truenas_dataset.test tank/tf-acc-manual-ds` into a fresh state; `terraform plan` shows no diff.
  8. `terraform destroy`
  9. Verify in UI the dataset no longer appears under `tank`.
  10. Verify via API: `midclt call pool.dataset.query '[["id","=","tank/tf-acc-manual-ds"]]'` returns an empty list.
- **Expected results:** dataset created with correct name/compression/comments; comments update applies in place; import produces no diff; dataset and its mountpoint are fully removed after destroy.
- **Cleanup:** `midclt call pool.dataset.delete '["tank/tf-acc-manual-ds", {"recursive": true, "force": true}]'` if destroy fails. Find leftovers: `midclt call pool.dataset.query '[["id","~","tf-acc-manual"]]'`.

---

### 3. truenas_zvol

**MT-STORAGE-004 — Zvol create, resize, import, destroy**

- **Resource:** truenas_zvol
- **Type:** CRUD
- **Environment:** any; requires a usable pool
- **Preconditions:** none; creates its own zvol.
- **Config:**
  ```hcl
  resource "truenas_zvol" "test" {
    name        = "tank/tf-acc-manual-zv"
    volsize     = 67108864   # 64 MiB
    compression = "lz4"
    comments    = "initial comment"
  }
  ```
- **Steps:**
  1. `terraform init`
  2. `terraform apply`
  3. Verify in UI: Datasets tree shows `tf-acc-manual-zv` as type `VOLUME`, size 64 MiB.
  4. Verify via API: `midclt call pool.dataset.query '[["id","=","tank/tf-acc-manual-zv"]]'` — confirm `type` is `VOLUME` and `volsize.parsed` is `67108864`.
  5. Update: change `volsize = 134217728` (128 MiB) and `comments = "updated comment"`; `terraform apply`. Confirm plan is an in-place update — `volblocksize` is set at create time only (RequiresReplace on change, but not touched here).
  6. Verify: `midclt call pool.dataset.query '[["id","=","tank/tf-acc-manual-zv"]]'` shows `volsize.parsed` = `134217728`.
  7. `terraform import truenas_zvol.test tank/tf-acc-manual-zv`. Note: `sparse` is write-only and not returned by the API, so import will show it as unset/null — this is expected, not a bug; a `terraform plan` after import will show `sparse` unset but no other diff.
  8. `terraform destroy`
  9. Verify in UI the zvol no longer appears.
  10. Verify via API: `midclt call pool.dataset.query '[["id","=","tank/tf-acc-manual-zv"]]'` returns an empty list.
- **Expected results:** zvol created at 64 MiB; resize to 128 MiB applies in place without replacement; import succeeds with `sparse` left unset (documented gap, not a failure); zvol fully removed after destroy.
- **Cleanup:** `midclt call pool.dataset.delete '["tank/tf-acc-manual-zv", {"force": true}]'`. Find leftovers: `midclt call pool.dataset.query '[["id","~","tf-acc-manual"],["type","=","VOLUME"]]'`.

---

### 4. truenas_snapshot

**MT-STORAGE-005 — Snapshot create, import, destroy (no update path)**

- **Resource:** truenas_snapshot
- **Type:** CRUD (create/import/destroy only — every attribute forces replacement, there is no Update)
- **Environment:** any; requires a usable pool
- **Preconditions:** creates its own dataset fixture plus a snapshot on it.
- **Config:**
  ```hcl
  resource "truenas_dataset" "fixture" {
    name = "tank/tf-acc-manual-snap-ds"
  }

  resource "truenas_snapshot" "test" {
    dataset   = truenas_dataset.fixture.name
    name      = "tf-acc-manual-snap"
    recursive = false
  }
  ```
- **Steps:**
  1. `terraform init`
  2. `terraform apply`
  3. Verify in UI: Datasets > Snapshots shows `tank/tf-acc-manual-snap-ds@tf-acc-manual-snap`.
  4. Verify via API: `midclt call pool.snapshot.query '[["id","=","tank/tf-acc-manual-snap-ds@tf-acc-manual-snap"]]'` — confirm one result, `pool` = `tank`, `createtxg` set.
  5. `terraform import truenas_snapshot.test "tank/tf-acc-manual-snap-ds@tf-acc-manual-snap"` (note: the import ID is `dataset@snapname`). `recursive` is write-only (not returned by the API) — after import a `terraform plan` will show `recursive` unset, which is expected.
  6. `terraform destroy` — this destroys both the snapshot and the fixture dataset.
  7. Verify in UI: snapshot and dataset no longer listed.
  8. Verify via API: `midclt call pool.snapshot.query '[["id","=","tank/tf-acc-manual-snap-ds@tf-acc-manual-snap"]]'` and `midclt call pool.dataset.query '[["id","=","tank/tf-acc-manual-snap-ds"]]'` both return empty lists.
- **Expected results:** snapshot created with correct dataset/name; import by `dataset@snapname` succeeds (modulo the documented `recursive` gap); destroy removes both snapshot and dataset fixture.
- **Cleanup:** `midclt call pool.snapshot.delete '["tank/tf-acc-manual-snap-ds@tf-acc-manual-snap"]'` then `midclt call pool.dataset.delete '["tank/tf-acc-manual-snap-ds", {"recursive": true, "force": true}]'`. Find leftovers: `midclt call pool.snapshot.query '[["id","~","tf-acc-manual"]]'`.

---

### 5. truenas_periodic_snapshot_task

**MT-STORAGE-006 — Periodic snapshot task create, update, import, destroy**

- **Resource:** truenas_periodic_snapshot_task
- **Type:** CRUD
- **Environment:** any; requires a usable pool
- **Preconditions:** creates its own dataset fixture plus a periodic snapshot task on it.
- **Config:**
  ```hcl
  resource "truenas_dataset" "test" {
    name = "tank/tf-acc-manual-pst"
  }

  resource "truenas_periodic_snapshot_task" "test" {
    dataset        = truenas_dataset.test.name
    recursive      = false
    lifetime_value = 2
    lifetime_unit  = "WEEK"
    naming_schema  = "tf-acc-manual-%Y%m%d-%H%M"
    enabled        = true
    schedule = {
      minute = "0"
      hour   = "0"
      dom    = "*"
      month  = "*"
      dow    = "*"
    }
  }
  ```
- **Steps:**
  1. `terraform init`
  2. `terraform apply`
  3. Verify in UI: Data Protection > Periodic Snapshot Tasks shows a task for `tank/tf-acc-manual-pst`, schedule "At 00:00", lifetime 2 weeks.
  4. Verify via API: `midclt call pool.snapshottask.query '[["dataset","=","tank/tf-acc-manual-pst"]]'` — confirm `lifetime_value` 2, `lifetime_unit` `WEEK`, `naming_schema` matches, `schedule.minute` `0`.
  5. Update: change `schedule.minute = "15"` and `lifetime_value = 3`; `terraform apply`. Confirm in-place update.
  6. Verify: `midclt call pool.snapshottask.query '[["dataset","=","tank/tf-acc-manual-pst"]]'` shows `schedule.minute` `15`, `lifetime_value` `3`.
  7. `terraform import truenas_periodic_snapshot_task.test <numeric task id from step 4's query>`. Confirm `terraform plan` shows no diff.
  8. `terraform destroy` — removes both the task and the dataset fixture.
  9. Verify in UI the task and dataset no longer exist.
  10. Verify via API: `midclt call pool.snapshottask.query '[["dataset","=","tank/tf-acc-manual-pst"]]'` and `midclt call pool.dataset.query '[["id","=","tank/tf-acc-manual-pst"]]'` both empty.
- **Expected results:** task created and scheduled correctly; schedule/lifetime updates apply in place; import by numeric id matches state; both task and dataset removed on destroy.
- **Cleanup:** `midclt call pool.snapshottask.delete '[<id>]'` then delete the dataset as in MT-STORAGE-003. Find leftovers: `midclt call pool.snapshottask.query '[["dataset","~","tf-acc-manual"]]'`.

---

### 6. truenas_scrub_task

**MT-STORAGE-007 — Scrub schedule create, update, import, destroy**

- **Resource:** truenas_scrub_task
- **Type:** CRUD
- **Environment:** primary test box; **only runs if the target pool has no existing scrub schedule** — TrueNAS allows at most one `pool.scrub` schedule per pool and rejects a second `pool.scrub.create` with `[EINVAL] pool_scrub_create.pool: A scrub with this pool already exists`. Most pools ship with a default scrub schedule already present.
- **Preconditions:**
  1. Look up the target pool's numeric id: `midclt call pool.query '[["name","=","tank"]]'` (note the `id` field).
  2. Check for an existing schedule: `midclt call pool.scrub.query '[["pool","=",<pool_id>]]'`. **If this returns any result, skip this test case** — do not delete a pre-existing schedule to make room for this test.
- **Config:**
  ```hcl
  resource "truenas_scrub_task" "test" {
    pool        = 1   # replace with the pool's numeric id from the precondition query
    threshold   = 30
    description = "tf-acc-manual scrub task"
    enabled     = true
    schedule = {
      minute = "0"
      hour   = "2"
      dom    = "*"
      month  = "*"
      dow    = "7"
    }
  }
  ```
- **Steps:**
  1. `terraform init`
  2. `terraform apply`
  3. Verify in UI: Data Protection > Scrub Tasks shows a schedule on the target pool, threshold 30 days, Sunday 02:00.
  4. Verify via API: `midclt call pool.scrub.query '[["pool","=",<pool_id>]]'` — confirm `threshold` 30, `enabled` true, `schedule.dow` `7`.
  5. Update: change `threshold = 45`; `terraform apply`. Confirm in-place update.
  6. Verify: `midclt call pool.scrub.query '[["pool","=",<pool_id>]]'` shows `threshold` 45.
  7. `terraform import truenas_scrub_task.test <numeric scrub task id from step 4>`. Confirm no diff.
  8. `terraform destroy`
  9. Verify in UI no scrub schedule remains on the pool.
  10. Verify via API: `midclt call pool.scrub.query '[["pool","=",<pool_id>]]'` returns an empty list.
- **Expected results:** schedule created only when none pre-existed; threshold update applies in place; import by numeric id matches; schedule fully removed on destroy (pool's default scrub, if any, is not restored automatically — see Cleanup).
- **Cleanup:** `midclt call pool.scrub.delete '[<id>]'` if destroy fails. Since this resource self-skips when a schedule already exists, there is normally no "restore the original schedule" step needed — but if this case was run against a pool that later needs its original default scrub schedule back, recreate one via the UI (Data Protection > Scrub Tasks > Add) with default settings (threshold 35 days, weekly).

---

### 7. truenas_resilver_config

**MT-STORAGE-008 — Resilver config datasource read**

- **Resource:** truenas_resilver_config (datasource)
- **Type:** Datasource
- **Environment:** any
- **Preconditions:** none; read-only.
- **Config:**
  ```hcl
  data "truenas_resilver_config" "test" {}
  ```
- **Steps:**
  1. `terraform init`
  2. `terraform apply`
  3. Verify state (`terraform show`) has `id = "resilver_config"`, `begin` and `end` set (e.g. `"18:00"`/`"09:00"`).
  4. Cross-check via API: `midclt call pool.resilver.config` — compare `begin`, `end`, `enabled`, `weekday` to Terraform state.
- **Expected results:** all fields match live API values.
- **Cleanup:** none — read-only.

**MT-STORAGE-009 — Resilver config set-and-restore (Tier-2, DISRUPTIVE)**

- **Resource:** truenas_resilver_config
- **Type:** Singleton set-and-restore
- **Environment:** primary test box only. This mutates the box's live, system-wide resilver priority schedule. Treat as Tier-2/DISRUPTIVE — do not run against a shared or production system without coordinating with other users of the box.
- **Preconditions:**
  1. **Before any change**, capture the current values: `midclt call pool.resilver.config` and record `enabled` and `weekday`.
  2. Have those values ready to restore even if a later step fails.
- **Config (step A — toggled value):**
  ```hcl
  resource "truenas_resilver_config" "test" {
    enabled = true   # set to the OPPOSITE of the value recorded in preconditions
  }
  ```
  **Config (step B — restore):**
  ```hcl
  resource "truenas_resilver_config" "test" {
    enabled = false  # set back to the ORIGINAL value recorded in preconditions
  }
  ```
- **Steps:**
  1. Record original `enabled`/`weekday` via `midclt call pool.resilver.config` (preconditions).
  2. `terraform init`
  3. Apply step-A config with `enabled` set to the opposite of the original value.
  4. Verify via API: `midclt call pool.resilver.config` shows `enabled` matching the toggled value.
  5. Apply step-B config with `enabled` restored to the original value.
  6. Verify via API: `midclt call pool.resilver.config` shows `enabled` back to the original value, and `weekday` unchanged throughout (this test never touches `weekday`).
  7. `terraform import truenas_resilver_config.test resilver_config` (any ID string is accepted and normalized to the fixed id `resilver_config`) into a fresh state; confirm `id` reads back as `resilver_config` and no diff against the restored config.
  8. `terraform destroy` — this only removes the resource from Terraform state; it does **not** revert the live TrueNAS configuration (there is no delete-equivalent on `pool.resilver`).
- **Expected results:** `enabled` toggles and is restored to the original value; the box's schedule ends the test run byte-for-byte identical to how it started; import always succeeds regardless of the ID string supplied, always normalizing to `resilver_config`.
- **Cleanup:** If the restore step (5-6) is not reached due to a failure, immediately run: `midclt call pool.resilver.update '{"enabled": <original>, "weekday": [<original list>]}'` using the values recorded in preconditions.

---

### 8. truenas_system_dataset

**MT-STORAGE-010 — System dataset datasource read**

- **Resource:** truenas_system_dataset (datasource)
- **Type:** Datasource
- **Environment:** any
- **Preconditions:** none; read-only.
- **Config:**
  ```hcl
  data "truenas_system_dataset" "test" {}
  ```
- **Steps:**
  1. `terraform init`
  2. `terraform apply`
  3. Verify state has `id = "system_dataset"`, `pool`, `basename`, `path`, `uuid` all set.
  4. Cross-check via API: `midclt call systemdataset.config` — compare `pool`, `basename`, `path`, `uuid` to Terraform state.
- **Expected results:** all fields match live API values.
- **Cleanup:** none — read-only.

**MT-STORAGE-011 — System dataset migration (Manual-only, documented; NOT to be run casually)**

- **Resource:** truenas_system_dataset
- **Type:** Manual-only (documented) — the acceptance test suite's `TestAccSystemDataset_basic` is unconditionally `t.Skip`'d and never runs, even under `TRUENAS_DISRUPTIVE=1`, because changing `pool` triggers a real, long-running migration of core system state (logs, reporting, syslog, samba4 data) between pools.
- **Environment:** a disposable/non-production TrueNAS instance with at least two pools available. Never run against a shared or production box.
- **Preconditions:**
  1. Read the current config first: `midclt call systemdataset.config`, note the current `pool` value.
  2. Confirm a second pool exists to migrate to and back from.
  3. Understand this is disruptive: system logging, reporting, and (if applicable) the SMB `samba4` state briefly move between pools; do not run during any window where logs/reporting continuity matters.
- **Config (step A — migrate to alternate pool):**
  ```hcl
  resource "truenas_system_dataset" "test" {
    pool = "alt-pool"   # a second, real pool on the disposable box
  }
  ```
  **Config (step B — migrate back):**
  ```hcl
  resource "truenas_system_dataset" "test" {
    pool = "tank"   # the ORIGINAL pool recorded in preconditions
  }
  ```
- **Steps:**
  1. Record the original `pool` via `midclt call systemdataset.config` (preconditions).
  2. `terraform init`
  3. Apply step-A config (migrate to the alternate pool). This is a long-running job — allow it to complete; do not interrupt.
  4. Verify via API: `midclt call systemdataset.config` shows `pool` = the alternate pool, and `basename`/`path` updated to reflect the new location.
  5. Verify in UI: System Settings > Advanced shows the system dataset pool as the alternate pool.
  6. Apply step-B config (migrate back to the original pool). Again a long-running job.
  7. Verify via API: `midclt call systemdataset.config` shows `pool` back to the original value.
  8. `terraform import truenas_system_dataset.test system_dataset` (any ID string normalizes to the fixed id `system_dataset`) into a fresh state; confirm no diff.
  9. `terraform destroy` — removes only the Terraform state entry; the system dataset itself is never deleted from TrueNAS (there is no delete-equivalent on `systemdataset`).
- **Expected results:** both migrations complete without error; the box ends the test with the system dataset back on its original pool, `basename`/`path`/`uuid` matching pre-test values; import always normalizes to `system_dataset`.
- **Cleanup:** If the migrate-back step (6-7) is not reached, manually trigger `midclt call systemdataset.update '{"pool": "<original pool>"}'` (this returns a job id — poll `midclt call core.get_jobs '[["id","=",<job_id>]]'` until `state` is `SUCCESS`) to restore the original pool.

---

### 9. truenas_nfs_share

**MT-STORAGE-012 — NFS share create, update, import, destroy**

- **Resource:** truenas_nfs_share
- **Type:** CRUD
- **Environment:** any; requires a usable pool
- **Preconditions:** creates its own dataset fixture to share.
- **Config:**
  ```hcl
  resource "truenas_dataset" "fixture" {
    name = "tank/tf-acc-manual-nfs-ds"
  }

  resource "truenas_nfs_share" "test" {
    path     = truenas_dataset.fixture.mountpoint
    comment  = "initial comment"
    networks = ["192.0.2.0/24"]   # TEST-NET-1
  }
  ```
- **Steps:**
  1. `terraform init`
  2. `terraform apply`
  3. Verify in UI: Sharing > NFS shows a share on `/mnt/tank/tf-acc-manual-nfs-ds`, comment "initial comment", allowed network `192.0.2.0/24`.
  4. Verify via API: `midclt call sharing.nfs.query '[["path","=","/mnt/tank/tf-acc-manual-nfs-ds"]]'` — confirm `comment`, `enabled` true, `networks` contains `192.0.2.0/24`.
  5. Update: change `comment = "updated comment"` and `networks = ["192.0.2.0/24", "198.51.100.0/24"]` (TEST-NET-2); `terraform apply`. Confirm in-place update (path unchanged, no replace).
  6. Verify: `midclt call sharing.nfs.query '[["path","=","/mnt/tank/tf-acc-manual-nfs-ds"]]'` shows updated `comment` and 2 entries in `networks`.
  7. `terraform import truenas_nfs_share.test <numeric share id from step 4>`. Confirm no diff.
  8. `terraform destroy` — removes the share and the dataset fixture.
  9. Verify in UI: share no longer listed under Sharing > NFS.
  10. Verify via API: `midclt call sharing.nfs.query '[["path","=","/mnt/tank/tf-acc-manual-nfs-ds"]]'` and `midclt call pool.dataset.query '[["id","=","tank/tf-acc-manual-nfs-ds"]]'` both empty.
- **Expected results:** share created with correct path/comment/networks; comment and networks update in place; import by numeric id matches; share and dataset fully removed on destroy.
- **Cleanup:** `midclt call sharing.nfs.delete '[<id>]'` then delete the dataset fixture as in MT-STORAGE-003. Find leftovers: `midclt call sharing.nfs.query '[["path","~","tf-acc-manual"]]'`.

---

### 10. truenas_smb_share

**MT-STORAGE-013 — SMB share create, update, import, destroy**

- **Resource:** truenas_smb_share
- **Type:** CRUD
- **Environment:** any; requires a usable pool
- **Preconditions:** creates its own dataset fixture to share.
- **Config:**
  ```hcl
  resource "truenas_dataset" "test" {
    name = "tank/tf-acc-manual-ds-smb"
  }

  resource "truenas_smb_share" "test" {
    path       = truenas_dataset.test.mountpoint
    name       = "tf-acc-manual-smb"
    comment    = "initial comment"
    enabled    = true
    abe        = false
    hostsallow = ["127.0.0.1", "192.0.2.0/24"]
  }
  ```
- **Steps:**
  1. `terraform init`
  2. `terraform apply`
  3. Verify in UI: Sharing > SMB shows share `tf-acc-manual-smb` on `/mnt/tank/tf-acc-manual-ds-smb`, comment "initial comment", ABE off.
  4. Verify via API: `midclt call sharing.smb.query '[["name","=","tf-acc-manual-smb"]]'` — confirm `comment`, `access_based_share_enumeration` (or `abe`, depending on release — see schema note about the 26.0 field rename) is false, `hostsallow` has 2 entries, `vuid` is set.
  5. Update: change `comment = "updated comment"` and `abe = true`; `terraform apply`. Confirm in-place update.
  6. Verify: `midclt call sharing.smb.query '[["name","=","tf-acc-manual-smb"]]'` shows updated comment and ABE true.
  7. `terraform import truenas_smb_share.test <numeric share id from step 4>`. Confirm no diff.
  8. `terraform destroy` — removes the share and the dataset fixture.
  9. Verify in UI the share is gone from Sharing > SMB.
  10. Verify via API: `midclt call sharing.smb.query '[["name","=","tf-acc-manual-smb"]]'` and the dataset query both empty.
- **Expected results:** share created correctly; comment/abe update in place; import by numeric id matches; note that on TrueNAS 26.0+, `abe` maps to the wire field `access_based_share_enumeration` and legacy fields like `hostsallow`/`recyclebin`/`guestok` only apply when `purpose` is (or defaults to) `LEGACY_SHARE` — confirm the target release before assuming those fields round-trip. Share and dataset fully removed on destroy.
- **Cleanup:** `midclt call sharing.smb.delete '[<id>]'` then delete the dataset fixture. Find leftovers: `midclt call sharing.smb.query '[["name","~","tf-acc-manual"]]'`.

---

### 11. truenas_webshare

**MT-STORAGE-014 — Webshare share create, update, import, destroy (26.0+ only)**

- **Resource:** truenas_webshare
- **Type:** CRUD
- **Environment:** primary or cross-release box; **requires TrueNAS 26.0 or later** — the `sharing.webshare` namespace does not exist on 25.10 (confirmed via `core.get_methods` exposing 0 `webshare.*`/`sharing.webshare.*` methods on that release). Before running, confirm the release: `midclt call system.version` (or check UI About) and skip this case entirely on any pre-26.0 box.
- **Preconditions:** creates its own dataset fixture to share. Also note this resource has **no `comment` field** — `sharing.webshare.create`/`update` reject an extra `comment` key with a clean EINVAL, unlike NFS/SMB shares.
- **Config:**
  ```hcl
  resource "truenas_dataset" "test" {
    name = "tank/tf-acc-manual-ds-webshare"
  }

  resource "truenas_webshare" "test" {
    path    = truenas_dataset.test.mountpoint
    name    = "tf-acc-manual-webshare"
    enabled = true
  }
  ```
- **Steps:**
  1. Confirm target release is 26.0+ (`midclt call system.version`); if not, skip this case.
  2. `terraform init`
  3. `terraform apply`
  4. Verify in UI: Sharing > Webshare (or equivalent 26.0 UI location) shows share `tf-acc-manual-webshare` on `/mnt/tank/tf-acc-manual-ds-webshare`, enabled.
  5. Verify via API: `midclt call sharing.webshare.query '[["name","=","tf-acc-manual-webshare"]]'` — confirm `enabled` true, `is_home_base` false. Note: `dataset`/`relative_path` may read back `null` immediately after create (a documented quirk) — re-query after a moment or after the update step to see them settle to real values.
  6. Update: change `enabled = false` (this resource's cosmetic-equivalent update field, since there is no `comment`); `terraform apply`. Confirm in-place update.
  7. Verify: `midclt call sharing.webshare.query '[["name","=","tf-acc-manual-webshare"]]'` shows `enabled` false, and re-check that `dataset`/`relative_path` are now populated (non-null).
  8. `terraform import truenas_webshare.test <numeric share id from step 5>`. Confirm no diff.
  9. `terraform destroy` — removes the share and the dataset fixture.
  10. Verify in UI the share is gone.
  11. Verify via API: `midclt call sharing.webshare.query '[["name","=","tf-acc-manual-webshare"]]'` and the dataset query both empty.
- **Expected results:** on 26.0+, share created and enabled toggle applies in place; `dataset`/`relative_path` settle to non-null values after the update read; on a pre-26.0 box, applying this config produces a clean provider error (not a raw API error) — verify the error message names the 26.0 requirement rather than surfacing a raw JSON-RPC method-not-found error.
- **Cleanup:** `midclt call sharing.webshare.delete '[<id>]'` then delete the dataset fixture. Find leftovers: `midclt call sharing.webshare.query '[["name","~","tf-acc-manual"]]'`.

---

### 12. truenas_nfs_config

**MT-STORAGE-015 — NFS config datasource read**

- **Resource:** truenas_nfs_config (datasource)
- **Type:** Datasource
- **Environment:** any
- **Preconditions:** none; read-only.
- **Config:**
  ```hcl
  data "truenas_nfs_config" "test" {}
  ```
- **Steps:**
  1. `terraform init`
  2. `terraform apply`
  3. Verify state has `id = "nfs_config"`, `servers` and `managed_nfsd` set.
  4. Cross-check via API: `midclt call nfs.config` against the Terraform state's `servers`, `protocols`, `v4_domain`, `bindip`, etc.
- **Expected results:** all fields match live API values.
- **Cleanup:** none — read-only.

**MT-STORAGE-016 — NFS config set-and-restore (Tier-2, DISRUPTIVE)**

- **Resource:** truenas_nfs_config
- **Type:** Singleton set-and-restore
- **Environment:** primary test box only. NFS is a system-critical singleton service — existing exports and connected clients may depend on it. Only `v4_domain` is touched here (a low-risk, plain-string field); `protocols`, `bindip`, `servers`, and the mountd/statd/lockd ports are never touched by this case, since changing those on a live system can disrupt NFS clients.
- **Preconditions:**
  1. Record the current value: `midclt call nfs.config`, note `v4_domain`.
  2. Confirm no NFS clients are actively depending on domain-sensitive ID mapping during the test window.
- **Config (step A — test value):**
  ```hcl
  resource "truenas_nfs_config" "test" {
    v4_domain = "tf-acc-manual.example"
  }
  ```
  **Config (step B — restore):**
  ```hcl
  resource "truenas_nfs_config" "test" {
    v4_domain = ""   # or whatever original value was recorded in preconditions
  }
  ```
- **Steps:**
  1. Record original `v4_domain` (preconditions).
  2. `terraform init`
  3. Apply step-A config.
  4. Verify via API: `midclt call nfs.config` shows `v4_domain` = `tf-acc-manual.example`.
  5. Apply step-B config (restore original value).
  6. Verify via API: `midclt call nfs.config` shows `v4_domain` back to the original value.
  7. `terraform import truenas_nfs_config.test nfs_config` (any ID normalizes to `nfs_config`) into a fresh state; confirm no diff.
  8. `terraform destroy` — removes only Terraform state; live NFS config is left in place (delete never calls the API).
- **Expected results:** `v4_domain` round-trips; no other NFS setting is touched; import always normalizes to `nfs_config`.
- **Cleanup:** If restore (step 5-6) is not reached, run `midclt call nfs.update '{"v4_domain": "<original value>"}'` directly.

---

### 13. truenas_smb_config

**MT-STORAGE-017 — SMB config datasource read**

- **Resource:** truenas_smb_config (datasource)
- **Type:** Datasource
- **Environment:** any
- **Preconditions:** none; read-only.
- **Config:**
  ```hcl
  data "truenas_smb_config" "test" {}
  ```
- **Steps:**
  1. `terraform init`
  2. `terraform apply`
  3. Verify state has `id = "smb_config"`, `netbiosname` and `server_sid` set.
  4. Cross-check via API: `midclt call smb.config` against the Terraform state's `workgroup`, `netbiosname`, `minimum_protocol`, `bindip`, `admin_group`, etc.
- **Expected results:** all fields match live API values.
- **Cleanup:** none — read-only.

**MT-STORAGE-018 — SMB config set-and-restore (Tier-2, DISRUPTIVE)**

- **Resource:** truenas_smb_config
- **Type:** Singleton set-and-restore
- **Environment:** primary test box only. SMB is a system-critical singleton service — existing shares and domain membership may depend on it. Only `description` is touched here (an inert, cosmetic label); `workgroup`, `netbiosname`, `minimum_protocol`, `bindip`, and `admin_group` are never touched by this case, since changing those on a live system can disrupt SMB clients or domain membership.
- **Preconditions:**
  1. Record the current value: `midclt call smb.config`, note `description`.
- **Config (step A — test value):**
  ```hcl
  resource "truenas_smb_config" "test" {
    description = "tf-acc-manual-description"
  }
  ```
  **Config (step B — restore):**
  ```hcl
  resource "truenas_smb_config" "test" {
    description = ""   # or whatever original value was recorded in preconditions
  }
  ```
- **Steps:**
  1. Record original `description` (preconditions).
  2. `terraform init`
  3. Apply step-A config.
  4. Verify via API: `midclt call smb.config` shows `description` = `tf-acc-manual-description`.
  5. Apply step-B config (restore original value).
  6. Verify via API: `midclt call smb.config` shows `description` back to the original value.
  7. `terraform import truenas_smb_config.test smb_config` (any ID normalizes to `smb_config`) into a fresh state; confirm no diff.
  8. `terraform destroy` — removes only Terraform state; live SMB config is left in place.
- **Expected results:** `description` round-trips; no other SMB setting is touched; import always normalizes to `smb_config`.
- **Cleanup:** If restore (step 5-6) is not reached, run `midclt call smb.update '{"description": "<original value>"}'` directly.

---

### 14. truenas_webshare_config

**MT-STORAGE-019 — Webshare config datasource read (26.0+ only)**

- **Resource:** truenas_webshare_config (datasource)
- **Type:** Datasource
- **Environment:** primary or cross-release box; **requires TrueNAS 26.0 or later** — the `webshare` namespace does not exist on 25.10 (confirmed via `core.get_methods` exposing 0 `webshare.*` methods on that release).
- **Preconditions:** confirm target release via `midclt call system.version`; skip if pre-26.0.
- **Config:**
  ```hcl
  data "truenas_webshare_config" "test" {}
  ```
- **Steps:**
  1. Confirm target release is 26.0+; if not, applying this config should produce a clean provider error naming the version requirement (verify that error message rather than skipping silently).
  2. `terraform init`
  3. `terraform apply`
  4. Verify state has `id = "webshare_config"`, `passkey` set.
  5. Cross-check via API: `midclt call webshare.config` against the Terraform state's `bindip`, `search`, `passkey`, `groups`.
- **Expected results:** on 26.0+, all fields match live API values; on pre-26.0, a clean version-gate error is returned rather than a raw method-not-found error.
- **Cleanup:** none — read-only.

**MT-STORAGE-020 — Webshare config set-and-restore (26.0+ only, Tier-2, DISRUPTIVE)**

- **Resource:** truenas_webshare_config
- **Type:** Singleton set-and-restore
- **Environment:** primary test box only; **requires TrueNAS 26.0 or later** (same version gate as above). This is a 26.0+ probe resource — confirm the release before running. Only `search` is touched here (a cosmetic, low-risk toggle carrying no authentication or network-exposure risk, unlike `passkey`/`groups`/`bindip`); those other fields are never touched by this case.
- **Preconditions:**
  1. Confirm target release is 26.0+.
  2. Record the current value: `midclt call webshare.config`, note `search`. If this read fails on the target box, do not proceed — treat as an environment issue, not a test failure.
- **Config (step A — toggled value):**
  ```hcl
  resource "truenas_webshare_config" "test" {
    search = true   # set to the OPPOSITE of the value recorded in preconditions
  }
  ```
  **Config (step B — restore):**
  ```hcl
  resource "truenas_webshare_config" "test" {
    search = false  # set back to the ORIGINAL value recorded in preconditions
  }
  ```
- **Steps:**
  1. Record original `search` (preconditions).
  2. `terraform init`
  3. Apply step-A config with `search` set to the opposite of the original value.
  4. Verify via API: `midclt call webshare.config` shows `search` matching the toggled value, and `bindip`/`passkey`/`groups` unchanged from before the apply (this call is a genuine partial update on 26.0 — confirmed live).
  5. Apply step-B config (restore original value).
  6. Verify via API: `midclt call webshare.config` shows `search` back to the original value.
  7. `terraform import truenas_webshare_config.test webshare_config` (any ID normalizes to `webshare_config`) into a fresh state; confirm no diff.
  8. `terraform destroy` — removes only Terraform state; live Webshare config is left in place.
- **Expected results:** `search` toggles and restores; `bindip`/`passkey`/`groups` remain untouched throughout; import always normalizes to `webshare_config`; on pre-26.0, this entire case is skipped (do not attempt on older releases).
- **Cleanup:** If restore (step 5-6) is not reached, run `midclt call webshare.update '{"search": <original boolean>}'` directly.

---

## Block Storage (iSCSI and NVMe-oF)

> **Safety note for every case in this section.** On a TrueNAS box that is serving production storage, any pre-existing live iSCSI portal/target/extent (often `id=1`) and any pre-existing live NVMe-oF subsystem/port/namespace/port_subsys (often `id=1`) are attached to real initiators/hosts and MUST NOT be modified, disabled, or deleted by any test in this section. All test fixtures below use `tf-acc-manual-` prefixed names, bind listeners to the box's own IP address (never `0.0.0.0` — TrueNAS 26.0+ rejects a second portal bound to `0.0.0.0` because it collides with the live portal), and use NVMe-oF test ports on service IDs `14420`/`14421` (distinct from the live port's `4420`) created with `enabled = false` so they never open a live listener. CHAP auth test cases use `tag = 998` or `999`, distinct from any live tag. Before starting, record the box's live iSCSI/NVMe-oF configuration (`midclt call iscsi.global.config`, `midclt call nvmet.global.config`, `midclt call iscsi.target.query`, `midclt call nvmet.subsys.query`) so any accidental drift is detectable.
>
> Environment variables referenced below (`TRUENAS_ENDPOINT`, `TRUENAS_API_KEY` or `TRUENAS_USERNAME`/`TRUENAS_PASSWORD`, `TRUENAS_TEST_POOL`) configure the provider block; substitute your own test box values. All HCL below assumes a `terraform` provider block for `truenas/truenas` is already configured in the working directory (not repeated in every case for brevity) and that a ZFS pool (default `tank`) exists for zvol fixtures.

### 1. truenas_iscsi_global

**MT-BLOCK-001 — Read the live iSCSI global configuration via datasource**

**Resource:** `truenas_iscsi_global` (datasource)
**Type:** Singleton (read-only)
**Environment:** Any TrueNAS box, including a production-serving one — this case makes no writes.
**Preconditions:** iSCSI service configured (default install has a global config even if the service isn't running).

**Config:**
```hcl
data "truenas_iscsi_global" "test" {}

output "iscsi_global" {
  value = data.truenas_iscsi_global.test
}
```

**Steps:**
1. `terraform init`
2. `terraform apply -auto-approve`
3. Inspect the output: `terraform output iscsi_global`
4. Cross-check against `midclt call iscsi.global.config`

**Expected results:** `id = "iscsi_global"`, `basename` and `listen_port` are set (non-empty/non-zero), `alua`/`iser` are booleans, `isns_servers` is a list (possibly empty), `pool_avail_threshold` reflects the box's current alert threshold (0 if unset). All values match the `midclt` output.

**Cleanup:** `terraform destroy -auto-approve` (removes only the datasource/output from state; no API calls are made).

---

**MT-BLOCK-002 — Singleton set-and-restore: pool_avail_threshold (DISRUPTIVE)**

**Resource:** `truenas_iscsi_global` (resource)
**Type:** Singleton — set-and-restore, DISRUPTIVE
**Environment:** A box where you are authorized to briefly change the iSCSI pool-capacity alert threshold. Per the acceptance-test source (`internal/resources/iscsi_global/acceptance_test.go`), `pool_avail_threshold` is advisory-only: it only controls the pool-capacity ALERT threshold used to warn about low free space backing iSCSI extents, and does not affect existing targets, extents, or active iSCSI sessions — so this is safe even on a production-serving box, but still record and restore the original value. Do NOT extend this test to `basename`, `listen_port`, `isns_servers`, or `alua` — those can disrupt active iSCSI sessions or discovery and are excluded from this manual test by design (mirroring the automated test's safety scope).
**Preconditions:** Record the box's current threshold: `midclt call iscsi.global.config | jq .pool_avail_threshold` (note whether it is `null`/unset or a number).

**Config (step A — set to a test value):**
```hcl
resource "truenas_iscsi_global" "test" {
  pool_avail_threshold = 80
}
```

**Config (step B — restore to the original value; if the box originally had no threshold set, use 0, which the provider maps to a JSON null on the wire):**
```hcl
resource "truenas_iscsi_global" "test" {
  pool_avail_threshold = 0
}
```

**Steps:**
1. `terraform init`
2. Apply Config A: `terraform apply -auto-approve`
3. Verify: `terraform state show truenas_iscsi_global.test` shows `pool_avail_threshold = 80`; `midclt call iscsi.global.config | jq .pool_avail_threshold` returns `80`. In the UI: Shares > Block Shares (iSCSI) > wrench icon > Global Configuration.
4. Apply Config B (restore): `terraform apply -auto-approve`
5. Verify: `pool_avail_threshold = 0` in state, and `midclt call iscsi.global.config | jq .pool_avail_threshold` returns `null` (or the original numeric value if the box had one — repeat with that specific number instead of 0 in Config B).
6. `terraform import truenas_iscsi_global.test iscsi_global`
7. Verify the imported state matches applied state (`pool_avail_threshold`, `basename`, `listen_port`, `alua`, `iser`, `isns_servers`).
8. `terraform destroy -auto-approve`

**Expected results:** Step 3 shows the threshold changed to 80 with no other iSCSI global fields altered and no interruption to any active iSCSI session. Step 5 shows the threshold restored to the pre-test value. Import in step 6 succeeds using the fixed ID string `iscsi_global` (this singleton has no numeric ID). Destroy in step 8 emits a warning ("iSCSI global configuration left in place") and makes **no** API call — `midclt call iscsi.global.config` after destroy still shows the restored threshold from step 5, confirming Delete never reset the box's config.

**Cleanup:** Confirm via `midclt call iscsi.global.config` that `pool_avail_threshold` matches the box's pre-test value recorded in Preconditions.

---

### 2. truenas_iscsi_portal

**MT-BLOCK-003 — Create, update, import, and destroy an iSCSI portal**

**Resource:** `truenas_iscsi_portal`
**Type:** CRUD
**Environment:** Any TrueNAS box. Test portal binds the box's own IP address — never `0.0.0.0`, which collides with the live portal (TrueNAS 26.0+ allows only one portal per IP, and the live portal on a production box typically already binds `0.0.0.0` or the box IP).
**Preconditions:** Know the box's iSCSI-facing IP address (the address you'll connect to it by, e.g. `10.0.0.10`).

**Config (initial):**
```hcl
resource "truenas_iscsi_portal" "test" {
  comment = "tf-acc-manual-portal"
  listen = [
    {
      ip = "10.0.0.10"   # substitute the box's own IP
    }
  ]
}
```

**Config (update — comment change only):**
```hcl
resource "truenas_iscsi_portal" "test" {
  comment = "tf-acc-manual-portal-updated"
  listen = [
    {
      ip = "10.0.0.10"
    }
  ]
}
```

**Steps:**
1. `terraform init`
2. `terraform plan` then `terraform apply -auto-approve` with the initial config
3. Verify: `terraform state show truenas_iscsi_portal.test` — `id` is a positive number, `tag` is set, `listen.0.ip` matches the box IP, `listen.0.port` is populated (read-only; per-listen port is not settable on TrueNAS 26.0+, the global `iscsi_global.listen_port` governs it). Cross-check: `midclt call iscsi.portal.query '[["comment","=","tf-acc-manual-portal"]]'`. UI: Shares > Block Shares (iSCSI) > Portals.
4. Apply the update config: `terraform apply -auto-approve`
5. Verify `comment` is now `tf-acc-manual-portal-updated` and `id`/`tag` are unchanged (in-place update, no replacement).
6. `terraform import truenas_iscsi_portal.import_test <id>` (use the numeric `id` from step 3/5)
7. Verify the imported resource's attributes match the applied state (`ImportStateVerify` in the automated suite has no ignored fields for this resource).
8. `terraform destroy -auto-approve`
9. Post-destroy: `midclt call iscsi.portal.query '[["comment","=","tf-acc-manual-portal-updated"]]'` returns an empty list.

**Expected results:** Create succeeds, portal appears in the UI and via API with correct IP. Update changes only the comment in place. Import reproduces identical state. Destroy removes the portal from TrueNAS.

**Cleanup:** None beyond step 8/9 (fully destroyed).

---

### 3. truenas_iscsi_initiator

**MT-BLOCK-004 — Create, update, import, and destroy an iSCSI initiator group**

**Resource:** `truenas_iscsi_initiator`
**Type:** CRUD
**Environment:** Any TrueNAS box.
**Preconditions:** None (no external fixtures required).

**Config (initial — empty initiators list, allows all initiators):**
```hcl
resource "truenas_iscsi_initiator" "test" {
  comment    = "tf-acc-manual-initiator"
  initiators = []
}
```

**Config (update — add a comment suffix and one IQN):**
```hcl
resource "truenas_iscsi_initiator" "test" {
  comment    = "tf-acc-manual-initiator-updated"
  initiators = ["iqn.2023-01.com.example:tf-acc-manual-host"]
}
```

**Steps:**
1. `terraform init`
2. Apply initial config: `terraform apply -auto-approve`
3. Verify: `id` set, `comment = "tf-acc-manual-initiator"`, `initiators.# = 0`. Cross-check: `midclt call iscsi.initiator.query '[["comment","=","tf-acc-manual-initiator"]]'`. UI: Shares > Block Shares (iSCSI) > Initiators Groups.
4. Apply update config: `terraform apply -auto-approve`
5. Verify `comment` updated and `initiators.0 = "iqn.2023-01.com.example:tf-acc-manual-host"`.
6. `terraform import truenas_iscsi_initiator.import_test <id>`
7. Verify imported state matches applied state.
8. `terraform destroy -auto-approve`
9. Post-destroy: query by the numeric id captured in step 3 (`midclt call iscsi.initiator.query '[["id","=",<id>]]'`) returns empty. (There is no unique name/comment guaranteed by the API, so the id — not the comment — is the reliable destroy-check key, matching the automated test's approach.)

**Expected results:** Create/update/import/destroy all behave as a standard CRUD resource; an empty `initiators` list means "allow all initiators" per TrueNAS semantics.

**Cleanup:** None beyond step 8/9.

---

### 4. truenas_iscsi_auth

**MT-BLOCK-005 — Create, update, import, and destroy an iSCSI CHAP auth entry**

**Resource:** `truenas_iscsi_auth`
**Type:** CRUD
**Environment:** Any TrueNAS box. Uses CHAP group `tag = 998`, distinct from any live tag and from the end-to-end test's tag (999) — do not reuse a tag already referenced by a live target group.
**Preconditions:** Confirm tag 998 is not already in use: `midclt call iscsi.auth.query '[["tag","=",998]]'` should return empty.

**Config (initial):**
```hcl
resource "truenas_iscsi_auth" "test" {
  tag    = 998
  user   = "tfaccmanualchapuser"
  secret = "tfacc-secret-1ch"   # 12-16 chars required
}
```

**Config (update — new secret, explicit discovery_auth):**
```hcl
resource "truenas_iscsi_auth" "test" {
  tag             = 998
  user            = "tfaccmanualchapuser"
  secret          = "tfacc-secret-2ch"
  discovery_auth  = "NONE"
}
```

**Steps:**
1. `terraform init`
2. Apply initial config: `terraform apply -auto-approve`
3. Verify: `user = "tfaccmanualchapuser"`, `tag = 998`, `id` set. `secret` is write-only and is NOT present in state (`terraform state show truenas_iscsi_auth.test` will show no `secret` attribute value — confirm it never appears). Cross-check user/tag only: `midclt call iscsi.auth.query '[["tag","=",998]]'` (the API also never exposes the secret in plaintext beyond this internal check).
4. Apply update config: `terraform apply -auto-approve`
5. Verify `discovery_auth = "NONE"`.
6. `terraform import truenas_iscsi_auth.import_test <id>`, with `-var` or inline `ImportStateVerifyIgnore` equivalent: since `secret` and `peersecret` are write-only and never read back, expect the imported resource to have those fields null/absent — this is expected, not a bug. Compare all other fields (`tag`, `user`, `peeruser`, `discovery_auth`) against the pre-import state.
7. `terraform destroy -auto-approve`
8. Post-destroy: `midclt call iscsi.auth.query '[["tag","=",998]]'` returns empty.

**Expected results:** `secret`/`peersecret` never appear in `terraform show`, `terraform state show`, or plan output in plaintext beyond the initial apply (Terraform's sensitive-value redaction applies, and the write-only mechanism means they are never persisted to state at all — Terraform >= 1.11 required for `secret`/`peersecret`). All other fields behave as normal CRUD attributes.

**Cleanup:** None beyond step 7/8.

---

### 5. truenas_iscsi_extent

**MT-BLOCK-006 — Create a DISK-backed extent over a zvol, update, import, destroy**

**Resource:** `truenas_iscsi_extent`
**Type:** CRUD
**Environment:** Any TrueNAS box with a usable pool.
**Preconditions:** A pool exists (default `tank`); this case creates its own zvol fixture, so no pre-existing zvol is required. `name` is `RequiresReplace` — changing it destroys and recreates the extent.

**Config (initial):**
```hcl
resource "truenas_zvol" "fixture" {
  name    = "tank/tf-acc-manual-extent-vol"
  volsize = 67108864   # 64 MiB
}

resource "truenas_iscsi_extent" "test" {
  name    = "tf-acc-manual-extent"
  type    = "DISK"
  disk    = "zvol/${truenas_zvol.fixture.name}"
  comment = "initial comment"
  enabled = true
}
```

**Config (update — comment only):**
```hcl
resource "truenas_zvol" "fixture" {
  name    = "tank/tf-acc-manual-extent-vol"
  volsize = 67108864
}

resource "truenas_iscsi_extent" "test" {
  name    = "tf-acc-manual-extent"
  type    = "DISK"
  disk    = "zvol/${truenas_zvol.fixture.name}"
  comment = "updated comment"
  enabled = true
}
```

**Steps:**
1. `terraform init`
2. Apply initial config: `terraform apply -auto-approve`
3. Verify: `name`, `type = "DISK"`, `disk = "zvol/tank/tf-acc-manual-extent-vol"`, `comment = "initial comment"`, `enabled = true`, `id`/`naa`/`serial` are set (server-generated). Cross-check: `midclt call iscsi.extent.query '[["name","=","tf-acc-manual-extent"]]'`. UI: Shares > Block Shares (iSCSI) > Extents.
4. Apply update config: `terraform apply -auto-approve`
5. Verify `comment = "updated comment"`; `naa`/`serial`/`id` unchanged (in-place update).
6. `terraform import truenas_iscsi_extent.import_test <id>`
7. Verify imported state matches applied state (no ignored fields).
8. `terraform destroy -auto-approve` (destroys the extent, then the zvol fixture)
9. Post-destroy: `midclt call iscsi.extent.query '[["name","=","tf-acc-manual-extent"]]'` returns empty; `midclt call pool.dataset.query '[["id","=","tank/tf-acc-manual-extent-vol"]]'` returns empty.

**Expected results:** Extent created against the zvol, `naa`/`serial`/`product_id`/`vendor`/`locked` are all populated read-only fields. Update changes comment only. Destroy removes both the extent and (via Terraform dependency ordering) the zvol.

**Cleanup:** None beyond step 8/9. If `terraform destroy` is interrupted, manually remove the extent then the zvol via the UI or `midclt call iscsi.extent.delete <id>` / `midclt call pool.dataset.delete tank/tf-acc-manual-extent-vol`.

---

### 6. truenas_iscsi_target

**MT-BLOCK-007 — Create a target referencing an own portal fixture, update, import, destroy**

**Resource:** `truenas_iscsi_target`
**Type:** CRUD
**Environment:** Any TrueNAS box. Uses its own portal fixture bound to the box IP (not `0.0.0.0`) — distinct from any pre-existing live portal/target on the box.
**Preconditions:** Know the box's iSCSI IP address.

**Config (initial):**
```hcl
resource "truenas_iscsi_portal" "fixture" {
  comment = "tf-acc-manual-target-portal"
  listen = [
    {
      ip = "10.0.0.10"   # substitute the box's own IP
    }
  ]
}

resource "truenas_iscsi_target" "test" {
  name  = "tf-acc-manual-target"
  alias = "initial-alias"
  mode  = "ISCSI"
  groups = [
    {
      portal     = truenas_iscsi_portal.fixture.id
      authmethod = "NONE"
    }
  ]
}
```

**Config (update — alias only):**
```hcl
resource "truenas_iscsi_portal" "fixture" {
  comment = "tf-acc-manual-target-portal"
  listen = [
    {
      ip = "10.0.0.10"
    }
  ]
}

resource "truenas_iscsi_target" "test" {
  name  = "tf-acc-manual-target"
  alias = "updated-alias"
  mode  = "ISCSI"
  groups = [
    {
      portal     = truenas_iscsi_portal.fixture.id
      authmethod = "NONE"
    }
  ]
}
```

**Steps:**
1. `terraform init`
2. Apply initial config: `terraform apply -auto-approve`
3. Verify: `name`, `mode = "ISCSI"`, `alias = "initial-alias"`, `groups.0.authmethod = "NONE"`, `groups.0.portal` equals the portal fixture's `id`, `rel_tgt_id` is set. Cross-check: `midclt call iscsi.target.query '[["name","=","tf-acc-manual-target"]]'`. UI: Shares > Block Shares (iSCSI) > Targets.
4. Apply update config: `terraform apply -auto-approve`
5. Verify `alias = "updated-alias"`; `id`/`rel_tgt_id` unchanged.
6. `terraform import truenas_iscsi_target.import_test <id>`
7. Verify imported state matches.
8. `terraform destroy -auto-approve` (destroys target, then portal fixture)
9. Post-destroy: `midclt call iscsi.target.query '[["name","=","tf-acc-manual-target"]]'` returns empty.

**Expected results:** Target created with a single portal group, `authmethod = NONE`, no initiator/auth restriction. Update changes alias in place. Destroy removes target and portal cleanly.

**Cleanup:** None beyond step 8/9.

---

### 7. truenas_iscsi_targetextent

**MT-BLOCK-008 — Associate a target and extent as a LUN mapping, import, destroy (standalone)**

**Resource:** `truenas_iscsi_targetextent`
**Type:** CRUD (both `target` and `extent` are `RequiresReplace` — there is no true in-place update path other than `lunid`, which is Optional+Computed and generally left to auto-assign)
**Environment:** Any TrueNAS box. This standalone case builds a minimal target+extent pair distinct from the full integration test in MT-BLOCK-009; use MT-BLOCK-009 to exercise the complete wiring chain.
**Preconditions:** A pool exists; this case creates its own zvol, extent, portal, and target fixtures.

**Config:**
```hcl
resource "truenas_zvol" "fixture" {
  name    = "tank/tf-acc-manual-te-vol"
  volsize = 67108864
}

resource "truenas_iscsi_extent" "fixture" {
  name = "tf-acc-manual-te-extent"
  type = "DISK"
  disk = "zvol/${truenas_zvol.fixture.name}"
}

resource "truenas_iscsi_portal" "fixture" {
  comment = "tf-acc-manual-te-portal"
  listen = [
    {
      ip = "10.0.0.10"   # substitute the box's own IP
    }
  ]
}

resource "truenas_iscsi_target" "fixture" {
  name = "tf-acc-manual-te-target"
  groups = [
    {
      portal     = truenas_iscsi_portal.fixture.id
      authmethod = "NONE"
    }
  ]
}

resource "truenas_iscsi_targetextent" "test" {
  target = truenas_iscsi_target.fixture.id
  extent = truenas_iscsi_extent.fixture.id
  lunid  = 0
}
```

**Steps:**
1. `terraform init`
2. `terraform apply -auto-approve`
3. Verify: `id` set, `target`/`extent` match the fixture ids, `lunid = 0`. Cross-check: `midclt call iscsi.targetextent.query '[["target","=",<target_id>],["extent","=",<extent_id>]]'`. UI: Shares > Block Shares (iSCSI) > Associated Targets.
4. `terraform import truenas_iscsi_targetextent.import_test <id>`
5. Verify imported state matches.
6. `terraform destroy -auto-approve`
7. Post-destroy: re-run the query from step 3 using the target/extent ids recorded before destroy (they are no longer resolvable by name after destroy) — expect an empty result.

**Expected results:** LUN association created at lunid 0; import/destroy behave as standard CRUD. Changing `target` or `extent` in a future apply forces replacement (verify by editing either field and confirming `terraform plan` shows a destroy/create, not an update, if you choose to extend this case).

**Cleanup:** None beyond step 6/7.

---

**MT-BLOCK-009 — Full iSCSI integration: portal → initiator → CHAP auth → zvol → extent → target → targetextent (end-to-end LUN wiring)**

**Resource:** `truenas_iscsi_portal`, `truenas_iscsi_initiator`, `truenas_iscsi_auth`, `truenas_zvol`, `truenas_iscsi_extent`, `truenas_iscsi_target`, `truenas_iscsi_targetextent`
**Type:** Integration (multi-resource)
**Environment:** Any TrueNAS box, including production-serving ones — this test is entirely self-contained and never references any pre-existing live iSCSI configuration on the box (live portal/target/extent/targetextent, if present). The test portal binds the box's own IP (not `0.0.0.0`), and the CHAP auth group uses `tag = 999`, distinct from any live tag. This reproduces `TestAccISCSIEndToEnd` in `internal/resources/iscsi_targetextent/acceptance_test.go`.
**Preconditions:** A pool exists (default `tank`); confirm tag 999 is free: `midclt call iscsi.auth.query '[["tag","=",999]]'` returns empty.

**Config (initial):**
```hcl
resource "truenas_iscsi_portal" "test" {
  comment = "tf-acc-manual-e2e-portal"
  listen = [
    {
      ip = "10.0.0.10"   # substitute the box's own IP
    }
  ]
}

resource "truenas_iscsi_initiator" "test" {
  comment    = "tf-acc-manual-e2e-initiator"
  initiators = []
}

resource "truenas_iscsi_auth" "test" {
  tag    = 999
  user   = "tfaccmanuale2eauthuser"
  secret = "tfacc-secret12"   # 14 chars
}

resource "truenas_zvol" "test" {
  name    = "tank/tf-acc-manual-e2e-zvol"
  volsize = 67108864
}

resource "truenas_iscsi_extent" "test" {
  name    = "tf-acc-manual-e2e-extent"
  type    = "DISK"
  disk    = "zvol/${truenas_zvol.test.name}"
  comment = "initial extent comment"
  enabled = true
}

resource "truenas_iscsi_target" "test" {
  name  = "tf-acc-manual-e2e-target"
  alias = "initial-alias"
  groups = [
    {
      portal     = truenas_iscsi_portal.test.id
      initiator  = truenas_iscsi_initiator.test.id
      auth       = truenas_iscsi_auth.test.tag
      authmethod = "CHAP"
    }
  ]
}

resource "truenas_iscsi_targetextent" "test" {
  target = truenas_iscsi_target.test.id
  extent = truenas_iscsi_extent.test.id
  lunid  = 0
}
```

**Config (update — change extent comment and target alias only; rest of the chain untouched):**
```hcl
# Same as above, except:
#   truenas_iscsi_extent.test.comment = "updated extent comment"
#   truenas_iscsi_target.test.alias   = "updated-alias"
```

**Steps:**
1. `terraform init`
2. `terraform plan` — confirm the plan creates all 7 resources with correct dependency ordering (portal/initiator/auth/zvol before extent/target, target+extent before targetextent).
3. `terraform apply -auto-approve`
4. Verify each resource in turn:
   - Portal: `comment`, `tag` set, `listen.0.ip` = box IP. `midclt call iscsi.portal.query '[["comment","=","tf-acc-manual-e2e-portal"]]'`
   - Initiator: `comment` set, empty `initiators` list. `midclt call iscsi.initiator.query '[["comment","=","tf-acc-manual-e2e-initiator"]]'`
   - Auth: `tag = 999`, `user = "tfaccmanuale2eauthuser"`. `midclt call iscsi.auth.query '[["tag","=",999]]'`
   - Zvol: `name = "tank/tf-acc-manual-e2e-zvol"`. `midclt call pool.dataset.query '[["id","=","tank/tf-acc-manual-e2e-zvol"]]'`
   - Extent: `type = "DISK"`, `disk = "zvol/tank/tf-acc-manual-e2e-zvol"`, `comment = "initial extent comment"`, `naa`/`serial` set. `midclt call iscsi.extent.query '[["name","=","tf-acc-manual-e2e-extent"]]'`
   - Target: `alias = "initial-alias"`, `groups.0.authmethod = "CHAP"`, `groups.0.portal` == portal id, `groups.0.initiator` == initiator id, `groups.0.auth` == auth tag. `midclt call iscsi.target.query '[["name","=","tf-acc-manual-e2e-target"]]'`
   - TargetExtent: `target`/`extent` match ids, `lunid = 0`. `midclt call iscsi.targetextent.query '[["target","=",<target_id>],["extent","=",<extent_id>]]'`
5. UI walkthrough: Shares > Block Shares (iSCSI) — confirm the portal, initiator group, auth (Advanced Settings), target, extent, and associated target/LUN mapping are all visible and correctly cross-referenced.
6. (Optional, requires an iSCSI initiator client) Discover the target via `iscsiadm -m discovery -t sendtargets -p <box-ip>` and confirm the target IQN appears with CHAP required; do not log in/mount if this is a shared test box.
7. Apply the update config: `terraform apply -auto-approve`
8. Verify only `truenas_iscsi_extent.test.comment` and `truenas_iscsi_target.test.alias` changed; all other resources (portal, initiator, auth, zvol, targetextent) are untouched (`terraform plan` after step 7 shows zero diff, and `id`s for all resources are unchanged from step 4).
9. `terraform import truenas_iscsi_targetextent.test <targetextent_id>` — verify import is clean.
10. `terraform import truenas_iscsi_target.test <target_id>` — verify import is clean.
11. `terraform import truenas_iscsi_extent.test <extent_id>` — verify import is clean.
12. `terraform destroy -auto-approve`
13. Post-destroy verification (query each by its distinguishing attribute, or by numeric id captured from pre-destroy state for the targetextent association since target/extent no longer exist to look up by name):
    - `midclt call iscsi.target.query '[["name","=","tf-acc-manual-e2e-target"]]'` → empty
    - `midclt call iscsi.extent.query '[["name","=","tf-acc-manual-e2e-extent"]]'` → empty
    - `midclt call iscsi.portal.query '[["comment","=","tf-acc-manual-e2e-portal"]]'` → empty
    - `midclt call iscsi.initiator.query '[["comment","=","tf-acc-manual-e2e-initiator"]]'` → empty
    - `midclt call iscsi.auth.query '[["tag","=",999],["user","=","tfaccmanuale2eauthuser"]]'` → empty
    - `midclt call pool.dataset.query '[["id","=","tank/tf-acc-manual-e2e-zvol"]]'` → empty
    - `midclt call iscsi.targetextent.query '[["target","=",<target_id>],["extent","=",<extent_id>]]'` → empty (using ids recorded before destroy)

**Expected results:** The full chain wires up correctly with CHAP authentication end to end: target group references the portal, initiator group, and auth tag by ID/tag, and the targetextent LUN mapping links target and extent. The update step proves partial changes don't cascade to unrelated resources in the chain. All imports round-trip cleanly. Destroy tears down the entire chain in dependency order with nothing orphaned, and the box's live iSCSI configuration (portal/target/extent id=1, targetextent id=2, if present) remains completely untouched throughout.

**Cleanup:** None beyond step 12/13. If destroy is interrupted partway, manually remove in this order via UI or `midclt`: targetextent, target, extent, zvol, auth, initiator, portal.

---

### 8. truenas_nvmet_global

**MT-BLOCK-010 — Read the live NVMe-oF global configuration via datasource**

**Resource:** `truenas_nvmet_global` (datasource)
**Type:** Singleton (read-only)
**Environment:** Any TrueNAS box, including a production-serving one — this case makes no writes.
**Preconditions:** NVMe-oF target service configured (config exists even if the service isn't running).

**Config:**
```hcl
data "truenas_nvmet_global" "test" {}

output "nvmet_global" {
  value = data.truenas_nvmet_global.test
}
```

**Steps:**
1. `terraform init`
2. `terraform apply -auto-approve`
3. Inspect: `terraform output nvmet_global`
4. Cross-check against `midclt call nvmet.global.config`

**Expected results:** `id = "nvmet_global"`, `basenqn` is set (non-empty), `ana`/`kernel`/`rdma`/`xport_referral` are booleans matching the `midclt` output.

**Cleanup:** `terraform destroy -auto-approve` (state-only; no API calls).

---

**MT-BLOCK-011 — Singleton set-and-restore: xport_referral (DISRUPTIVE)**

**Resource:** `truenas_nvmet_global` (resource)
**Type:** Singleton — set-and-restore, DISRUPTIVE
**Environment:** A box where you are authorized to briefly toggle the NVMe-oF discovery transport-referral flag. Per the acceptance-test source (`internal/resources/nvmet_global/acceptance_test.go`), `xport_referral` only controls whether NVMe-oF discovery responses advertise port referrals (an advisory discovery-log hint) and does not gate or restart any port or subsystem — safe on a production-serving box, but still restore the original value. Do NOT extend this test to `basenqn`, `ana`, `kernel`, or `rdma` — those can disrupt active NVMe-oF connections or discovery and are excluded from this manual test by design.
**Preconditions:** Record the current value: `midclt call nvmet.global.config | jq .xport_referral`.

**Config (step A — set to the opposite of the recorded value, e.g. if original is `false`):**
```hcl
resource "truenas_nvmet_global" "test" {
  xport_referral = true
}
```

**Config (step B — restore original value):**
```hcl
resource "truenas_nvmet_global" "test" {
  xport_referral = false   # substitute the box's actual original value
}
```

**Steps:**
1. `terraform init`
2. Apply Config A: `terraform apply -auto-approve`
3. Verify: `terraform state show truenas_nvmet_global.test` shows `xport_referral = true`; `midclt call nvmet.global.config | jq .xport_referral` returns `true`. UI: Shares > Block Shares (NVMe-oF Targets) > wrench icon > Global Configuration (if exposed) or System Settings > Services > NVMe-oF Target > Configure.
4. Apply Config B (restore): `terraform apply -auto-approve`
5. Verify `xport_referral` matches the pre-test value recorded in Preconditions.
6. `terraform import truenas_nvmet_global.test nvmet_global`
7. Verify imported state matches applied state (`basenqn`, `ana`, `kernel`, `rdma`, `xport_referral`).
8. `terraform destroy -auto-approve`

**Expected results:** Step 3 confirms the flag toggled with no other NVMe-oF global fields altered and no interruption to any active NVMe-oF connection. Step 5 confirms restoration. Import uses the fixed ID string `nvmet_global`. Destroy in step 8 emits a warning ("NVMe-oF global configuration left in place") and makes no API call — `midclt call nvmet.global.config` after destroy still shows the restored value from step 5.

**Cleanup:** Confirm via `midclt call nvmet.global.config` that `xport_referral` matches the box's pre-test value.

---

### 9. truenas_nvmet_subsys

**MT-BLOCK-012 — Create, update, import, and destroy an NVMe-oF subsystem**

**Resource:** `truenas_nvmet_subsys`
**Type:** CRUD
**Environment:** Any TrueNAS box. `name` is `RequiresReplace`. Never touches any pre-existing live subsystem on the box.
**Preconditions:** None.

**Config (initial):**
```hcl
resource "truenas_nvmet_subsys" "test" {
  name           = "tf-acc-manual-subsys"
  allow_any_host = false
}
```

**Config (update — flip allow_any_host):**
```hcl
resource "truenas_nvmet_subsys" "test" {
  name           = "tf-acc-manual-subsys"
  allow_any_host = true
}
```

**Steps:**
1. `terraform init`
2. Apply initial config: `terraform apply -auto-approve`
3. Verify: `name`, `allow_any_host = false`, `id`/`subnqn`/`serial` set (server-derived when `subnqn` is left unset). Cross-check: `midclt call nvmet.subsys.query '[["name","=","tf-acc-manual-subsys"]]'`. UI: Shares > Block Shares (NVMe-oF Targets) > Subsystems.
4. Apply update config: `terraform apply -auto-approve`
5. Verify `allow_any_host = true`; `subnqn`/`serial`/`id` unchanged.
6. `terraform import truenas_nvmet_subsys.import_test <id>`
7. Verify imported state matches.
8. `terraform destroy -auto-approve`
9. Post-destroy: `midclt call nvmet.subsys.query '[["name","=","tf-acc-manual-subsys"]]'` returns empty.

**Expected results:** Subsystem created with a server-derived NQN; `allow_any_host` updates in place without replacement.

**Cleanup:** None beyond step 8/9.

---

### 10. truenas_nvmet_port

**MT-BLOCK-013 — Create, update, import, and destroy an NVMe-oF TCP port**

**Resource:** `truenas_nvmet_port`
**Type:** CRUD
**Environment:** Any TrueNAS box. Uses service ID **14421** (distinct from the box's live port, typically `id=1` TCP `4420`, and from the end-to-end test's port on 14420 in MT-BLOCK-014). `addr_trtype` is `RequiresReplace`. Created with `enabled = false` initially so it never opens a live listener during this test.
**Preconditions:** Know the box's iSCSI/NVMe-facing IP address.

**Config (initial — disabled):**
```hcl
resource "truenas_nvmet_port" "test" {
  addr_trtype  = "TCP"
  addr_traddr  = "10.0.0.10"   # substitute the box's own IP
  addr_trsvcid = 14421
  enabled      = false
}
```

**Config (update — set max_queue_size, still disabled):**
```hcl
resource "truenas_nvmet_port" "test" {
  addr_trtype    = "TCP"
  addr_traddr    = "10.0.0.10"
  addr_trsvcid   = 14421
  enabled        = false
  max_queue_size = 128
}
```

**Steps:**
1. `terraform init`
2. Apply initial config: `terraform apply -auto-approve`
3. Verify: `addr_trtype = "TCP"`, `addr_traddr` = box IP, `addr_trsvcid = 14421`, `enabled = false`, `id`/`index`/`addr_adrfam` set. Cross-check: `midclt call nvmet.port.query '[["addr_trsvcid","=",14421]]'`. UI: Shares > Block Shares (NVMe-oF Targets) > Ports. Confirm no listener is actually open: `ss -tln | grep 14421` returns nothing (port disabled).
4. Apply update config: `terraform apply -auto-approve`
5. Verify `max_queue_size = 128`; other fields unchanged.
6. `terraform import truenas_nvmet_port.import_test <id>`
7. Verify imported state matches.
8. `terraform destroy -auto-approve`
9. Post-destroy: `midclt call nvmet.port.query '[["addr_trsvcid","=",14421]]'` returns empty.

**Expected results:** Port created but never listening (disabled throughout this test). Update sets `max_queue_size` in place.

**Cleanup:** None beyond step 8/9.

---

### 11. truenas_nvmet_namespace

**MT-BLOCK-014 — Create a ZVOL-backed namespace under an own subsystem, update, import, destroy**

**Resource:** `truenas_nvmet_namespace`
**Type:** CRUD
**Environment:** Any TrueNAS box with a usable pool. `subsys_id` and `nsid` are both `RequiresReplace`. Never touches the box's live subsystem/namespace (commonly `id=1`).
**Preconditions:** A pool exists (default `tank`); this case creates its own subsystem and zvol fixtures.

**Config (initial — enabled):**
```hcl
resource "truenas_nvmet_subsys" "test" {
  name = "tf-acc-manual-ns-subsys"
}

resource "truenas_zvol" "test" {
  name    = "tank/tf-acc-manual-ns-zvol"
  volsize = 67108864
}

resource "truenas_nvmet_namespace" "test" {
  subsys_id   = truenas_nvmet_subsys.test.id
  device_path = "zvol/${truenas_zvol.test.name}"
  device_type = "ZVOL"
  enabled     = true
}
```

**Config (update — disabled):**
```hcl
resource "truenas_nvmet_subsys" "test" {
  name = "tf-acc-manual-ns-subsys"
}

resource "truenas_zvol" "test" {
  name    = "tank/tf-acc-manual-ns-zvol"
  volsize = 67108864
}

resource "truenas_nvmet_namespace" "test" {
  subsys_id   = truenas_nvmet_subsys.test.id
  device_path = "zvol/${truenas_zvol.test.name}"
  device_type = "ZVOL"
  enabled     = false
}
```

**Steps:**
1. `terraform init`
2. Apply initial config: `terraform apply -auto-approve`
3. Verify: `device_path = "zvol/tank/tf-acc-manual-ns-zvol"`, `device_type = "ZVOL"`, `enabled = true`, `id`/`nsid`/`device_nguid`/`device_uuid` set. Cross-check: `midclt call nvmet.namespace.query '[["device_path","=","zvol/tank/tf-acc-manual-ns-zvol"]]'`. UI: Shares > Block Shares (NVMe-oF Targets) > Namespaces.
4. Apply update config: `terraform apply -auto-approve`
5. Verify `enabled = false`; `nsid`/`device_nguid`/`device_uuid`/`id` unchanged (in-place update; `nsid` is immutable after create but is not being changed here).
6. `terraform import truenas_nvmet_namespace.import_test <id>`
7. Verify imported state matches.
8. `terraform destroy -auto-approve` (destroys namespace, then subsystem and zvol)
9. Post-destroy: `midclt call nvmet.namespace.query '[["device_path","=","zvol/tank/tf-acc-manual-ns-zvol"]]'` returns empty; `midclt call pool.dataset.query '[["id","=","tank/tf-acc-manual-ns-zvol"]]'` returns empty.

**Expected results:** Namespace created over the zvol with auto-assigned `nsid`; `enabled` toggles in place.

**Cleanup:** None beyond step 8/9.

---

### 12. truenas_nvmet_host

**MT-BLOCK-015 — Create, rename in place, import, and destroy an NVMe-oF host**

**Resource:** `truenas_nvmet_host`
**Type:** CRUD
**Environment:** Any TrueNAS box. `hostnqn` must be a full RFC-4122 UUID-form NQN (`nqn.2014-08.org.nvmexpress:uuid:<uuid>`) — a short/abbreviated suffix is rejected by TrueNAS ("uuid is incorrect length"). `description` only exists on the wire from TrueNAS 26.0+; omit it on older releases. `hostnqn` is mutable in place (no `RequiresReplace`).
**Preconditions:** Generate a valid v4 UUID for the NQN, e.g. via `uuidgen` (lowercase). Confirm the box's TrueNAS version if testing `description`: `midclt call system.version`.

**Config (initial):**
```hcl
resource "truenas_nvmet_host" "test" {
  hostnqn     = "nqn.2014-08.org.nvmexpress:uuid:11111111-1111-4111-8111-111111111111"
  description = "tf-acc-manual host"   # omit this line on TrueNAS < 26.0
}
```

**Config (update — description change, same NQN):**
```hcl
resource "truenas_nvmet_host" "test" {
  hostnqn     = "nqn.2014-08.org.nvmexpress:uuid:11111111-1111-4111-8111-111111111111"
  description = "tf-acc-manual host updated"
}
```

**Config (rename — new NQN, in place):**
```hcl
resource "truenas_nvmet_host" "test" {
  hostnqn     = "nqn.2014-08.org.nvmexpress:uuid:22222222-2222-4222-8222-222222222222"
  description = "tf-acc-manual host updated"
}
```

**Steps:**
1. `terraform init`
2. Apply initial config: `terraform apply -auto-approve`
3. Verify: `hostnqn` matches, `description` matches (if applicable), `id` set. `dhchap_key`/`dhchap_ctrl_key` are write-only and never appear in state. Cross-check: `midclt call nvmet.host.query '[["hostnqn","=","nqn.2014-08.org.nvmexpress:uuid:11111111-1111-4111-8111-111111111111"]]'`. UI: Shares > Block Shares (NVMe-oF Targets) > Hosts.
4. Apply description-update config: `terraform apply -auto-approve`
5. Verify `description` updated, `id`/`hostnqn` unchanged.
6. Apply rename config: `terraform apply -auto-approve`. Confirm `terraform plan` before applying shows an **in-place update** (not destroy/create) — `hostnqn` has no `RequiresReplace`.
7. Verify `hostnqn` is now the new UUID NQN, `id` is unchanged from step 3 (same underlying object, renamed).
8. `terraform import truenas_nvmet_host.import_test <id>`. `dhchap_key`/`dhchap_ctrl_key` are expected to import as null/absent (write-only, never read back) — this is expected, not a bug; compare all other fields.
9. `terraform destroy -auto-approve`
10. Post-destroy: `midclt call nvmet.host.query '[["hostnqn","=","nqn.2014-08.org.nvmexpress:uuid:22222222-2222-4222-8222-222222222222"]]'` returns empty.

**Expected results:** Host created, description updates in place, and renaming `hostnqn` updates the existing object in place rather than replacing it (same `id` before/after step 6/7). `dhchap_key`/`dhchap_ctrl_key` never leak into state or plan output.

**Cleanup:** None beyond step 9/10.

---

### 13. truenas_nvmet_host_subsys

**MT-BLOCK-016 — Associate an NVMe-oF host with a subsystem (access-control grant), import, destroy**

**Resource:** `truenas_nvmet_host_subsys`
**Type:** CRUD (both `host_id` and `subsys_id` are `RequiresReplace` — no in-place update path exists)
**Environment:** Any TrueNAS box. Never touches the box's live host_subsys association (commonly `id=1`).
**Preconditions:** None; this case creates its own host and subsystem fixtures.

**Config:**
```hcl
resource "truenas_nvmet_host" "test" {
  hostnqn = "nqn.2014-08.org.nvmexpress:uuid:33333333-3333-4333-8333-333333333333"
}

resource "truenas_nvmet_subsys" "test" {
  name = "tf-acc-manual-host-subsys"
}

resource "truenas_nvmet_host_subsys" "test" {
  host_id   = truenas_nvmet_host.test.id
  subsys_id = truenas_nvmet_subsys.test.id
}
```

**Steps:**
1. `terraform init`
2. `terraform apply -auto-approve`
3. Verify: `id` set, `host_id` matches the host fixture's `id`, `subsys_id` matches the subsystem fixture's `id`. Cross-check: `midclt call nvmet.host_subsys.query '[["id","=",<id>]]'`. UI: Shares > Block Shares (NVMe-oF Targets) > Subsystems > (subsystem detail) > Hosts, or Hosts > (host detail) > Subsystems, depending on UI version.
4. `terraform import truenas_nvmet_host_subsys.import_test <id>`
5. Verify imported state matches.
6. `terraform destroy -auto-approve` (destroys the association, then host and subsystem)
7. Post-destroy: `midclt call nvmet.host_subsys.query '[["id","=",<id>]]'` returns empty (use the id captured in step 3, since host/subsys can no longer be looked up by name).

**Expected results:** Association grants the host access to the subsystem; since neither `host_id` nor `subsys_id` supports in-place update, any change to either field forces replacement (optionally verify: edit `host_id` to reference a second host fixture and confirm `terraform plan` shows destroy/create, not update).

**Cleanup:** None beyond step 6/7.

---

### 14. truenas_nvmet_port_subsys

**MT-BLOCK-017 — Associate an NVMe-oF port with a subsystem (advertise subsystem on port), import, destroy (standalone)**

**Resource:** `truenas_nvmet_port_subsys`
**Type:** CRUD (both `port_id` and `subsys_id` are `RequiresReplace`)
**Environment:** Any TrueNAS box. Uses a disabled test port on service ID 14421 so no live listener opens. Never touches the box's live port_subsys association (commonly `id=1`). This standalone case is a minimal pairing; use MT-BLOCK-018 to exercise the full wiring chain.
**Preconditions:** None; this case creates its own port and subsystem fixtures.

**Config:**
```hcl
resource "truenas_nvmet_port" "test" {
  addr_trtype  = "TCP"
  addr_traddr  = "10.0.0.10"   # substitute the box's own IP
  addr_trsvcid = 14421
  enabled      = false
}

resource "truenas_nvmet_subsys" "test" {
  name = "tf-acc-manual-port-subsys"
}

resource "truenas_nvmet_port_subsys" "test" {
  port_id   = truenas_nvmet_port.test.id
  subsys_id = truenas_nvmet_subsys.test.id
}
```

**Steps:**
1. `terraform init`
2. `terraform apply -auto-approve`
3. Verify: `id` set, `port_id` matches the port fixture's `id`, `subsys_id` matches the subsystem fixture's `id`. Cross-check: `midclt call nvmet.port_subsys.query '[["id","=",<id>]]'`. UI: Shares > Block Shares (NVMe-oF Targets) > Ports > (port detail) > Subsystems.
4. `terraform import truenas_nvmet_port_subsys.import_test <id>`
5. Verify imported state matches.
6. `terraform destroy -auto-approve` (destroys the association, then port and subsystem)
7. Post-destroy: `midclt call nvmet.port_subsys.query '[["id","=",<id>]]'` returns empty (use the id captured in step 3).

**Expected results:** Association advertises the subsystem on the port; standard CRUD behavior with RequiresReplace on both keys.

**Cleanup:** None beyond step 6/7.

---

**MT-BLOCK-018 — Full NVMe-oF integration: subsys → port → namespace → host → host/subsys and port/subsys associations (end-to-end wiring)**

**Resource:** `truenas_nvmet_subsys`, `truenas_nvmet_port`, `truenas_zvol`, `truenas_nvmet_namespace`, `truenas_nvmet_host`, `truenas_nvmet_host_subsys`, `truenas_nvmet_port_subsys`
**Type:** Integration (multi-resource)
**Environment:** Any TrueNAS box, including production-serving ones — this test is entirely self-contained and never references any pre-existing live NVMe-oF configuration on the box (live subsys/port/namespace/port_subsys, if present). The test port listens on the box IP at service ID **14420** (distinct from the live port's 4420) and is created with `enabled = false` throughout so it never opens a live listener. This reproduces `TestAccNVMeTEndToEnd` in `internal/resources/nvmet_port_subsys/acceptance_test.go`. `description` on the host only exists on the wire from TrueNAS 26.0+; omit it on older releases.
**Preconditions:** A pool exists (default `tank`). Generate a valid v4 UUID NQN for the host fixture. Confirm TrueNAS version for the `description` field: `midclt call system.version`.

**Config (initial — namespace enabled, host description set):**
```hcl
resource "truenas_nvmet_subsys" "test" {
  name           = "tf-acc-manual-e2e-nvmet-subsys"
  allow_any_host = false
}

resource "truenas_nvmet_port" "test" {
  addr_trtype  = "TCP"
  addr_traddr  = "10.0.0.10"   # substitute the box's own IP
  addr_trsvcid = 14420
  enabled      = false
}

resource "truenas_zvol" "test" {
  name    = "tank/tf-acc-manual-e2e-nvmet-zvol"
  volsize = 67108864
}

resource "truenas_nvmet_namespace" "test" {
  subsys_id   = truenas_nvmet_subsys.test.id
  device_path = "zvol/${truenas_zvol.test.name}"
  device_type = "ZVOL"
  enabled     = true
}

resource "truenas_nvmet_host" "test" {
  hostnqn     = "nqn.2014-08.org.nvmexpress:uuid:44444444-4444-4444-8444-444444444444"
  description = "initial host description"   # omit on TrueNAS < 26.0
}

resource "truenas_nvmet_host_subsys" "test" {
  host_id   = truenas_nvmet_host.test.id
  subsys_id = truenas_nvmet_subsys.test.id
}

resource "truenas_nvmet_port_subsys" "test" {
  port_id   = truenas_nvmet_port.test.id
  subsys_id = truenas_nvmet_subsys.test.id
}
```

**Config (update — disable namespace and change host description; subsys.allow_any_host deliberately left untouched since toggling it would conflict with the explicit host_subsys grant already created):**
```hcl
# Same as above, except:
#   truenas_nvmet_namespace.test.enabled       = false
#   truenas_nvmet_host.test.description        = "updated host description"
```

**Steps:**
1. `terraform init`
2. `terraform plan` — confirm the plan creates all 7 resources with correct dependency ordering (subsys/port/zvol before namespace/host, host+subsys before host_subsys, port+subsys before port_subsys).
3. `terraform apply -auto-approve`
4. Verify each resource:
   - Subsys: `name`, `allow_any_host = false`, `serial` set. `midclt call nvmet.subsys.query '[["name","=","tf-acc-manual-e2e-nvmet-subsys"]]'`
   - Port: `addr_trtype = "TCP"`, `addr_traddr` = box IP, `addr_trsvcid = 14420`, `enabled = false`. `midclt call nvmet.port.query '[["addr_trsvcid","=",14420]]'`. Confirm no live listener: `ss -tln | grep 14420` returns nothing.
   - Zvol: `name = "tank/tf-acc-manual-e2e-nvmet-zvol"`. `midclt call pool.dataset.query '[["id","=","tank/tf-acc-manual-e2e-nvmet-zvol"]]'`
   - Namespace: `device_path = "zvol/tank/tf-acc-manual-e2e-nvmet-zvol"`, `device_type = "ZVOL"`, `enabled = true`, `subsys_id` matches subsys id, `nsid` set. `midclt call nvmet.namespace.query '[["device_path","=","zvol/tank/tf-acc-manual-e2e-nvmet-zvol"]]'`
   - Host: `hostnqn` matches, `description = "initial host description"` (if 26.0+). `midclt call nvmet.host.query '[["hostnqn","=","nqn.2014-08.org.nvmexpress:uuid:44444444-4444-4444-8444-444444444444"]]'`
   - Host/subsys: `host_id`/`subsys_id` match. `midclt call nvmet.host_subsys.query '[["host_id","=",<host_id>],["subsys_id","=",<subsys_id>]]'`
   - Port/subsys: `port_id`/`subsys_id` match. `midclt call nvmet.port_subsys.query '[["port_id","=",<port_id>],["subsys_id","=",<subsys_id>]]'`
5. UI walkthrough: Shares > Block Shares (NVMe-oF Targets) — confirm subsystem, port, namespace, host, and both associations are visible and cross-referenced correctly.
6. Apply the update config: `terraform apply -auto-approve`
7. Verify only `truenas_nvmet_namespace.test.enabled` (now `false`) and `truenas_nvmet_host.test.description` (now "updated host description") changed; `truenas_nvmet_subsys.test.allow_any_host` remains `false` and all other resources are untouched (`terraform plan` after step 6 shows zero diff; all `id`s unchanged from step 4).
8. `terraform import truenas_nvmet_port_subsys.test <port_subsys_id>` — verify clean import.
9. `terraform import truenas_nvmet_host_subsys.test <host_subsys_id>` — verify clean import.
10. `terraform import truenas_nvmet_namespace.test <namespace_id>` — verify clean import.
11. `terraform destroy -auto-approve`
12. Post-destroy verification (some by name, some by id captured before destroy since host_subsys/port_subsys can no longer be looked up by name):
    - `midclt call nvmet.subsys.query '[["name","=","tf-acc-manual-e2e-nvmet-subsys"]]'` → empty
    - `midclt call nvmet.port.query '[["addr_traddr","=","10.0.0.10"],["addr_trsvcid","=",14420]]'` → empty
    - `midclt call nvmet.namespace.query '[["device_path","=","zvol/tank/tf-acc-manual-e2e-nvmet-zvol"]]'` → empty
    - `midclt call nvmet.host.query '[["hostnqn","=","nqn.2014-08.org.nvmexpress:uuid:44444444-4444-4444-8444-444444444444"]]'` → empty
    - `midclt call pool.dataset.query '[["id","=","tank/tf-acc-manual-e2e-nvmet-zvol"]]'` → empty
    - `midclt call nvmet.host_subsys.query '[["host_id","=",<host_id>],["subsys_id","=",<subsys_id>]]'` → empty (ids from step 4)
    - `midclt call nvmet.port_subsys.query '[["port_id","=",<port_id>],["subsys_id","=",<subsys_id>]]'` → empty (ids from step 4)

**Expected results:** The full NVMe-oF chain wires up correctly: namespace attached to the subsystem, host granted access via host_subsys, subsystem advertised on the port via port_subsys. The update step proves partial changes (namespace enabled flag, host description) don't cascade to unrelated resources — in particular, `allow_any_host` on the subsystem stays `false` throughout, since flipping it would conflict with the explicit per-host grant. All imports round-trip cleanly. Destroy tears down the entire chain with nothing orphaned, the disabled test port never opens a real listener at any point, and the box's live NVMe-oF configuration (subsys/port/namespace/port_subsys id=1, if present) remains completely untouched throughout.

**Cleanup:** None beyond step 11/12. If destroy is interrupted partway, manually remove in this order via UI or `midclt`: port_subsys, host_subsys, namespace, host, port, subsys, zvol.

---

## Accounts, Access, and Directory Services

These cases are run by hand with the Terraform CLI plus the TrueNAS web UI and
`midclt call` (run in a TrueNAS shell, or via SSH to the TrueNAS box) as the
verification tools — none of them use the Go acceptance-test harness in this
repository. All resource/user/group names use a `tf-acc-manual-` prefix so
they are easy to distinguish from anything the Go harness may have left
behind, and easy to search for and remove if a case is aborted partway
through.

**Shared setup (do this once before running any case below):**

1. Create a working directory, e.g. `~/tf-manual-tests/`, and `cd` into it.
2. Create `provider.tf`:

   ```hcl
   terraform {
     required_version = ">= 1.11.0" # required for WriteOnly attributes used below
     required_providers {
       truenas = {
         source = "truenas/truenas"
       }
     }
   }

   provider "truenas" {
     endpoint = var.truenas_endpoint
     insecure = true # or set ca_cert instead, if the box has a trusted cert
   }
   ```

3. Create `variables.tf`:

   ```hcl
   variable "truenas_endpoint" {
     type        = string
     description = "wss://<host>/api/current"
   }

   variable "truenas_username" {
     type = string
   }

   variable "truenas_password" {
     type      = string
     sensitive = true
   }
   ```

4. Create `terraform.tfvars` (do **not** commit this file) with real values
   for `truenas_endpoint`, `truenas_username`, `truenas_password` for a local
   admin account on the target TrueNAS system.
5. Run `terraform init`.

Each case below adds its own resource block(s) to a new `.tf` file in this
same directory (e.g. `case-001.tf`), applies, verifies, and then removes the
file (or runs `terraform destroy -target=...`) as part of Cleanup. Additional
`variable` blocks a case needs for secrets are shown in that case's Config.

UI navigation below reflects TrueNAS 25.10; menu wording can shift a
little between releases — if a path doesn't match what's on screen, treat the
`midclt call` output as the source of truth.

### 1. truenas_user

**MT-IDENTITY-001 — Local user full lifecycle (create, update, import, destroy)**

**Resource:** `truenas_user`
**Type:** Functional — full lifecycle
**Environment:** Any TrueNAS system reachable over the management
network; local admin credentials (or an API key with the `FULL_ADMIN`/local
administrator role).
**Preconditions:**
- Terraform CLI >= 1.11.0 (the `password` attribute is WriteOnly).
- No existing local user named `tf-acc-manual-user-01` on the target system
  (`Credentials > Local Users` in the UI, or `midclt call user.query
  '[["username","=","tf-acc-manual-user-01"]]'` returns `[]`).

**Config:**
```hcl
resource "truenas_user" "test" {
  username          = "tf-acc-manual-user-01"
  full_name         = "TF Manual Test User"
  password          = "Tf-Acc-Manual-Passw0rd!"
  password_disabled = false
  home              = "/var/empty"
  shell             = "/usr/bin/bash"
  smb               = false
  group_create      = true
}
```

**Steps:**
1. Save the config above as `case-001.tf` and run `terraform apply`. Confirm
   the plan shows one resource to add, then approve.
2. Verify in the UI: open `Credentials > Local Users`, find
   `tf-acc-manual-user-01`, confirm Full Name = "TF Manual Test User", Shell =
   `/usr/bin/bash`, Home Directory = `/var/empty`, Locked = No.
3. Verify via API: `midclt call user.query
   '[["username","=","tf-acc-manual-user-01"]]'`. Confirm the single result's
   `full_name`, `shell`, `home`, `locked` (`false`), `smb` (`false`), and
   that `group` resolves to a newly created group also named
   `tf-acc-manual-user-01` (a side effect of `group_create = true`).
4. Update in place: change `full_name = "Updated Manual Test User"` in
   `case-001.tf`, leave everything else unchanged, and run `terraform apply`.
   Confirm the plan shows an in-place update (no replacement) affecting only
   `full_name`.
5. Verify the update via UI and via `midclt call user.query
   '[["username","=","tf-acc-manual-user-01"]]'` — `full_name` now reads
   "Updated Manual Test User".
6. Import: note the user's numeric id from `terraform state show
   truenas_user.test` (or the prior query's `id` field), then run
   `terraform state rm truenas_user.test` followed by `terraform import
   truenas_user.test <id>`.
7. Run `terraform plan` after the import. Expect a diff only on `password`
   and `group_create` — both are write-only/create-only and are never read
   back from TrueNAS, so Terraform correctly shows them as needing to be
   (re)supplied; every other attribute should show no difference from the
   live object.
8. Destroy: run `terraform destroy` and approve.
9. Post-destroy verify: `midclt call user.query
   '[["username","=","tf-acc-manual-user-01"]]'` returns `[]`; the user is
   also gone from `Credentials > Local Users` in the UI. Confirm the
   `group_create`-created group `tf-acc-manual-user-01` was also removed
   (`midclt call group.query '[["group","=","tf-acc-manual-user-01"]]'`
   returns `[]` — TrueNAS removes an auto-created primary group when its
   owning user is deleted, as long as nothing else references it).

**Expected results:** User creates with the configured attributes, updates
`full_name` in place without replacement, imports cleanly (modulo the two
known write-only/create-only fields), and is fully removed — along with its
auto-created primary group — on destroy.
**Cleanup:** If any step fails partway through, delete
`tf-acc-manual-user-01` directly via the UI or `midclt call user.query` +
`midclt call user.delete <id>`, then remove `case-001.tf`.

### 2. truenas_group

**MT-IDENTITY-002 — Local group full lifecycle (create, update, import, destroy)**

**Resource:** `truenas_group`
**Type:** Functional — full lifecycle
**Environment:** Any TrueNAS system; local admin credentials.
**Preconditions:** No existing group named `tf-acc-manual-grp-01`
(`Credentials > Local Groups`, or `midclt call group.query
'[["group","=","tf-acc-manual-grp-01"]]'` returns `[]`).

**Config:**
```hcl
resource "truenas_group" "test" {
  name          = "tf-acc-manual-grp-01"
  smb           = false
  sudo_commands = []
}
```

**Steps:**
1. Save as `case-002.tf`, `terraform apply`.
2. Verify via UI (`Credentials > Local Groups`, find
   `tf-acc-manual-grp-01`, SMB = No) and via `midclt call group.query
   '[["group","=","tf-acc-manual-grp-01"]]'` (`smb: false`,
   `sudo_commands: []`).
3. Update in place: set `smb = true` and `sudo_commands = ["/usr/bin/whoami",
   "/usr/bin/id"]`, `terraform apply`. Confirm no replacement is planned.
4. Verify: UI shows SMB = Yes; `midclt call group.query
   '[["group","=","tf-acc-manual-grp-01"]]'` shows `smb: true` and
   `sudo_commands: ["/usr/bin/whoami", "/usr/bin/id"]` in that order.
5. Import: get the numeric id, `terraform state rm truenas_group.test`,
   `terraform import truenas_group.test <id>`, then `terraform plan` — expect
   no differences at all (unlike `truenas_user`, nothing in this resource is
   write-only).
6. `terraform destroy`.
7. Post-destroy verify: `midclt call group.query
   '[["group","=","tf-acc-manual-grp-01"]]'` returns `[]`; group is gone from
   the UI.

**Expected results:** Group creates, updates `smb`/`sudo_commands` in place,
imports with a clean (empty) diff, and is fully removed on destroy.
**Cleanup:** Remove `tf-acc-manual-grp-01` directly if any step fails, then
remove `case-002.tf`.

### 3. truenas_api_key

**MT-IDENTITY-003 — API key create, second-session authentication check, rename, import, destroy**

**Resource:** `truenas_api_key`
**Type:** Functional — full lifecycle plus credential verification
**Environment:** Any TrueNAS system; local admin credentials for the
Terraform provider connection, plus a local user account to own the new key
(the built-in `truenas_admin` account is a convenient default; substitute
any local user you have credentials for).
**Preconditions:** No existing API key named `tf-acc-manual-key-01`
(`Credentials > API Keys` in the UI, or `midclt call api_key.query
'[["name","=","tf-acc-manual-key-01"]]'` returns `[]`).

**Config:**
```hcl
variable "api_key_owner" {
  type    = string
  default = "truenas_admin"
}

resource "truenas_api_key" "test" {
  name     = "tf-acc-manual-key-01"
  username = var.api_key_owner
}

output "api_key_value" {
  value     = truenas_api_key.test.key
  sensitive = true
}
```

**Steps:**
1. Save as `case-003.tf`, `terraform apply`.
2. Verify via UI: `Credentials > API Keys` shows `tf-acc-manual-key-01`
   owned by the configured user, with a "Local" badge and not revoked.
3. Verify via API: `midclt call api_key.query
   '[["name","=","tf-acc-manual-key-01"]]'` — confirm `username` matches,
   `local: true`, `revoked: false`, and that this response does **not**
   include the plaintext key (`api_key.query` never returns it — only
   `api_key.create`'s own response does, which is why the provider stores
   it in state at creation and never re-reads it afterward).
4. Retrieve the plaintext key value: `terraform output -raw api_key_value`.
5. Authenticate a second, independent session with that key —
   this is the load-bearing check for this resource, proving the key
   TrueNAS handed back actually works, not just that Terraform stored a
   string. Easiest method, from any machine with network access to the box:
   ```
   curl -sk -H "Authorization: Bearer <key from step 4>" \
     https://<truenas-host>/api/v2.0/system/version
   ```
   Expect an HTTP 200 with a version string body. (If your release has
   retired the `/api/v2.0` REST alias, use the same Bearer token against
   whatever current REST base path the box's API documentation page —
   `https://<truenas-host>/api/docs` — advertises.)
6. Rename in place: change `name = "tf-acc-manual-key-01-renamed"`,
   `terraform apply`. Confirm no replacement is planned and `key` in state is
   unchanged (`terraform output -raw api_key_value` still returns the same
   value as step 4).
7. Import: get the numeric id, `terraform state rm truenas_api_key.test`,
   `terraform import truenas_api_key.test <id>`, then `terraform plan` —
   expect a diff on `key` only (unknowable on import — TrueNAS never
   re-exposes a plaintext key after creation) and no diff on any other
   attribute.
8. `terraform destroy`.
9. Post-destroy verify: `midclt call api_key.query '[["id","=",<id>]]'`
   returns `[]`; re-running the step 5 `curl` command with the same key now
   fails authentication.

**Expected results:** Key creates and is usable to authenticate a brand-new
session immediately; rename is in-place; import correctly cannot recover the
plaintext key while every other attribute round-trips; destroy revokes the
key so the previously-working `curl` check now fails.
**Cleanup:** Remove any leftover key directly via `Credentials > API Keys`
if a step fails, then remove `case-003.tf`. Note: this resource does not
expose key rotation (TrueNAS's `reset` flag) — to rotate a key, `terraform
taint truenas_api_key.test` and re-apply to force a destroy+recreate.

### 4. truenas_privilege

**MT-IDENTITY-004 — Custom privilege full lifecycle (create, update, import, destroy)**

**Resource:** `truenas_privilege`
**Type:** Functional — full lifecycle
**Environment:** Any TrueNAS system; local admin credentials.
**Preconditions:** No existing privilege named `tf-acc-manual-priv-01` and no
group named `tf-acc-manual-priv-grp-01` (`Credentials > Privileges` /
`Credentials > Local Groups` in the UI, or `midclt call privilege.query
'[["name","=","tf-acc-manual-priv-01"]]'` and `midclt call group.query
'[["group","=","tf-acc-manual-priv-grp-01"]]'` both return `[]`).
**CAUTION:** Never modify or delete the three built-in privileges (`Local
Administrator`, `Read-Only Administrator`, `Sharing Administrator`) while
running or cleaning up this case — always filter by this case's own
`tf-acc-manual-priv-01` name.

**Config:**
```hcl
resource "truenas_group" "fixture" {
  name = "tf-acc-manual-priv-grp-01"
  smb  = false
}

resource "truenas_privilege" "test" {
  name         = "tf-acc-manual-priv-01"
  local_groups = [truenas_group.fixture.gid]
  roles        = ["READONLY_ADMIN"]
  web_shell    = false
}
```

**Steps:**
1. Save as `case-004.tf`, `terraform apply` (creates both resources).
2. Verify via UI: `Credentials > Privileges` shows `tf-acc-manual-priv-01`
   with the fixture group listed under Local Groups and role
   `READONLY_ADMIN`.
3. Verify via API: `midclt call privilege.query
   '[["name","=","tf-acc-manual-priv-01"]]'` — confirm `local_groups`
   contains an entry whose `gid` matches the fixture group's gid,
   `ds_groups` is `[]`, `roles` is `["READONLY_ADMIN"]`, `web_shell: false`,
   `builtin_name: null`.
4. Update in place: change `roles = ["READONLY_ADMIN", "SHARING_READ"]`,
   `terraform apply`. Confirm no replacement.
5. Verify: `midclt call privilege.query
   '[["name","=","tf-acc-manual-priv-01"]]'` shows `roles` with both entries
   in that order.
6. Import: get the privilege's numeric id, `terraform state rm
   truenas_privilege.test`, `terraform import truenas_privilege.test <id>`,
   `terraform plan` — expect no differences.
7. `terraform destroy` (removes both the privilege and the fixture group).
8. Post-destroy verify: both `midclt call privilege.query
   '[["name","=","tf-acc-manual-priv-01"]]'` and `midclt call group.query
   '[["group","=","tf-acc-manual-priv-grp-01"]]'` return `[]`; confirm the
   three built-in privileges are still present and untouched (`midclt call
   privilege.query '[]'` and check `builtin_name` is non-null for those
   three).

**Expected results:** Privilege creates referencing the fixture group's gid,
updates `roles` in place, imports cleanly, and both the privilege and the
fixture group are removed on destroy without touching any built-in
privilege.
**Cleanup:** Remove `tf-acc-manual-priv-01` and
`tf-acc-manual-priv-grp-01` directly if a step fails, then remove
`case-004.tf`.

### 5. truenas_twofactor_auth

This is a singleton, system-wide security setting: there is one
two-factor authentication configuration per TrueNAS system, so `create`/
`update` both call `auth.twofactor.update`, and `destroy` only removes the
resource from Terraform state — it never resets or disables 2FA on the box
(the provider emits a warning saying so instead). **`enabled` controls
whether 2FA is required system-wide.** API-key authentication (the kind this
provider itself uses) is unaffected by it either way, but flipping it can
lock password-based UI/SSH logins for accounts that don't have a TOTP secret
enrolled. Treat it with care.

**MT-IDENTITY-005 — Safe window-only set and restore**

**Resource:** `truenas_twofactor_auth`
**Type:** Functional — singleton set-and-restore (safe: `window` only)
**Environment:** Any TrueNAS system; local admin credentials. Safe to
run against a shared system, since `enabled` and `services` are never
touched.
**Preconditions:** None beyond normal access. Read and record the box's
current `window` value first — this test intentionally leaves the box
as it found it.

**Config:**
```hcl
resource "truenas_twofactor_auth" "test" {
  window = 90 # replace with (original window + 30) once you've read it below
}
```

**Steps:**
1. Read the current configuration before touching anything: `midclt call
   auth.twofactor.config`. Record the `window` value (call it `ORIG`) and the
   `enabled` value (for later confirmation it never changes).
2. Edit `case-005.tf` so `window = ORIG + 30`, save, `terraform apply`.
3. Verify: `midclt call auth.twofactor.config` shows `window: ORIG + 30` and
   `enabled` unchanged from step 1. (The UI's Two-Factor Authentication
   settings page — reachable from the top-right user-avatar menu, or
   `System Settings > General` depending on release — should also reflect
   the new window if it exposes the field; treat the API response as
   authoritative either way.)
4. Restore: edit `case-005.tf` back to `window = ORIG`, `terraform apply`.
5. Verify: `midclt call auth.twofactor.config` shows `window: ORIG` again.
6. Import: `terraform state rm truenas_twofactor_auth.test`, then
   `terraform import truenas_twofactor_auth.test twofactor_auth` (the fixed
   singleton id — any string works as the import id, the resource normalizes
   it). `terraform plan` afterward should show no differences.
7. `terraform destroy`.
8. Post-destroy verify: `midclt call auth.twofactor.config` is unchanged —
   `window` still `ORIG`, `enabled` still whatever it was in step 1. Destroy
   only removed the resource from Terraform state; it made no API call.

**Expected results:** `window` changes and restores cleanly through
`auth.twofactor.update`; `enabled`/`services` are never sent (config-driven
payload building means unset HCL attributes are never resent); destroy is a
no-op against the box and only affects Terraform state, with a warning
diagnostic to that effect in the `terraform destroy` output.
**Cleanup:** If a step fails before restore, immediately run `midclt call
auth.twofactor.update '{"window": ORIG}'` directly to put the box back,
then remove `case-005.tf`.

**MT-IDENTITY-006 — [MANUAL/SCRIPTED-ONLY, RISKY] System-wide 2FA enable/disable**

**Resource:** `truenas_twofactor_auth`
**Type:** Safety-restricted — manual/scripted-only, never part of an
automated suite
**Environment:** A disposable or otherwise non-production TrueNAS system
only. Local admin credentials for both the Terraform provider connection and
a second, independent verification path (see step 4).

> **WARNING — read before running this case.** Flipping `enabled` to `true`
> enforces two-factor authentication system-wide. A prior verification
> against a live box confirmed that API-key authentication (what this
> provider itself uses) keeps working normally with `enabled = true`, and
> that is what step 4 below reconfirms. What was **not** verified as safe is
> what happens to *password-based* logins for accounts that have never
> enrolled a TOTP secret — treat that as unknown/unsafe until you've checked
> it on the specific box in front of you. Do not run this case against a
> system anyone else depends on, and do not leave `enabled = true` set
> longer than the test requires.

**Preconditions:**
- Confirm every local user's 2FA enrollment status before starting (UI:
  `Credentials > Local Users`, each user's edit panel has a "Two-Factor
  Authentication" section showing whether a secret is configured; there is
  no bulk indicator, so check each account you care about individually).
  This case is meant to be run on a box where either (a) no account has an
  enrolled secret, or (b) you have already confirmed enrolled accounts can
  still complete a TOTP challenge.
- Record the current `enabled` value: `midclt call auth.twofactor.config`.

**Config:**
```hcl
resource "truenas_twofactor_auth" "test" {
  enabled = true # set back to the box's original value in the restore step
  window  = 90
}
```

**Steps:**
1. Save as `case-006.tf` with `enabled = true` (or whatever explicit boolean
   you're testing) and `terraform apply`.
2. Verify: `midclt call auth.twofactor.config` shows `enabled: true`.
3. From a **separate** terminal/session (do not reuse the Terraform
   provider's own connection), confirm password-based UI login behavior for
   a throwaway or non-critical local account: attempt to log in to the web
   UI with username+password only. Expect either a TOTP prompt (if a secret
   is enrolled) or — if no secret is enrolled — observe and record
   what TrueNAS does (this is the behavior this case exists to characterize
   on your box; it is not asserted here).
4. Confirm API-key auth is unaffected: repeat the `curl -sk -H
   "Authorization: Bearer <key>" https://<host>/api/v2.0/system/version`
   check from MT-IDENTITY-003 (any valid API key on the box) and confirm it
   still returns 200 with `enabled = true` in effect.
5. Restore immediately: edit `case-006.tf` back to `enabled = <original
   value from Preconditions>`, `terraform apply`.
6. Verify: `midclt call auth.twofactor.config` shows `enabled` back to its
   original value.
7. `terraform destroy` and remove `case-006.tf`.

**Expected results:** API-key authentication is unaffected by `enabled`
either way. Password-based login behavior for accounts without an enrolled
TOTP secret is the thing this case exists to observe and record for your
specific TrueNAS release — do not assume it matches a prior run on a
different box or release.
**Cleanup:** Restore `enabled` to its original value immediately (step 5) —
do not defer this. If Terraform itself fails partway through, restore
directly via `midclt call auth.twofactor.update '{"enabled": <original>}'`.

### 6. truenas_directoryservices

This is a singleton, system-wide resource: one directory services
configuration per TrueNAS system. `create`/`update` call
`directoryservices.update` (a job — joining/binding can take a while), and
after any update that leaves `enable = true` the provider polls
`directoryservices.status` until it reports `HEALTHY` (up to 5 minutes)
before considering the apply done. **`destroy` never calls
`directoryservices.leave`** — it only sends `enable = false`, disabling
directory services locally without removing the TrueNAS computer account
from the domain controller (AD/IPA) or unbinding (LDAP). This is deliberate:
leaving a domain needs a domain-admin credential that may not be available
(or wanted) at destroy time.

In the automated suite, these tests are gated behind `TRUENAS_DS=1` plus
`TRUENAS_DS_ALLOWED_ENDPOINT` (which must match the target
`TRUENAS_ENDPOINT`) — a guard against accidentally joining a shared or
production box. Running this by hand carries the same risk without that
guard rail, so the manual equivalent is: **before you start, confirm out
loud (or in a ticket) that the TrueNAS system in front of you is a
disposable test instance dedicated to this purpose.** Never run any of the
three cases below against a shared or production TrueNAS system, or against
a production directory server.

Each of the three cases below needs its own directory server prerequisite,
described generically (no specific lab addresses — substitute your own test
environment's hostname/IP):
- **AD case:** a reachable Active Directory domain controller, with a
  domain account that has rights to join computers to the domain (the
  bundled `Administrator`/domain-admin account is simplest for a disposable
  test domain).
- **LDAP case:** a reachable plain LDAP (or OpenLDAP) directory with a bind
  account that has read access to the relevant subtree, and at least one
  known test user entry under it.
- **IPA case:** a reachable FreeIPA server, with the realm's `admin`
  account credential (or another account with join rights).

**MT-IDENTITY-007 — Active Directory join, health check, update, destroy=disable**

**Resource:** `truenas_directoryservices`
**Type:** Functional — full join lifecycle (disruptive, disposable box only)
**Environment:** Disposable TrueNAS system; a reachable AD domain
controller as described above; Terraform CLI >= 1.11.0 (`credential.password`
is WriteOnly). Equivalent of the automated `TRUENAS_DS`/
`TRUENAS_DS_ALLOWED_ENDPOINT` gate: confirmed disposable box, confirmed
disposable domain, before starting.
**Preconditions:**
- `midclt call directoryservices.status` reports `{"status": null, ...}` (or
  `"DISABLED"`) — i.e. directory services are not currently enabled/mid-join
  on this box. If it reports anything else (`HEALTHY`, `JOINING`,
  `FAULTED`, `LEAVING`), stop — some other join is already in progress or
  left in a bad state and must be resolved first.
- Record the box's current DNS nameserver setting so it can be restored:
  `midclt call network.configuration.config` (note `nameserver1`).

**Config:**
```hcl
variable "ad_domain" {
  type = string # e.g. "example.internal"
}
variable "ad_hostname" {
  type    = string
  default = "tn-manual-test"
}
variable "ad_join_username" {
  type = string # domain account with join rights
}
variable "ad_join_password" {
  type      = string
  sensitive = true
}

resource "truenas_directoryservices" "test" {
  service_type = "ACTIVEDIRECTORY"
  enable       = true
  timeout      = 10

  credential = {
    credential_type = "KERBEROS_USER"
    username        = var.ad_join_username
    password        = var.ad_join_password
  }

  configuration_activedirectory = {
    hostname = var.ad_hostname
    domain   = var.ad_domain

    idmap = {
      builtin = {
        range_low  = 91000001
        range_high = 92000000
      }
      idmap_domain = {
        idmap_backend = "RID"
        range_low     = 201000001
        range_high    = 202000000
      }
    }
  }
}
```

**Steps:**
1. Point the box's nameserver at the AD domain controller (Active Directory
   join relies on DNS SRV lookups): UI `Network > Global Configuration >
   Nameserver 1`, or `midclt call network.configuration.update
   '{"nameserver1": "<dc-ip>"}'`. Confirm it applied (`midclt call
   network.configuration.config`).
2. Save the config above as `case-007.tf` with your `terraform.tfvars`
   populated for the four variables, `terraform apply`. This is a job-backed
   call, so it can take a while — Terraform will not return until the update
   job itself completes.
3. Poll for health: repeat `midclt call directoryservices.status` every 5-10
   seconds until `status` reads `"HEALTHY"` (or the UI's `Credentials >
   Directory Services` page shows a healthy/green status badge), up to 5
   minutes. If it reports `"FAULTED"`, read `status_msg` for the reason and
   stop — do not proceed until this is resolved.
4. Verify via UI: `Credentials > Directory Services` shows Active Directory
   enabled, domain matches `var.ad_domain`, hostname matches
   `var.ad_hostname`.
5. Verify via API: `midclt call directoryservices.config` — `service_type:
   "ACTIVEDIRECTORY"`, `enable: true`, `configuration.hostname`/`.domain`
   match, `configuration.idmap.builtin.range_low/range_high` = 91000001 /
   92000000, `configuration.idmap.idmap_domain.idmap_backend` = `"RID"`,
   `.range_low/.range_high` = 201000001 / 202000000 (confirming the explicit
   idmap block actually reached TrueNAS rather than a server-side default
   being silently substituted).
6. Verify on the domain side: confirm a computer account for
   `var.ad_hostname` now exists in Active Directory (e.g. via `Active
   Directory Users and Computers` on the DC, or `samba-tool computer show
   <hostname>` if it's a Samba AD DC).
7. Update in place: change `timeout = 20`, `terraform apply`. Confirm this
   does **not** trigger a rejoin — no new computer account, no change in
   `status_msg`/join timestamp — only `timeout` differs afterward
   (`midclt call directoryservices.config`).
8. Re-poll status once more and confirm it still reads `"HEALTHY"`.
9. `terraform destroy`.
10. Post-destroy verify: `midclt call directoryservices.config` shows
    `enable: false` but `service_type` **still** `"ACTIVEDIRECTORY"` (the
    join configuration is left in place, only disabled) — destroy must never
    have called `directoryservices.leave`. Confirm the computer account from
    step 6 is **still present** on the domain controller (proof that nothing
    was actually left/removed).
11. Restore the box's original nameserver: `midclt call
    network.configuration.update '{"nameserver1": "<value recorded in
    Preconditions>"}'`. Confirm via `midclt call
    network.configuration.config`.

**Expected results:** Join succeeds and reaches `HEALTHY`; the explicit
`idmap` block round-trips as configured; updating `timeout` alone
does not trigger a rejoin; destroy disables directory services locally
while leaving both the TrueNAS-side join configuration and the AD computer
account in place.
**Cleanup:** If the box is left joined and you want it fully clean, either
re-apply with `enable = true` then run a manual `directoryservices.leave`
via `midclt call directoryservices.leave '{"username": "<domain admin>",
"password": "<password>"}'` (this is intentionally outside what Terraform
does), or hand the disposable box back for a rebuild. Always restore the
nameserver (step 11) even if earlier steps failed.

**MT-IDENTITY-008 — Plain LDAP bind, health check, update, destroy=disable**

**Resource:** `truenas_directoryservices`
**Type:** Functional — full bind lifecycle (disruptive, disposable box only)
**Environment:** Disposable TrueNAS system; a reachable LDAP server as
described above; Terraform CLI >= 1.11.0 (`credential.bindpw` is
WriteOnly).
**Preconditions:**
- `midclt call directoryservices.status` reports disabled/null, as in
  MT-IDENTITY-007.
- No nameserver change is needed for this case — address the LDAP server by
  IP or by a name your box's existing resolver can already reach.
- Know one existing user entry under the LDAP directory (name + uid) to
  confirm visibility once bound.

**Config:**
```hcl
variable "ldap_server_url" {
  type = string # e.g. "ldaps://ldap.example.internal" or "ldap://..."
}
variable "ldap_basedn" {
  type = string # e.g. "dc=example,dc=internal"
}
variable "ldap_binddn" {
  type = string # e.g. "cn=admin,dc=example,dc=internal"
}
variable "ldap_bindpw" {
  type      = string
  sensitive = true
}

resource "truenas_directoryservices" "test" {
  service_type = "LDAP"
  enable       = true
  timeout      = 10

  credential = {
    credential_type = "LDAP_PLAIN"
    binddn          = var.ldap_binddn
    bindpw          = var.ldap_bindpw
  }

  configuration_ldap = {
    server_urls           = [var.ldap_server_url]
    basedn                = var.ldap_basedn
    starttls               = false # set true only for a "ldap://" URL that needs StartTLS
    validate_certificates  = false # relax only if the LDAP server uses a self-signed cert
  }
}
```

**Steps:**
1. Save as `case-008.tf` with tfvars populated, `terraform apply`.
2. Poll `midclt call directoryservices.status` until `"HEALTHY"`, as in
   MT-IDENTITY-007 step 3.
3. Verify via UI (`Credentials > Directory Services`, LDAP tab shows bound,
   base DN matches) and via API: `midclt call directoryservices.config` —
   `service_type: "LDAP"`, `enable: true`,
   `configuration.server_urls`/`.basedn` match.
4. Confirm the known LDAP user is now visible through TrueNAS's account
   layer: `midclt call user.query '[["username","=","<known ldap
   username>"]]'`. Confirm it returns one result, `uid` matches what
   you expect, and `local: false` (proving it's LDAP-provided, not a local
   account).
5. Update in place: `timeout = 20`, `terraform apply`. Confirm no rebind is
   triggered (status stays `"HEALTHY"` throughout, no interruption to the
   step-4 user lookup).
6. `terraform destroy`.
7. Post-destroy verify: `midclt call directoryservices.config` shows
   `enable: false`, `service_type` still `"LDAP"` — the bind configuration
   is left in place, only disabled (there is no "leave" concept for a plain
   LDAP bind to begin with).

**Expected results:** Bind succeeds and reaches `HEALTHY`; the seeded LDAP
user resolves through `user.query` while bound; `timeout`-only update does
not rebind; destroy disables locally without discarding the bind
configuration.
**Cleanup:** Remove `case-008.tf`. No nameserver restore needed for this
case.

**MT-IDENTITY-009 — FreeIPA join, health check, update, destroy=disable**

**Resource:** `truenas_directoryservices`
**Type:** Functional — full join lifecycle (disruptive, disposable box only)
**Environment:** Disposable TrueNAS system; a reachable FreeIPA server
as described above; Terraform CLI >= 1.11.0 (`credential.password` is
WriteOnly).
**Preconditions:**
- `midclt call directoryservices.status` reports disabled/null, as in
  MT-IDENTITY-007.
- Record the box's current DNS nameserver setting so it can be restored
  (FreeIPA join, like AD, relies on DNS).

**Config:**
```hcl
variable "ipa_target_server" {
  type = string # e.g. "ipa.example.internal"
}
variable "ipa_domain" {
  type = string # e.g. "ipa.internal"
}
variable "ipa_hostname" {
  type    = string
  default = "tn-manual-test"
}
variable "ipa_password" {
  type      = string
  sensitive = true
}

locals {
  # FreeIPA derives its base DN from the domain, one "dc=" RDN per label —
  # e.g. "ipa.internal" -> "dc=ipa,dc=internal".
  ipa_basedn = join(",", [for l in split(".", var.ipa_domain) : "dc=${l}"])
}

resource "truenas_directoryservices" "test" {
  service_type = "IPA"
  enable       = true
  timeout      = 10

  credential = {
    credential_type = "KERBEROS_USER"
    username        = "admin"
    password        = var.ipa_password
  }

  configuration_ipa = {
    target_server         = var.ipa_target_server
    hostname               = var.ipa_hostname
    domain                 = var.ipa_domain
    basedn                 = local.ipa_basedn
    validate_certificates  = false
  }
}
```

**Steps:**
1. Point the box's nameserver at the FreeIPA server (it runs its own DNS for
   its zone): UI `Network > Global Configuration > Nameserver 1`, or `midclt
   call network.configuration.update '{"nameserver1": "<ipa-server-ip>"}'`.
2. Save as `case-009.tf` with tfvars populated, `terraform apply`.
3. Poll `midclt call directoryservices.status` until `"HEALTHY"`, as in
   MT-IDENTITY-007 step 3.
4. Verify via UI (`Credentials > Directory Services`, IPA tab shows joined)
   and via API: `midclt call directoryservices.config` — `service_type:
   "IPA"`, `enable: true`, `configuration.target_server`/`.hostname`/
   `.domain`/`.basedn` match.
5. Update in place: `timeout = 20`, `terraform apply`. Confirm no rejoin is
   triggered (status stays `"HEALTHY"`).
6. `terraform destroy`.
7. Post-destroy verify: `midclt call directoryservices.config` shows
   `enable: false`, `service_type` still `"IPA"` — the join is left in
   place, only disabled.
8. Restore the box's original nameserver (as recorded in Preconditions).

**Expected results:** Join succeeds and reaches `HEALTHY`; `timeout`-only
update does not rejoin; destroy disables locally without leaving the realm
or removing the host entry from IPA.
**Cleanup:** Remove `case-009.tf`; always restore the nameserver (step 8)
even if earlier steps failed. If switching directly between this case and
MT-IDENTITY-008/007 on the same box, expect the provider's stale-service-type
reset logic to run automatically on the next apply — it clears the previous
service's leftover `kerberos_realm`/`credential` before applying the new
one; no manual intervention is needed for that specifically.

### 7. truenas_kerberos_config

This is a singleton, system-wide resource (the free-form `[appdefaults]`/
`[libdefaults]` additions to `krb5.conf`): one Kerberos configuration
per TrueNAS system. `create`/`update` call `kerberos.update`; `destroy` only
removes the resource from Terraform state and leaves the box's configuration
untouched (with a warning diagnostic saying so).

**MT-IDENTITY-010 — Safe appdefaults_aux set and restore**

**Resource:** `truenas_kerberos_config`
**Type:** Functional — singleton set-and-restore
**Environment:** Any TrueNAS system; local admin credentials.
**Preconditions:** Read and record the box's current `appdefaults_aux`
value — this test restores it.

**Config:**
```hcl
resource "truenas_kerberos_config" "test" {
  appdefaults_aux = "no_addresses = true" # a real, recognized MIT krb5.conf key
}
```

**Steps:**
1. Read current values: `midclt call kerberos.config`. Record
   `appdefaults_aux` (call it `ORIG`).
2. If `ORIG` already equals `"no_addresses = true"`, use `"no_addresses =
   false"` instead as the test marker throughout this case.
3. Save as `case-010.tf`, `terraform apply`.
4. Verify: `midclt call kerberos.config` shows `appdefaults_aux` equal to
   the marker string.
5. Restore: edit `case-010.tf` back to `appdefaults_aux = "<ORIG>"`,
   `terraform apply`.
6. Verify: `midclt call kerberos.config` shows `appdefaults_aux` back to
   `ORIG`.
7. Import: `terraform state rm truenas_kerberos_config.test`, `terraform
   import truenas_kerberos_config.test kerberos_config`, `terraform plan` —
   expect no differences.
8. `terraform destroy`.
9. Post-destroy verify: `midclt call kerberos.config` is unchanged — destroy
   made no API call, only removed the resource from Terraform state.

**Expected results:** `appdefaults_aux` sets and restores correctly through
`kerberos.update`; destroy is a state-only no-op with a warning diagnostic.
**Cleanup:** If a step fails before restore, run `midclt call
kerberos.update '{"appdefaults_aux": "<ORIG>"}'` directly, then remove
`case-010.tf`.

**MT-IDENTITY-011 — [CAUTION] Malformed appdefaults_aux line crashes the middleware**

**Resource:** `truenas_kerberos_config`
**Type:** Negative / regression caution — documents a known middleware
defect, not a provider bug
**Environment:** Any TrueNAS system; local admin credentials. Prefer a
disposable/test box for this one, since a failed `kerberos.update` job can
leave `terraform apply` reporting an error while the box's actual state is
uncertain until you check it directly.
**Preconditions:** Read and record the box's current `appdefaults_aux`
value, as in MT-IDENTITY-010.

**Config:**
```hcl
resource "truenas_kerberos_config" "test" {
  appdefaults_aux = "# just a comment, not a key = value line"
}
```

**Steps:**
1. Save as `case-011.tf`, `terraform apply`.
2. **Expect this to fail.** `kerberos.update` parses `appdefaults_aux`
   server-side line by line splitting on `=`; a line that doesn't contain
   `=` at all (like the comment above) makes the server-side parser crash
   with a raw `list index out of range` `APIError` rather than a clean
   validation error — Terraform surfaces this as an apply-time error, not a
   normal plan-time validation diagnostic.
3. Immediately check the box's actual state directly (do not trust
   Terraform's local state after a failed apply here): `midclt call
   kerberos.config`. Confirm whether `appdefaults_aux` was left unchanged
   from the value recorded in Preconditions, or partially applied — record
   whichever it is.
4. As a contrasting check, try a syntactically-valid-shaped but *unknown*
   key, e.g. `appdefaults_aux = "not_a_real_key = true"`. This should be
   **cleanly rejected** (`EINVAL`-style validation error, not a crash) —
   confirming the crash in step 2 is specific to lines that don't even split
   on `=`, not to unrecognized-but-shaped input.
5. Restore: set `appdefaults_aux` back to the value recorded in
   Preconditions and `terraform apply` (or, if Terraform's own state is now
   unreliable after the failed apply, restore directly via `midclt call
   kerberos.update '{"appdefaults_aux": "<ORIG>"}'`).
6. Verify: `midclt call kerberos.config` shows `appdefaults_aux` back to its
   original value.
7. Remove `case-011.tf`.

**Expected results:** A malformed (non-`key=value`) `appdefaults_aux` line
crashes `kerberos.update` server-side with an unhelpful `APIError`, not a
clean validation message — this is a known TrueNAS middleware defect, not
something this provider can validate away client-side (the schema has no
way to know which strings are valid krb5.conf lines). A recognized-but-wrong
key is, by contrast, cleanly rejected. **Caution for anyone scripting
against `truenas_kerberos_config.appdefaults_aux`/`libdefaults_aux`:
validate that any generated value is shaped as `key = value` before
applying, and be prepared for an apply-time crash (not a plan-time
validation error) if it isn't.**
**Cleanup:** Confirm `appdefaults_aux` is restored to its original value
(step 5/6) before ending the session.

### 8. truenas_kerberos_realm

**MT-IDENTITY-012 — Synthetic Kerberos realm full lifecycle**

**Resource:** `truenas_kerberos_realm`
**Type:** Functional — full lifecycle
**Environment:** Any TrueNAS system; local admin credentials. No real
KDC is required — `kerberos.realm.create` does not validate KDC reachability
at creation time, so a synthetic realm pointed at an address that will never
answer is safe to use here.
**Preconditions:** No existing realm named `TF-ACC-MANUAL-01.LAN` (`midclt
call kerberos.realm.query '[["realm","=","TF-ACC-MANUAL-01.LAN"]]'` returns
`[]`).

**Config:**
```hcl
resource "truenas_kerberos_realm" "test" {
  realm        = "TF-ACC-MANUAL-01.LAN"
  kdc          = ["192.0.2.88"] # RFC 5737 TEST-NET-1: reserved, never answers
  admin_server = []
}
```

**Steps:**
1. Save as `case-012.tf`, `terraform apply`. Confirm it completes quickly
   (a few hundred milliseconds to a couple of seconds) — proof
   `kerberos.realm.create` isn't attempting to contact `192.0.2.88`.
2. Verify via UI (`Directory Services > Advanced Settings` — or `System
   Settings > Advanced Settings` depending on release — `Kerberos Realms`
   card, showing `TF-ACC-MANUAL-01.LAN` with kdc `192.0.2.88`) and via API:
   `midclt call kerberos.realm.query
   '[["realm","=","TF-ACC-MANUAL-01.LAN"]]'` — `kdc: ["192.0.2.88"]`,
   `admin_server: []`, `primary_kdc: null`.
3. Update in place: `admin_server = ["192.0.2.89"]`, `terraform apply`.
   Confirm no replacement.
4. Verify: `midclt call kerberos.realm.query
   '[["realm","=","TF-ACC-MANUAL-01.LAN"]]'` shows `admin_server:
   ["192.0.2.89"]`.
5. Import: get the numeric id, `terraform state rm
   truenas_kerberos_realm.test`, `terraform import
   truenas_kerberos_realm.test <id>`, `terraform plan` — expect no
   differences.
6. `terraform destroy`.
7. Post-destroy verify: `midclt call kerberos.realm.query
   '[["realm","=","TF-ACC-MANUAL-01.LAN"]]'` returns `[]`.

**Expected results:** A synthetic realm with an unreachable KDC creates
without delay or error (confirming realm creation doesn't validate
reachability), updates `admin_server` in place, imports cleanly, and is
fully removed on destroy.
**Cleanup:** Remove `TF-ACC-MANUAL-01.LAN` directly via `midclt call
kerberos.realm.query` + `midclt call kerberos.realm.delete <id>` if a step
fails, then remove `case-012.tf`.

### 9. truenas_kerberos_keytab

**MT-IDENTITY-013 — Real keytab create, rename, import, destroy**

**Resource:** `truenas_kerberos_keytab`
**Type:** Functional — full lifecycle (requires a real domain controller)
**Environment:** Any TrueNAS system; local admin credentials; a
reachable Active Directory (or Samba AD) domain controller you can export a
keytab from.
**Preconditions:**
- On the domain controller, export a keytab for a real principal, e.g.:
  ```
  samba-tool domain exportkeytab /tmp/tfacc.keytab --principal=Administrator@YOURDOMAIN.LAN
  base64 -w0 /tmp/tfacc.keytab
  ```
  Copy the resulting base64 text — this is the value for the `file`
  variable below. Treat it as a secret; it decodes to real Kerberos key
  material.
- No existing keytab entry named `tf-acc-manual-keytab-01` (`midclt call
  kerberos.keytab.query
  '[["name","=","tf-acc-manual-keytab-01"]]'` returns `[]`).

**Config:**
```hcl
variable "keytab_file_b64" {
  type      = string
  sensitive = true
}

resource "truenas_kerberos_keytab" "test" {
  name = "tf-acc-manual-keytab-01"
  file = var.keytab_file_b64
}
```

**Steps:**
1. Add `keytab_file_b64` to `variables.tf`/`terraform.tfvars` with the
   base64 text from Preconditions.
2. Save the resource block as `case-013.tf`, `terraform apply`.
3. Verify via UI (`Directory Services > Advanced Settings` — or `System
   Settings > Advanced Settings` — `Kerberos Keytabs` card, showing
   `tf-acc-manual-keytab-01`) and via API: `midclt call kerberos.keytab.query
   '[["name","=","tf-acc-manual-keytab-01"]]'` — confirm `file` equals the
   base64 text you supplied, byte-for-byte (`file` is Sensitive but not
   write-only for this resource — TrueNAS returns it intact on every read,
   unlike `truenas_user.password`).
4. Rename in place: `name = "tf-acc-manual-keytab-01-renamed"`, `terraform
   apply`. Confirm no replacement and `file` unchanged.
5. Import: get the numeric id, `terraform state rm
   truenas_kerberos_keytab.test`, `terraform import
   truenas_kerberos_keytab.test <id>`, `terraform plan` — expect no
   differences, including on `file` (it does round-trip on import, unlike
   `truenas_api_key.key`).
6. `terraform destroy`.
7. Post-destroy verify: `midclt call kerberos.keytab.query
   '[["id","=",<id>]]'` returns `[]`.

**Expected results:** A keytab exported from a real domain controller
creates, round-trips its `file` content on every read (including
after a name-only update and after import), renames in place, and is fully
removed on destroy.
**Cleanup:** Remove `tf-acc-manual-keytab-01`/`-renamed` directly via
`midclt call kerberos.keytab.query` + `midclt call kerberos.keytab.delete
<id>` if a step fails, then remove `case-013.tf` and delete
`/tmp/tfacc.keytab` from the domain controller.

---

## Certificates, Keychain, Replication, and Tasks

These are **manual** test cases: a human QA engineer runs `terraform` (and, for
verification, `midclt` on the TrueNAS box and/or the TrueNAS web UI) by hand.
They are not part of the Go acceptance-test harness (`internal/resources/*/acceptance_test.go`),
though several mirror what that harness automates.

### Conventions used throughout this section

- **Test box**: examples below use `192.168.1.68` as the TrueNAS test
  box's address. Substitute your own test box's hostname/IP everywhere you
  see it if it differs — do not use a shared/production box.
- **Provider block**: every `Config:` is a complete, standalone `.tf` file.
  Most repeat this boilerplate:

  ```hcl
  terraform {
    required_providers {
      truenas = {
        source  = "truenas/truenas"
        version = "~> 0.1"
      }
    }
  }

  variable "truenas_api_key" {
    type      = string
    sensitive = true
  }

  provider "truenas" {
    endpoint = "wss://192.168.1.68/api/current"
    api_key  = var.truenas_api_key
    insecure = true
  }
  ```

  Set the key once per shell session: `export TF_VAR_truenas_api_key="<your API key>"`.
  Get a key from the TrueNAS UI (**user icon → API Keys → Add**) or
  `midclt call api_key.create '{"name": "tf-manual-tests", "username": "root"}'`.
- **Secrets/PEM material**: generated with `openssl`/`ssh-keygen` into local
  files, then loaded into Terraform variables via shell `TF_VAR_*` environment
  variables (`export TF_VAR_foo="$(cat file)"`) — never pasted into `.tf`
  files or committed to disk.
- **Naming**: all TrueNAS-side objects use `tf-acc-manual-<resource>-<NNN>`
  names, matching this doc's `MT-TASKS-NNN` numbering, so leftovers are easy
  to spot and clean up.
- **Verification**: every case's Steps use `midclt call <method>.query
  '[["<field>","=","<value>"]]'` run over SSH on the TrueNAS box (or via the
  UI) — not the Go test harness — as the ground truth for what the API
  actually persisted, independent of what Terraform state claims.
- **`terraform destroy`** at the end of each case is assumed to run
  `terraform destroy -auto-approve` unless noted otherwise; the Cleanup field
  calls out anything destroy does not handle (e.g. singleton resources,
  files left on disk).

---

### 1. truenas_certificate

`certificate.create`/`update`/`delete` are all asynchronous TrueNAS **jobs** —
`terraform apply`/`destroy` blocks until the job finishes (usually a few
seconds), but if something looks stuck, check **System → Advanced → Task
Manager** in the UI or `midclt call core.get_jobs '[["method","=","certificate.create"]]'`.

**Never** point any of these test cases at certificate id 1 or any
certificate currently serving the TrueNAS UI/API — `certificate.create`
always creates a brand-new certificate row; there is no way to "adopt" the
live one short of `terraform import`, which none of these cases do.

**MT-TASKS-001 — Import a self-signed certificate (CERTIFICATE_CREATE_IMPORTED)**

**Resource:** truenas_certificate
**Type:** Positive — full CRUD (create, rename update, import, destroy)
**Environment:** TrueNAS 25.10+ test box at 192.168.1.68; Terraform CLI
≥ 1.5; `openssl` on the workstation running Terraform.
**Preconditions:**

Generate a self-signed RSA-2048 certificate + private key with openssl:

```
openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout tf-acc-manual-cert-001.key \
  -out   tf-acc-manual-cert-001.crt \
  -days 365 \
  -subj "/CN=tf-acc-manual-001.example.com" \
  -addext "subjectAltName=DNS:tf-acc-manual-001.example.com"
```

Load the PEM files into Terraform variables:

```
export TF_VAR_certificate_pem="$(cat tf-acc-manual-cert-001.crt)"
export TF_VAR_privatekey_pem="$(cat tf-acc-manual-cert-001.key)"
```

**Config:**

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 0.1"
    }
  }
}

variable "truenas_api_key" {
  type      = string
  sensitive = true
}
variable "certificate_pem" {
  type      = string
  sensitive = true
}
variable "privatekey_pem" {
  type      = string
  sensitive = true
}

provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = var.truenas_api_key
  insecure = true
}

resource "truenas_certificate" "test" {
  name        = "tf-acc-manual-cert-001"
  create_type = "CERTIFICATE_CREATE_IMPORTED"
  certificate = var.certificate_pem
  privatekey  = var.privatekey_pem
}
```

**Steps:**

1. `terraform init`
2. `terraform apply` (type `yes`). Wait for the `certificate.create` job to finish.
3. Verify in the UI: **Credentials → Certificates** shows `tf-acc-manual-cert-001`
   with type "Certificate", not expired.
4. Verify via API: `midclt call certificate.query '[["name","=","tf-acc-manual-cert-001"]]'`
   — confirm `"certificate"` matches the contents of `tf-acc-manual-cert-001.crt`
   byte-for-byte, `"privatekey"` matches `tf-acc-manual-cert-001.key`
   byte-for-byte (**not** masked as `"********"` — that masking only ever
   appears in the raw job result, never in `certificate.query`), `"key_type":
   "RSA"`, `"key_length": 2048`, `"expired": false`, `"fingerprint"` is
   non-null.
5. Check `terraform show` / `terraform state show truenas_certificate.test`:
   confirm `privatekey` and `certificate` are populated (not `(sensitive
   value)`-blank/null) and `certificate_path`/`privatekey_path` are non-null
   filesystem paths under `/etc/certificates/`.
6. Edit the config: change `name` to `tf-acc-manual-cert-001-renamed`. Run
   `terraform plan` — confirm the plan shows an **in-place update** (not a
   replace) for `name` only.
7. `terraform apply`. Verify via `midclt call certificate.query
   '[["id","=","<id from state>"]]'` that `"name"` is now
   `tf-acc-manual-cert-001-renamed` and `"certificate_path"` changed to embed
   the new name (e.g. `/etc/certificates/tf-acc-manual-cert-001-renamed.crt`).
8. `terraform import truenas_certificate.import_check <id>` into a scratch
   config containing only a bare `resource "truenas_certificate"
   "import_check" {}` block, then `terraform plan` — confirm no diff other
   than possibly `create_type` (not recoverable on import — it is never
   returned by `certificate.query`) and `passphrase` (also never echoed
   back). Everything else, including `privatekey`, must read back identical.
9. `terraform destroy` (on the original config). Wait for the
   `certificate.delete` job to finish.
10. Post-destroy verify: `midclt call certificate.query
    '[["name","=","tf-acc-manual-cert-001-renamed"]]'` returns `[]`.

**Expected results:** Certificate imports cleanly with both PEM blobs
round-tripping; rename is an in-place update, not a replace; import
recovers every field except `create_type`/`passphrase`; destroy removes the
certificate from the box.

**Cleanup:** `rm tf-acc-manual-cert-001.key tf-acc-manual-cert-001.crt`;
remove the scratch import-check state/config directory.

---

**MT-TASKS-002 — Generate a CSR on-box (CERTIFICATE_CREATE_CSR, RSA)**

**Resource:** truenas_certificate
**Type:** Positive — create, verify, destroy (no update: exercises the "TrueNAS generates the key pair" path)
**Environment:** Same as MT-TASKS-001. No openssl needed — the key pair and CSR are generated server-side.
**Preconditions:** None.
**Config:**

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 0.1"
    }
  }
}

variable "truenas_api_key" {
  type      = string
  sensitive = true
}

provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = var.truenas_api_key
  insecure = true
}

resource "truenas_certificate" "test" {
  name        = "tf-acc-manual-cert-002"
  create_type = "CERTIFICATE_CREATE_CSR"
  key_type    = "RSA"
  key_length  = 2048
  common      = "tf-acc-manual-002.example.com"
  san         = ["tf-acc-manual-002.example.com"]
}
```

**Steps:**

1. `terraform init && terraform apply` (type `yes`).
2. Verify via `midclt call certificate.query '[["name","=","tf-acc-manual-cert-002"]]'`:
   `"cert_type"` should reflect a CSR (not yet a signed certificate — `"certificate"`
   is null), `"CSR"` is a non-empty PEM block beginning
   `-----BEGIN CERTIFICATE REQUEST-----`, `"privatekey"` is populated
   (server-generated), `"csr_path"` is a non-null path.
3. `terraform state show truenas_certificate.test` — confirm `csr` in state
   matches the API's `"CSR"` value and `san` reads back as
   `["tf-acc-manual-002.example.com"]` (the provider strips the API's
   `"DNS:"` prefix automatically — if you see `"DNS:tf-acc-manual-..."` in
   state, that is a bug).
4. In the UI, confirm **Credentials → Certificate Signing Requests** lists
   `tf-acc-manual-cert-002`.
5. `terraform destroy`.
6. Post-destroy verify: `midclt call certificate.query
   '[["name","=","tf-acc-manual-cert-002"]]'` returns `[]`.

**Expected results:** CSR + fresh key pair generated on-box; `san` read back
without the `DNS:` prefix; certificate/CSR/key fields all populated
correctly; clean destroy.

**Cleanup:** None (no local files created).

---

**MT-TASKS-003 — Import an externally-generated CSR (CERTIFICATE_CREATE_IMPORTED_CSR)**

**Resource:** truenas_certificate
**Type:** Positive — create, verify, destroy
**Environment:** Same as MT-TASKS-001; `openssl` required.
**Preconditions:**

Generate a CSR + private key with openssl (do **not** self-sign — this is
importing the request, not a certificate):

```
openssl req -newkey rsa:2048 -nodes \
  -keyout tf-acc-manual-cert-003.key \
  -out   tf-acc-manual-cert-003.csr \
  -subj "/CN=tf-acc-manual-003.example.com" \
  -addext "subjectAltName=DNS:tf-acc-manual-003.example.com"
```

```
export TF_VAR_csr_pem="$(cat tf-acc-manual-cert-003.csr)"
export TF_VAR_privatekey_pem="$(cat tf-acc-manual-cert-003.key)"
```

**Config:**

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 0.1"
    }
  }
}

variable "truenas_api_key" {
  type      = string
  sensitive = true
}
variable "csr_pem" {
  type      = string
  sensitive = true
}
variable "privatekey_pem" {
  type      = string
  sensitive = true
}

provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = var.truenas_api_key
  insecure = true
}

resource "truenas_certificate" "test" {
  name        = "tf-acc-manual-cert-003"
  create_type = "CERTIFICATE_CREATE_IMPORTED_CSR"
  csr         = var.csr_pem
  privatekey  = var.privatekey_pem
}
```

**Steps:**

1. `terraform init && terraform apply` (type `yes`).
2. Verify via `midclt call certificate.query '[["name","=","tf-acc-manual-cert-003"]]'`:
   `"CSR"` matches `tf-acc-manual-cert-003.csr` byte-for-byte, `"privatekey"`
   matches `tf-acc-manual-cert-003.key` byte-for-byte, `"certificate"` is
   null (a CSR import is not a signed certificate).
3. `terraform import truenas_certificate.import_check <id>` into a scratch
   config; `terraform plan` — confirm clean import aside from
   `create_type`/`passphrase`.
4. `terraform destroy`.
5. Post-destroy verify: query by name returns `[]`.

**Expected results:** Externally-generated CSR + key import round-trips
cleanly; clean destroy.

**Cleanup:** `rm tf-acc-manual-cert-003.key tf-acc-manual-cert-003.csr`;
remove scratch import-check config/state.

---

**MT-TASKS-004 — CSR generation with an EC key (key_type=EC, default curve)**

**Resource:** truenas_certificate
**Type:** Positive — coverage of the EC key_type branch (no key_length required; ec_curve defaults server-side)
**Environment:** Same as MT-TASKS-001.
**Preconditions:** None.
**Config:**

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 0.1"
    }
  }
}

variable "truenas_api_key" {
  type      = string
  sensitive = true
}

provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = var.truenas_api_key
  insecure = true
}

resource "truenas_certificate" "test" {
  name        = "tf-acc-manual-cert-004"
  create_type = "CERTIFICATE_CREATE_CSR"
  key_type    = "EC"
  # ec_curve intentionally left unset — the API defaults to SECP384R1.
  common      = "tf-acc-manual-004.example.com"
  san         = ["tf-acc-manual-004.example.com"]
}
```

**Steps:**

1. `terraform init && terraform apply` (type `yes`) — note no `key_length`
   is required/rejected for `key_type = "EC"` (unlike RSA).
2. Verify via `midclt call certificate.query '[["name","=","tf-acc-manual-cert-004"]]'`:
   `"key_type": "EC"`, `"key_length"` reflects the curve's bit size (384 for
   the default SECP384R1), `"CSR"` is a populated PEM.
3. `terraform state show truenas_certificate.test` — confirm `ec_curve`
   reads back as whatever you set in config (null here, since it was left
   unset) — this field is deliberately **not** read back from the API even
   though the server did pick SECP384R1 internally; that is expected
   behavior, not a bug.
4. `terraform destroy`.
5. Post-destroy verify: query by name returns `[]`.

**Expected results:** EC CSR generation succeeds without `key_length`;
`key_length` in state reflects the curve's bit size; `ec_curve` stays null
in state (write-only-by-convention).

**Cleanup:** None.

---

**MT-TASKS-005 — Preflight validation errors (negative)**

**Resource:** truenas_certificate
**Type:** Negative — client-side (Terraform-level) preflight validation, no API call made
**Environment:** Same as MT-TASKS-001.
**Preconditions:** None.
**Config (attempt A — IMPORTED missing required fields):**

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 0.1"
    }
  }
}

variable "truenas_api_key" {
  type      = string
  sensitive = true
}

provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = var.truenas_api_key
  insecure = true
}

resource "truenas_certificate" "bad_import" {
  name        = "tf-acc-manual-cert-005a"
  create_type = "CERTIFICATE_CREATE_IMPORTED"
  # certificate and privatekey deliberately omitted
}
```

**Config (attempt B — renew_days set on a non-ACME create_type):**

```hcl
# ... same terraform/provider/variable blocks as attempt A ...

resource "truenas_certificate" "bad_renew" {
  name        = "tf-acc-manual-cert-005b"
  create_type = "CERTIFICATE_CREATE_CSR"
  key_type    = "RSA"
  key_length  = 2048
  san         = ["tf-acc-manual-005b.example.com"]
  renew_days  = 5
}
```

**Steps:**

1. Attempt A: `terraform init && terraform apply`. Confirm Terraform fails
   at plan/apply time with two clear errors: `"certificate" is required.`
   and `"privatekey" is required.` — **no** `certificate.create` job is ever
   submitted (verify with `midclt call certificate.query
   '[["name","=","tf-acc-manual-cert-005a"]]'` → `[]`, both before and after
   the failed apply).
2. Attempt B: replace the config with attempt B's, `terraform apply`.
   Confirm Terraform fails with `"renew_days" is only supported for
   create_type CERTIFICATE_CREATE_ACME.` and no certificate is created
   (`midclt call certificate.query '[["name","=","tf-acc-manual-cert-005b"]]'`
   → `[]`).

**Expected results:** Both misconfigurations are caught client-side with a
clear, specific error message before any API call is made; the TrueNAS box
never has a `tf-acc-manual-cert-005a`/`-005b` object created.

**Cleanup:** None (nothing was ever created).

---

**MT-TASKS-006 — CERTIFICATE_CREATE_ACME (documented-skip)**

**Resource:** truenas_certificate
**Type:** Manual-only, **not runnable** in a normal test environment — documented skip
**Environment:** N/A without a real ACME account + a domain you can complete
a DNS-01 challenge for.
**Preconditions:** N/A.
**Config:** N/A.
**Steps:** N/A — do not attempt this against a disposable/throwaway test box.

**Expected results / rationale:** `create_type = "CERTIFICATE_CREATE_ACME"`
is schema-and-preflight only in this provider version: `acme_directory_uri`/
`csr_id`/`tos`/`dns_mapping` are accepted and forwarded to
`certificate.create`, but live ACME issuance (DNS challenge orchestration
via a `truenas_acme_dns_authenticator`, renewal polling) is not implemented
or exercised by this provider — treat it as unverified beyond "the API
accepts the payload shape." If you must validate this path, you need: (1) a
real domain you control DNS for, (2) a working
`truenas_acme_dns_authenticator` with genuine (not dummy) credentials for
that domain's DNS provider (see section 2), (3) an existing
`truenas_certificate` with `create_type = CERTIFICATE_CREATE_CSR` whose id
feeds `csr_id`, (4) `tos = true`, and (5) `dns_mapping = { "<domain>" =
<authenticator id> }`. Expect the `certificate.create` job to run for
several minutes while it completes the DNS-01 challenge with the real
provider — never point this at a shared/production DNS zone.

**Cleanup:** N/A.

---

### 2. truenas_acme_dns_authenticator

`acme.dns.authenticator.create` does **not** validate credentials against
the real DNS provider for any variant except `shell` (which does a local
filesystem check that `script` exists under a pool mount point) — so dummy
credential values are fine for CRUD testing every other variant.

**MT-TASKS-007 — cloudflare variant, full CRUD**

**Resource:** truenas_acme_dns_authenticator
**Type:** Positive — full CRUD (create, update, import, destroy)
**Environment:** Same as MT-TASKS-001. No real Cloudflare account needed.
**Preconditions:** None.
**Config:**

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 0.1"
    }
  }
}

variable "truenas_api_key" {
  type      = string
  sensitive = true
}

provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = var.truenas_api_key
  insecure = true
}

resource "truenas_acme_dns_authenticator" "test" {
  name = "tf-acc-manual-acmedns-007"
  attributes = jsonencode({
    authenticator = "cloudflare"
    api_token     = "tf-acc-manual-dummy-token-1"
  })
}
```

**Steps:**

1. `terraform init && terraform apply` (type `yes`).
2. Verify via `midclt call acme.dns.authenticator.query
   '[["name","=","tf-acc-manual-acmedns-007"]]'` — confirm
   `"attributes"` is `{"authenticator": "cloudflare", "api_token":
   "tf-acc-manual-dummy-token-1", ...}` in **cleartext** (this is expected:
   the API does not mask DNS authenticator credentials on read-back, unlike
   `truenas_certificate`'s private key job-result masking).
3. In the UI, confirm **Credentials → Certificates → ACME DNS
   Authenticators** lists `tf-acc-manual-acmedns-007`.
4. Edit config: change `api_token` to `"tf-acc-manual-dummy-token-2"`. Run
   `terraform plan` — confirm an in-place update (not a replace).
   `terraform apply`.
5. Verify via `midclt call acme.dns.authenticator.query
   '[["name","=","tf-acc-manual-acmedns-007"]]'` that `api_token` is now
   `tf-acc-manual-dummy-token-2` — confirm the whole `attributes` object was
   resent (a full replace under the hood, not a partial patch).
6. `terraform import truenas_acme_dns_authenticator.import_check <id>` into
   a scratch config; `terraform plan` — confirm no diff except `attributes`
   itself (expected: `ImportStateVerifyIgnore`-equivalent — the raw JSON
   string ordering may differ even though the underlying values are
   identical; compare semantically, e.g. with `jq -S`, not byte-for-byte).
7. `terraform destroy`.
8. Post-destroy verify: query by name returns `[]`.

**Expected results:** Credentials stored and read back in cleartext (not
masked); update is a full replace under the hood; clean destroy.

**Cleanup:** Remove scratch import-check config/state.

---

**MT-TASKS-008 — route53 variant (different discriminated schema shape)**

**Resource:** truenas_acme_dns_authenticator
**Type:** Positive — coverage of a second `authenticator` variant with entirely different required fields
**Environment:** Same as MT-TASKS-001. No real AWS account needed.
**Preconditions:** None.
**Config:**

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 0.1"
    }
  }
}

variable "truenas_api_key" {
  type      = string
  sensitive = true
}

provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = var.truenas_api_key
  insecure = true
}

resource "truenas_acme_dns_authenticator" "test" {
  name = "tf-acc-manual-acmedns-008"
  attributes = jsonencode({
    authenticator     = "route53"
    access_key_id     = "TFACCMANUALDUMMY"
    secret_access_key = "tf-acc-manual-dummy-secret"
  })
}
```

**Steps:**

1. `terraform init && terraform apply` (type `yes`).
2. Verify via `midclt call acme.dns.authenticator.query
   '[["name","=","tf-acc-manual-acmedns-008"]]'` — confirm `"attributes"`
   shows `"authenticator": "route53"` plus `access_key_id`/
   `secret_access_key` in cleartext, and that create succeeded despite the
   fabricated AWS credentials (confirms route53, like cloudflare, is not
   validated against the real provider at create time).
3. `terraform destroy`.
4. Post-destroy verify: query by name returns `[]`.

**Expected results:** A structurally different variant (route53's
`access_key_id`/`secret_access_key` vs. cloudflare's `api_token`) is
accepted through the same free-form `attributes` JSON attribute; no live
AWS validation occurs.

**Cleanup:** None.

---

**MT-TASKS-009 — Invalid attributes JSON (negative)**

**Resource:** truenas_acme_dns_authenticator
**Type:** Negative — client-side validation of the `attributes` JSON document
**Environment:** Same as MT-TASKS-001.
**Preconditions:** None.
**Config:**

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 0.1"
    }
  }
}

variable "truenas_api_key" {
  type      = string
  sensitive = true
}

provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = var.truenas_api_key
  insecure = true
}

resource "truenas_acme_dns_authenticator" "bad" {
  name = "tf-acc-manual-acmedns-009"
  attributes = jsonencode({
    api_token = "tf-acc-manual-dummy-token"
    # "authenticator" key deliberately omitted
  })
}
```

**Steps:**

1. `terraform init && terraform apply`.
2. Confirm Terraform fails at apply time with an error to the effect of
   `attributes JSON must contain an "authenticator" key (one of: cloudflare,
   digitalocean, OVH, route53, shell)` — **before** any
   `acme.dns.authenticator.create` call is made.
3. Verify: `midclt call acme.dns.authenticator.query
   '[["name","=","tf-acc-manual-acmedns-009"]]'` returns `[]`.

**Expected results:** Missing `authenticator` discriminator is caught
client-side with a clear message; nothing is created on the box.

**Cleanup:** None.

---

### 3. truenas_keychain_ssh_keypair

**MT-TASKS-010 — Server-generated key pair (generate=true)**

**Resource:** truenas_keychain_ssh_keypair
**Type:** Positive — full CRUD (create, rename update, import, destroy)
**Environment:** Same as MT-TASKS-001.
**Preconditions:** None — TrueNAS generates the RSA key pair server-side.
**Config:**

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 0.1"
    }
  }
}

variable "truenas_api_key" {
  type      = string
  sensitive = true
}

provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = var.truenas_api_key
  insecure = true
}

resource "truenas_keychain_ssh_keypair" "test" {
  name     = "tf-acc-manual-keypair-010"
  generate = true
}
```

**Steps:**

1. `terraform init && terraform apply` (type `yes`).
2. Verify via `midclt call keychaincredential.query
   '[["name","=","tf-acc-manual-keypair-010"]]'` — confirm `"type":
   "SSH_KEY_PAIR"`, `attributes.private_key` begins `-----BEGIN ... PRIVATE
   KEY-----`, `attributes.public_key` begins `ssh-rsa `.
3. `terraform state show truenas_keychain_ssh_keypair.test` — confirm
   `private_key`/`public_key` are populated in state (Sensitive but not
   WriteOnly — they persist and read back).
4. In the UI, confirm **Credentials → Backup Credentials → SSH Keypairs**
   lists `tf-acc-manual-keypair-010`.
5. Edit config: rename to `tf-acc-manual-keypair-010-renamed`.
   `terraform plan` — confirm in-place update, not replace.
   `terraform apply`.
6. Verify via `midclt call keychaincredential.query` by the new name that
   `private_key`/`public_key` are unchanged from step 2.
7. `terraform import truenas_keychain_ssh_keypair.import_check <id>` into a
   scratch config; `terraform plan` — confirm no diff except `generate`
   (reads back as null on import — it has no wire counterpart).
8. `terraform destroy`.
9. Post-destroy verify: query by name returns `[]`.

**Expected results:** Server generates a fresh RSA key pair; rename is
in-place; import recovers everything except `generate`; clean destroy.

**Cleanup:** Remove scratch import-check config/state.

---

**MT-TASKS-011 — Supplied private key (generate unset)**

**Resource:** truenas_keychain_ssh_keypair
**Type:** Positive — full CRUD with a caller-supplied key
**Environment:** Same as MT-TASKS-001; `ssh-keygen` required on the workstation.
**Preconditions:**

Generate an ed25519 key pair with ssh-keygen (no passphrase):

```
ssh-keygen -t ed25519 -N "" -C "tf-acc-manual-keypair-011" \
  -f ./tf-acc-manual-keypair-011
```

This produces `tf-acc-manual-keypair-011` (private, OpenSSH format) and
`tf-acc-manual-keypair-011.pub` (public, unused directly — TrueNAS derives
it server-side).

```
export TF_VAR_private_key="$(cat tf-acc-manual-keypair-011)"
```

**Config:**

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 0.1"
    }
  }
}

variable "truenas_api_key" {
  type      = string
  sensitive = true
}
variable "private_key" {
  type      = string
  sensitive = true
}

provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = var.truenas_api_key
  insecure = true
}

resource "truenas_keychain_ssh_keypair" "test" {
  name        = "tf-acc-manual-keypair-011"
  private_key = var.private_key
}
```

**Steps:**

1. `terraform init && terraform apply` (type `yes`).
2. Verify via `midclt call keychaincredential.query
   '[["name","=","tf-acc-manual-keypair-011"]]'` — confirm
   `attributes.private_key` matches `tf-acc-manual-keypair-011` byte-for-byte,
   and `attributes.public_key` was **derived automatically** by TrueNAS
   (compare its base64 body against `tf-acc-manual-keypair-011.pub`'s — the
   comment suffix may differ slightly in whitespace but the algorithm+key
   material must match).
3. `terraform state show truenas_keychain_ssh_keypair.test` — confirm
   `generate = false`.
4. Edit config: rename to `tf-acc-manual-keypair-011-renamed`.
   `terraform apply` — confirm in-place update; `private_key` unchanged.
5. `terraform import truenas_keychain_ssh_keypair.import_check <id>` into a
   scratch config; `terraform plan` — clean aside from `generate`.
6. `terraform destroy`.
7. Post-destroy verify: query by name returns `[]`.

**Expected results:** Supplied private key stored and read back;
public key auto-derived server-side; rename is in-place; clean destroy.

**Cleanup:** `rm tf-acc-manual-keypair-011 tf-acc-manual-keypair-011.pub`;
remove scratch import-check config/state.

---

### 4. truenas_keychain_ssh_connection

**MT-TASKS-012 — Loopback SSH connection to the test box itself**

**Resource:** truenas_keychain_ssh_connection (plus fixtures: truenas_dataset,
truenas_user, truenas_keychain_ssh_keypair)
**Type:** Positive — full CRUD (create, in-place update, import, destroy) over a real loopback SSH connection
**Environment:** Same as MT-TASKS-001; `ssh-keygen` required. The test box
must have SSH (port 22) enabled and reachable at its own address
(192.168.1.68).
**Preconditions:**

Generate an ed25519 key pair to authorize a **dedicated, disposable** login
user (never `root`/`truenas_admin`) on the box:

```
ssh-keygen -t ed25519 -N "" -C "tf-acc-manual-sshconn-012" \
  -f ./tf-acc-manual-sshconn-012
```

```
export TF_VAR_public_key="$(cat tf-acc-manual-sshconn-012.pub)"
export TF_VAR_private_key="$(cat tf-acc-manual-sshconn-012)"
```

Discover the box's own SSH host key(s), either via ssh-keyscan from the
workstation:

```
ssh-keyscan -T 5 -p 22 192.168.1.68 > tf-acc-manual-sshconn-012-hostkey.txt
```

or via TrueNAS's own helper method (run from the box, or via any admin
session):

```
midclt call keychaincredential.remote_ssh_host_key_scan \
  '{"host": "192.168.1.68", "port": 22}' > tf-acc-manual-sshconn-012-hostkey.txt
```

Either way, strip the leading `#`-comment lines ssh-keyscan adds (the
`midclt` output is already clean — a single JSON string with one host-key
line per algorithm, newline-separated) and load it:

```
export TF_VAR_remote_host_key="$(grep -v '^#' tf-acc-manual-sshconn-012-hostkey.txt)"
```

**Config:**

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 0.1"
    }
  }
}

variable "truenas_api_key" {
  type      = string
  sensitive = true
}
variable "public_key" {
  type = string
}
variable "private_key" {
  type      = string
  sensitive = true
}
variable "remote_host_key" {
  type = string
}

provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = var.truenas_api_key
  insecure = true
}

# Dedicated home dataset + throwaway login user for the loopback connection
# — never authorize this key against truenas_admin/root.
resource "truenas_dataset" "sshuser_home" {
  name = "tank/tf-acc-manual-sshconn-012-home"
}

resource "truenas_user" "sshuser" {
  username          = "tfaccmanualsshconn012"
  full_name         = "TF Manual Test SSH Connection User"
  password_disabled = true
  home              = "/mnt/${truenas_dataset.sshuser_home.name}"
  shell             = "/usr/bin/bash"
  group_create      = true
  sshpubkey         = trimspace(var.public_key)
}

resource "truenas_keychain_ssh_keypair" "test" {
  name        = "tf-acc-manual-sshconn-012-keypair"
  private_key = var.private_key
}

resource "truenas_keychain_ssh_connection" "test" {
  name            = "tf-acc-manual-sshconn-012"
  host            = "192.168.1.68"
  port            = 22
  username        = truenas_user.sshuser.username
  private_key_id  = truenas_keychain_ssh_keypair.test.id
  remote_host_key = var.remote_host_key
  connect_timeout = 10
}
```

**Steps:**

1. `terraform init && terraform apply` (type `yes`).
2. Verify via `midclt call keychaincredential.query
   '[["name","=","tf-acc-manual-sshconn-012"]]'` — confirm `"type":
   "SSH_CREDENTIALS"`, `attributes.host = "192.168.1.68"`,
   `attributes.port = 22`, `attributes.username = "tfaccmanualsshconn012"`,
   `attributes.private_key` equals the keypair fixture's numeric id (not key
   material), `attributes.connect_timeout = 10`.
3. Confirm the credential actually works: SSH in manually as a sanity check
   (`ssh -i tf-acc-manual-sshconn-012 -o StrictHostKeyChecking=no
   tfaccmanualsshconn012@192.168.1.68 whoami` should print
   `tfaccmanualsshconn012` without a password prompt) — note this step
   verifies real connectivity; the TrueNAS API itself never checks this at
   create time.
4. In the UI, confirm **Credentials → Backup Credentials** lists
   `tf-acc-manual-sshconn-012`.
5. Edit config: rename to `tf-acc-manual-sshconn-012-renamed` and change
   `connect_timeout` to `20`. `terraform plan` — confirm in-place update (no
   attribute here carries `RequiresReplace`). `terraform apply`.
6. Verify via `midclt call keychaincredential.query` by the new name that
   `connect_timeout` is now 20.
7. `terraform import truenas_keychain_ssh_connection.import_check <id>` into
   a scratch config; `terraform plan` — confirm a clean import (every field
   is echoed back by the API, no ignore list needed).
8. `terraform destroy`. This also removes the keypair, user, and home
   dataset fixtures.
9. Post-destroy verify: `midclt call keychaincredential.query
   '[["name","=","tf-acc-manual-sshconn-012-renamed"]]'` → `[]`; `midclt
   call user.query '[["username","=","tfaccmanualsshconn012"]]'` → `[]`.

**Expected results:** Loopback SSH connection stores correctly and actually
authenticates; every field updates in place; import is clean; destroy tears
down all fixtures including the throwaway user.

**Cleanup:** `rm tf-acc-manual-sshconn-012 tf-acc-manual-sshconn-012.pub
tf-acc-manual-sshconn-012-hostkey.txt`; remove scratch import-check
config/state.

---

### 5. truenas_replication

**MT-TASKS-013 — LOCAL push replication (dataset → dataset)**

**Resource:** truenas_replication (plus fixtures: truenas_dataset x2)
**Type:** Positive — full CRUD plus a real `replication.run`
**Environment:** Same as MT-TASKS-001.
**Preconditions:** None.
**Config:**

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 0.1"
    }
  }
}

variable "truenas_api_key" {
  type      = string
  sensitive = true
}

provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = var.truenas_api_key
  insecure = true
}

resource "truenas_dataset" "src" {
  name = "tank/tf-acc-manual-repl-013-src"
}

resource "truenas_dataset" "dst" {
  name = "tank/tf-acc-manual-repl-013-dst"
}

resource "truenas_replication_task" "test" {
  name             = "tf-acc-manual-repl-013"
  direction        = "PUSH"
  transport        = "LOCAL"
  source_datasets  = [truenas_dataset.src.name]
  target_dataset   = truenas_dataset.dst.name
  recursive        = false
  auto             = false
  retention_policy = "SOURCE"
  enabled          = false

  also_include_naming_schema = ["auto-%Y-%m-%d_%H-%M"]
}
```

**Steps:**

1. `terraform init && terraform apply` (type `yes`).
2. Verify via `midclt call replication.query
   '[["name","=","tf-acc-manual-repl-013"]]'` — confirm `"direction":
   "PUSH"`, `"transport": "LOCAL"`, `"source_datasets":
   ["tank/tf-acc-manual-repl-013-src"]`, `"target_dataset":
   "tank/tf-acc-manual-repl-013-dst"`, `"enabled": false`.
3. In the UI, confirm **Data Protection → Replication Tasks** lists
   `tf-acc-manual-repl-013`.
4. Create a snapshot on the source dataset matching the naming schema, then
   run the task manually and confirm it succeeds:

   ```
   midclt call pool.snapshot.create \
     '{"dataset": "tank/tf-acc-manual-repl-013-src", "name": "auto-'"$(date +%Y-%m-%d_%H-%M)"'"}'
   midclt call -job replication.run <id from state>
   ```

   Confirm the job completes without error and `midclt call
   zfs.snapshot.query '[["dataset","=","tank/tf-acc-manual-repl-013-dst"]]'`
   shows the replicated snapshot landed on the destination.
5. Edit config: set `enabled = true`. `terraform plan` — confirm in-place
   update. `terraform apply`.
6. Verify via `midclt call replication.query` that `enabled` is now `true`.
7. `terraform import truenas_replication_task.import_check <id>` into a
   scratch config; `terraform plan` — confirm clean import, no diff.
8. `terraform destroy`. This also removes both dataset fixtures (and, with
   them, all snapshots).
9. Post-destroy verify: `midclt call replication.query
   '[["name","=","tf-acc-manual-repl-013"]]'` → `[]`; `midclt call
   pool.dataset.query '[["id","=","tank/tf-acc-manual-repl-013-src"]]'` → `[]`.

**Expected results:** LOCAL push replication task created and a real
replication run actually copies a snapshot to the target dataset; in-place
enable/disable; clean import; destroy removes both fixture datasets.

**Cleanup:** None beyond destroy.

---

**MT-TASKS-014 — Remote SSH push replication (loopback via truenas_keychain_ssh_connection)**

**Resource:** truenas_replication (plus fixtures: truenas_dataset x2,
truenas_user, truenas_keychain_ssh_keypair, truenas_keychain_ssh_connection)
**Type:** Positive — full CRUD over SSH transport, plus a real
`replication.run` round trip
**Environment:** Same as MT-TASKS-012 (loopback SSH to 192.168.1.68);
`ssh-keygen` required.
**Preconditions:** Same key-generation and host-key-discovery steps as
MT-TASKS-012, using the `-014` suffix instead of `-012`:

```
ssh-keygen -t ed25519 -N "" -C "tf-acc-manual-repl-014" \
  -f ./tf-acc-manual-repl-014
midclt call keychaincredential.remote_ssh_host_key_scan \
  '{"host": "192.168.1.68", "port": 22}' > tf-acc-manual-repl-014-hostkey.txt
export TF_VAR_public_key="$(cat tf-acc-manual-repl-014.pub)"
export TF_VAR_private_key="$(cat tf-acc-manual-repl-014)"
export TF_VAR_remote_host_key="$(cat tf-acc-manual-repl-014-hostkey.txt)"
```

**Config:**

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 0.1"
    }
  }
}

variable "truenas_api_key" {
  type      = string
  sensitive = true
}
variable "public_key" {
  type = string
}
variable "private_key" {
  type      = string
  sensitive = true
}
variable "remote_host_key" {
  type = string
}

provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = var.truenas_api_key
  insecure = true
}

resource "truenas_dataset" "src" {
  name = "tank/tf-acc-manual-repl-014-src"
}

resource "truenas_dataset" "dst" {
  name = "tank/tf-acc-manual-repl-014-dst"
}

resource "truenas_dataset" "sshuser_home" {
  name = "tank/tf-acc-manual-repl-014-home"
}

# sudo_commands_nopasswd is required: replication.run over SSH needs
# passwordless sudo on the remote side for zfs receive/destroy when
# sudo=true below. Never grant this to a pre-existing/production account.
resource "truenas_user" "sshuser" {
  username               = "tfaccmanualrepl014"
  full_name              = "TF Manual Test SSH Replication User"
  password_disabled      = true
  home                   = "/mnt/${truenas_dataset.sshuser_home.name}"
  shell                  = "/usr/bin/bash"
  group_create           = true
  sshpubkey              = trimspace(var.public_key)
  sudo_commands_nopasswd = ["ALL"]
}

resource "truenas_keychain_ssh_keypair" "test" {
  name        = "tf-acc-manual-repl-014-keypair"
  private_key = var.private_key
}

resource "truenas_keychain_ssh_connection" "test" {
  name            = "tf-acc-manual-repl-014-conn"
  host            = "192.168.1.68"
  port            = 22
  username        = truenas_user.sshuser.username
  private_key_id  = truenas_keychain_ssh_keypair.test.id
  remote_host_key = var.remote_host_key
  connect_timeout = 10
}

resource "truenas_replication_task" "test" {
  name             = "tf-acc-manual-repl-014"
  direction        = "PUSH"
  transport        = "SSH"
  ssh_credentials  = truenas_keychain_ssh_connection.test.id
  sudo             = true
  compression      = "LZ4"
  speed_limit      = 1048576
  source_datasets  = [truenas_dataset.src.name]
  target_dataset   = truenas_dataset.dst.name
  recursive        = false
  auto             = false
  retention_policy = "SOURCE"
  readonly         = "IGNORE"
  enabled          = false

  also_include_naming_schema = ["auto-%Y-%m-%d_%H-%M"]
}
```

**Steps:**

1. `terraform init && terraform apply` (type `yes`).
2. Verify via `midclt call replication.query
   '[["name","=","tf-acc-manual-repl-014"]]'` — confirm `"transport":
   "SSH"`, `"sudo": true`, `"compression": "LZ4"`, `"speed_limit":
   1048576`, and `"ssh_credentials"` resolves to the connection fixture's id.
3. Create a matching snapshot and run the task, same as MT-TASKS-013 step 4
   but pointed at `tank/tf-acc-manual-repl-014-src`/`-dst`:

   ```
   midclt call pool.snapshot.create \
     '{"dataset": "tank/tf-acc-manual-repl-014-src", "name": "auto-'"$(date +%Y-%m-%d_%H-%M)"'"}'
   midclt call -job replication.run <id from state>
   ```

   Confirm the job completes without error (expect roughly 10-20s on first
   run; near-instant on a repeat no-op run) and the snapshot lands on
   `tank/tf-acc-manual-repl-014-dst`.
4. Edit config: set `enabled = true`. `terraform plan` — confirm this is an
   in-place update (`transport` carries `RequiresReplace`, but nothing else
   in this diff touches it). `terraform apply`.
5. `terraform import truenas_replication_task.import_check <id>` into a
   scratch config; `terraform plan` — confirm clean import.
6. `terraform destroy`. This removes the replication task, SSH connection,
   keypair, throwaway user, and both dataset fixtures.
7. Post-destroy verify: `midclt call replication.query
   '[["name","=","tf-acc-manual-repl-014"]]'` → `[]`; `midclt call
   keychaincredential.query '[["name","=","tf-acc-manual-repl-014-conn"]]'`
   → `[]`; `midclt call user.query
   '[["username","=","tfaccmanualrepl014"]]'` → `[]`.

**Expected results:** SSH-transport replication actually authenticates and
transfers a real snapshot over loopback SSH; in-place enable toggle; clean
import; destroy removes every fixture including the throwaway sudo-enabled
user.

**Cleanup:** `rm tf-acc-manual-repl-014 tf-acc-manual-repl-014.pub
tf-acc-manual-repl-014-hostkey.txt`; remove scratch import-check
config/state.

---

### 6. truenas_replication_config

This is a **singleton** — there is one replication configuration
per TrueNAS system. `terraform destroy` never deletes anything server-side;
it only removes the resource from Terraform state, leaving the box's
setting as last applied. Because of this, this case is **disruptive**: it
changes a system-wide, non-namespaced setting, so record and restore the
original value.

**MT-TASKS-015 — Set and restore max_parallel_replication_tasks**

**Resource:** truenas_replication_config
**Type:** Disruptive — singleton resource, requires manual restore
**Environment:** Same as MT-TASKS-001.
**Preconditions:** Record the box's current value before touching anything:

```
midclt call replication.config.config
```

Note the `max_parallel_replication_tasks` value (it may be `null`, meaning
"unlimited" — the provider represents that as state value `0`).

**Config:**

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 0.1"
    }
  }
}

variable "truenas_api_key" {
  type      = string
  sensitive = true
}

provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = var.truenas_api_key
  insecure = true
}

resource "truenas_replication_config" "test" {
  max_parallel_replication_tasks = 3
}
```

**Steps:**

1. `terraform init && terraform apply` (type `yes`).
2. Verify via `midclt call replication.config.config` — confirm
   `"max_parallel_replication_tasks": 3`.
3. `terraform state show truenas_replication_config.test` — confirm `id =
   "replication_config"` (a fixed string, not a numeric id).
4. Edit config: change to `max_parallel_replication_tasks = 0` (meaning
   "unlimited" — sends JSON `null` to the API). `terraform apply`.
5. Verify via `midclt call replication.config.config` that
   `"max_parallel_replication_tasks"` is now `null`, and
   `terraform state show` shows `0` (the sentinel value for "unlimited" —
   this is expected, not a bug).
6. `terraform import truenas_replication_config.import_check
   replication_config` into a scratch config (note the fixed import id);
   `terraform plan` — confirm clean import.
7. `terraform destroy`. Confirm Terraform emits a **warning** (not an
   error) that the configuration was left in place, only removed from
   state.
8. Post-destroy verify: `midclt call replication.config.config` still shows
   `"max_parallel_replication_tasks": null` (unchanged by destroy — this is
   expected).
9. **Restore** the box's original value by hand (Terraform cannot do this
   once the resource is out of state):

   ```
   midclt call replication.config.update \
     '{"max_parallel_replication_tasks": <original value or null, from Preconditions>}'
   ```

**Expected results:** Singleton create/update both map to
`replication.config.update`; `0` in state round-trips to `null` on the wire
and back; destroy only warns and leaves the box's setting untouched; import
uses the fixed id `"replication_config"`.

**Cleanup:** Step 9 above is mandatory cleanup, not optional — verify with
`midclt call replication.config.config` that the box matches its original
pre-test value before ending the session.

---

### 7. truenas_cronjob

**MT-TASKS-016 — Harmless disabled cron job**

**Resource:** truenas_cronjob
**Type:** Positive — full CRUD (create, update, import, destroy)
**Environment:** Same as MT-TASKS-001.
**Preconditions:** None.
**Config:**

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 0.1"
    }
  }
}

variable "truenas_api_key" {
  type      = string
  sensitive = true
}

provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = var.truenas_api_key
  insecure = true
}

resource "truenas_cronjob" "test" {
  command     = "/usr/bin/true"
  user        = "root"
  enabled     = false
  description = "tf-acc-manual-cronjob-016"
  schedule = {
    minute = "0"
    hour   = "3"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}
```

**Steps:**

1. `terraform init && terraform apply` (type `yes`).
2. Verify via `midclt call cronjob.query
   '[["description","=","tf-acc-manual-cronjob-016"]]'` — confirm
   `"command": "/usr/bin/true"`, `"user": "root"`, `"enabled": false`,
   `"schedule": {"minute": "0", "hour": "3", "dom": "*", "month": "*",
   "dow": "*"}`.
3. In the UI, confirm **System → Advanced Settings → Cron Jobs** lists it,
   disabled.
4. Edit config: change `description` to
   `"tf-acc-manual-cronjob-016-updated"`. `terraform plan` — confirm
   in-place update. `terraform apply`.
5. Verify via `midclt call cronjob.query` by the new description that the
   change persisted, and that `command`/`user`/`schedule` are unchanged.
6. `terraform import truenas_cronjob.import_check <id>` into a scratch
   config; `terraform plan` — confirm a completely clean import, no ignore
   list needed (every field the API tracks is echoed back).
7. `terraform destroy`.
8. Post-destroy verify: `midclt call cronjob.query
   '[["description","=","tf-acc-manual-cronjob-016-updated"]]'` → `[]`.

**Expected results:** Cron job never actually runs (`enabled = false`,
`/usr/bin/true` even if it did); description updates in place; fully clean
import; clean destroy.

**Cleanup:** Remove scratch import-check config/state.

---

### 8. truenas_init_shutdown_script

**MT-TASKS-017 — Harmless disabled init/shutdown command**

**Resource:** truenas_init_shutdown_script
**Type:** Positive — full CRUD (create, update, import, destroy)
**Environment:** Same as MT-TASKS-001.
**Preconditions:** None.
**Config:**

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 0.1"
    }
  }
}

variable "truenas_api_key" {
  type      = string
  sensitive = true
}

provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = var.truenas_api_key
  insecure = true
}

resource "truenas_init_shutdown_script" "test" {
  type    = "COMMAND"
  command = "/usr/bin/true"
  when    = "POSTINIT"
  enabled = false
  comment = "tf-acc-manual-ish-017"
}
```

**Steps:**

1. `terraform init && terraform apply` (type `yes`).
2. Verify via `midclt call initshutdownscript.query
   '[["comment","=","tf-acc-manual-ish-017"]]'` — confirm `"type":
   "COMMAND"`, `"command": "/usr/bin/true"`, `"when": "POSTINIT"`,
   `"enabled": false`.
3. In the UI, confirm **System → Advanced Settings → Init/Shutdown
   Scripts** lists it, disabled.
4. Edit config: change `comment` to `"tf-acc-manual-ish-017-updated"`.
   `terraform plan` — confirm in-place update. `terraform apply`.
5. Verify via `midclt call initshutdownscript.query` by the new comment
   that the change persisted.
6. `terraform import truenas_init_shutdown_script.import_check <id>` into a
   scratch config; `terraform plan` — confirm a completely clean import.
7. `terraform destroy`.
8. Post-destroy verify: `midclt call initshutdownscript.query
   '[["comment","=","tf-acc-manual-ish-017-updated"]]'` → `[]`.

**Expected results:** Script never actually executes (`enabled = false`);
comment updates in place; fully clean import; clean destroy.

**Cleanup:** Remove scratch import-check config/state.

---

### 9. truenas_rsync_task

**MT-TASKS-018 — MODULE mode, unreachable remote**

**Resource:** truenas_rsync_task (plus fixture: truenas_dataset)
**Type:** Positive — full CRUD (create, update, import, destroy); demonstrates
`rsynctask.create` does not validate remote connectivity in MODULE mode
**Environment:** Same as MT-TASKS-001.
**Preconditions:** None.
**Config:**

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 0.1"
    }
  }
}

variable "truenas_api_key" {
  type      = string
  sensitive = true
}

provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = var.truenas_api_key
  insecure = true
}

resource "truenas_dataset" "fixture" {
  name = "tank/tf-acc-manual-rsync-018"
}

resource "truenas_rsync_task" "test" {
  path           = truenas_dataset.fixture.mountpoint
  user           = "root"
  mode           = "MODULE"
  remotehost     = "192.0.2.10" # TEST-NET-1: guaranteed unreachable, deliberately
  remotemodule   = "tfacc"
  enabled        = false
  validate_rpath = false
  desc           = "tf-acc-manual-rsync-018"
  schedule = {
    minute = "0"
    hour   = "3"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}
```

**Steps:**

1. `terraform init && terraform apply` (type `yes`) — confirm this succeeds
   even though `remotehost` is unreachable: MODULE mode's create does not
   validate remote connectivity, and `validate_rpath` is an SSH-mode-only
   concept (a no-op here regardless of its value).
2. Verify via `midclt call rsynctask.query
   '[["path","=","/mnt/tank/tf-acc-manual-rsync-018"]]'` — confirm `"mode":
   "MODULE"`, `"remotehost": "192.0.2.10"`, `"remotemodule": "tfacc"`,
   `"enabled": false`.
3. In the UI, confirm **Data Protection → Rsync Tasks** lists it, disabled.
4. Edit config: change `desc` to `"tf-acc-manual-rsync-018-updated"`.
   `terraform plan` — confirm in-place update. `terraform apply`.
5. Verify via `midclt call rsynctask.query` that `desc` persisted.
6. `terraform import truenas_rsync_task.import_check <id>` into a scratch
   config; `terraform plan` — confirm clean import **except**
   `validate_rpath` and `ssh_keyscan` (both write-only: `rsynctask.query`
   never returns them, so they always show as unknown/changed on import —
   this is expected, not a bug).
7. `terraform destroy`. This also removes the dataset fixture.
8. Post-destroy verify: `midclt call rsynctask.query
   '[["path","=","/mnt/tank/tf-acc-manual-rsync-018"]]'` → `[]`; `midclt
   call pool.dataset.query
   '[["id","=","tank/tf-acc-manual-rsync-018"]]'` → `[]`.

**Expected results:** MODULE-mode task creates successfully against an
unreachable host; `desc` updates in place; import is clean aside from the
two documented write-only flags; destroy removes the task and its dataset
fixture.

**Cleanup:** Remove scratch import-check config/state.

---

**MT-TASKS-019 — SSH mode, loopback to the test box itself**

**Resource:** truenas_rsync_task (plus fixtures: truenas_dataset x2,
truenas_user, truenas_keychain_ssh_keypair, truenas_keychain_ssh_connection)
**Type:** Positive — full CRUD over SSH mode, real reachable loopback target
**Environment:** Same as MT-TASKS-012 (loopback SSH to 192.168.1.68);
`ssh-keygen` required.
**Preconditions:** Same key-generation/host-key steps as MT-TASKS-012, `-019` suffix:

```
ssh-keygen -t ed25519 -N "" -C "tf-acc-manual-rsync-019" \
  -f ./tf-acc-manual-rsync-019
midclt call keychaincredential.remote_ssh_host_key_scan \
  '{"host": "192.168.1.68", "port": 22}' > tf-acc-manual-rsync-019-hostkey.txt
export TF_VAR_public_key="$(cat tf-acc-manual-rsync-019.pub)"
export TF_VAR_private_key="$(cat tf-acc-manual-rsync-019)"
export TF_VAR_remote_host_key="$(cat tf-acc-manual-rsync-019-hostkey.txt)"
```

**Config:**

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 0.1"
    }
  }
}

variable "truenas_api_key" {
  type      = string
  sensitive = true
}
variable "public_key" {
  type = string
}
variable "private_key" {
  type      = string
  sensitive = true
}
variable "remote_host_key" {
  type = string
}

provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = var.truenas_api_key
  insecure = true
}

resource "truenas_dataset" "src" {
  name = "tank/tf-acc-manual-rsync-019-src"
}

resource "truenas_dataset" "dst" {
  name = "tank/tf-acc-manual-rsync-019-dst"
}

resource "truenas_dataset" "sshuser_home" {
  name = "tank/tf-acc-manual-rsync-019-home"
}

resource "truenas_user" "sshuser" {
  username          = "tfaccmanualrsync019"
  full_name         = "TF Manual Test Rsync SSH User"
  password_disabled = true
  home              = "/mnt/${truenas_dataset.sshuser_home.name}"
  shell             = "/usr/bin/bash"
  group_create      = true
  sshpubkey         = trimspace(var.public_key)
}

resource "truenas_keychain_ssh_keypair" "test" {
  name        = "tf-acc-manual-rsync-019-keypair"
  private_key = var.private_key
}

resource "truenas_keychain_ssh_connection" "test" {
  name            = "tf-acc-manual-rsync-019-conn"
  host            = "192.168.1.68"
  port            = 22
  username        = truenas_user.sshuser.username
  private_key_id  = truenas_keychain_ssh_keypair.test.id
  remote_host_key = var.remote_host_key
  connect_timeout = 10
}

resource "truenas_rsync_task" "test" {
  path            = truenas_dataset.src.mountpoint
  user            = "root"
  mode            = "SSH"
  remotehost      = "192.168.1.68"
  remoteport      = 22
  ssh_credentials = truenas_keychain_ssh_connection.test.id
  remotepath      = truenas_dataset.dst.mountpoint
  direction       = "PUSH"
  enabled         = false
  validate_rpath  = false
  desc            = "tf-acc-manual-rsync-019"
  schedule = {
    minute = "0"
    hour   = "3"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}
```

**Steps:**

1. `terraform init && terraform apply` (type `yes`).
2. Verify via `midclt call rsynctask.query
   '[["path","=","/mnt/tank/tf-acc-manual-rsync-019-src"]]'` — confirm
   `"mode": "SSH"`, `"remotehost": "192.168.1.68"`, `"remoteport": 22`,
   `"ssh_credentials"` resolves to the connection fixture's id, `"remotepath":
   "/mnt/tank/tf-acc-manual-rsync-019-dst"`.
3. Optional real-run verification: put a throwaway file in the source
   dataset, then trigger a manual run and confirm the file lands on the
   target:

   ```
   touch /mnt/tank/tf-acc-manual-rsync-019-src/probe.txt
   midclt call -job rsynctask.run <id from state>
   ```

   then check (over SSH or the UI shell) that
   `/mnt/tank/tf-acc-manual-rsync-019-dst/probe.txt` exists.
4. Edit config: change `desc` to `"tf-acc-manual-rsync-019-updated"`.
   `terraform apply` — confirm in-place update.
5. `terraform import truenas_rsync_task.import_check <id>` into a scratch
   config; `terraform plan` — confirm clean aside from `validate_rpath`/
   `ssh_keyscan` (write-only, same as MT-TASKS-018).
6. `terraform destroy`. This removes the rsync task, SSH connection,
   keypair, throwaway user, and all three dataset fixtures.
7. Post-destroy verify: `midclt call rsynctask.query
   '[["path","=","/mnt/tank/tf-acc-manual-rsync-019-src"]]'` → `[]`;
   `midclt call user.query
   '[["username","=","tfaccmanualrsync019"]]'` → `[]`.

**Expected results:** SSH-mode rsync task authenticates and (if step 3 is
exercised) actually transfers a file over loopback SSH; in-place update;
clean import aside from the two write-only flags; destroy removes every
fixture.

**Cleanup:** `rm tf-acc-manual-rsync-019 tf-acc-manual-rsync-019.pub
tf-acc-manual-rsync-019-hostkey.txt`; remove scratch import-check
config/state.

---

### 10. truenas_cloudsync

`cloudsync.create` does **not** validate the credential/bucket against a
real remote endpoint (unlike `truenas_cloud_backup` — see section 12), so
this case is safe to run with entirely dummy provider values chained
through a throwaway `truenas_cloudsync_credentials` fixture.

**MT-TASKS-020 — Cloud sync task with dummy STORJ_IX credentials**

**Resource:** truenas_cloudsync (plus fixture: truenas_cloudsync_credentials)
**Type:** Positive — full CRUD (create, update, import); no destroy-by-run (never call `cloudsync.sync`)
**Environment:** Same as MT-TASKS-001.
**Preconditions:** None — this deliberately never triggers a real sync.
**Config:**

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 0.1"
    }
  }
}

variable "truenas_api_key" {
  type      = string
  sensitive = true
}

provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = var.truenas_api_key
  insecure = true
}

resource "truenas_cloudsync_credentials" "test" {
  name = "tf-acc-manual-cloudsync-020-creds"
  provider_config = jsonencode({
    type              = "STORJ_IX"
    access_key_id     = "tf-acc-manual-dummy"
    secret_access_key = "tf-acc-manual-dummy"
  })
}

resource "truenas_cloudsync_task" "test" {
  description   = "tf-acc-manual-cloudsync-020"
  path          = "/mnt/tank"
  credentials   = truenas_cloudsync_credentials.test.id
  direction     = "PUSH"
  transfer_mode = "SYNC"
  attributes    = jsonencode({ bucket = "tf-acc-manual-bucket", folder = "backups" })
  enabled       = false
  schedule = {
    minute = "0"
    hour   = "0"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}
```

**Steps:**

1. `terraform init && terraform apply` (type `yes`) — confirm this succeeds
   with dummy credentials and a dummy bucket name.
2. Verify via `midclt call cloudsync.query
   '[["description","=","tf-acc-manual-cloudsync-020"]]'` — confirm
   `"direction": "PUSH"`, `"transfer_mode": "SYNC"`, `"path": "/mnt/tank"`,
   `"enabled": false`, `"attributes"` includes `"bucket":
   "tf-acc-manual-bucket"`.
3. In the UI, confirm **Data Protection → Cloud Sync Tasks** lists it,
   disabled.
4. Edit config: set `enabled = true`. `terraform plan` — confirm in-place
   update. `terraform apply`.
5. Verify via `midclt call cloudsync.query` that `enabled` is now `true`.
   **Do not** manually trigger `cloudsync.sync` — that would attempt a real
   network call against the dummy STORJ_IX endpoint and is out of scope for
   this CRUD case.
6. `terraform import truenas_cloudsync_task.import_check <id>` into a
   scratch config; `terraform plan` — confirm clean import.
7. `terraform destroy`. This also removes the credentials fixture.
8. Post-destroy verify: `midclt call cloudsync.query
   '[["description","=","tf-acc-manual-cloudsync-020"]]'` → `[]`.

**Expected results:** Cloud sync task creates/updates/imports cleanly with
dummy provider values (confirming `cloudsync.create` performs no real
endpoint validation); clean destroy.

**Cleanup:** Remove scratch import-check config/state.

---

### 11. truenas_cloudsync_credentials

**MT-TASKS-021 — STORJ_IX credentials, full CRUD**

**Resource:** truenas_cloudsync_credentials
**Type:** Positive — full CRUD (create, rename update, import, destroy)
**Environment:** Same as MT-TASKS-001.
**Preconditions:** None.
**Config:**

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 0.1"
    }
  }
}

variable "truenas_api_key" {
  type      = string
  sensitive = true
}

provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = var.truenas_api_key
  insecure = true
}

resource "truenas_cloudsync_credentials" "test" {
  name = "tf-acc-manual-cloudcreds-021"
  provider_config = jsonencode({
    type              = "STORJ_IX"
    access_key_id     = "tf-acc-manual-dummy"
    secret_access_key = "tf-acc-manual-dummy"
  })
}
```

**Steps:**

1. `terraform init && terraform apply` (type `yes`).
2. Verify via `midclt call cloudsync.credentials.query
   '[["name","=","tf-acc-manual-cloudcreds-021"]]'` — confirm
   `"provider"` shows `"type": "STORJ_IX"` plus the dummy access
   key/secret, and that create succeeded without contacting a real STORJ
   endpoint.
3. In the UI, confirm **Credentials → Backup Credentials → Cloud
   Credentials** lists it.
4. Edit config: rename to `tf-acc-manual-cloudcreds-021-renamed`.
   `terraform plan` — confirm in-place update. `terraform apply`.
5. Verify via `midclt call cloudsync.credentials.query` by the new name
   that it persisted.
6. `terraform import truenas_cloudsync_credentials.import_check <id>` into
   a scratch config; `terraform plan` — confirm clean import except
   `provider_config` (compare semantically with `jq -S`, not
   byte-for-byte, same caveat as MT-TASKS-007).
7. `terraform destroy`.
8. Post-destroy verify: query by the renamed name returns `[]`.

**Expected results:** Credentials created/renamed/imported/destroyed
cleanly with dummy values; no real STORJ_IX validation occurs.

**Cleanup:** Remove scratch import-check config/state.

---

### 12. truenas_cloud_backup

**MT-TASKS-022 — Real S3-compatible credentials required (Manual-only)**

**Resource:** truenas_cloud_backup
**Type:** Manual-only — `cloud_backup.create` validates the credential
against the **real** remote bucket at apply time (confirmed live: a
fabricated S3 access key was rejected with `InvalidAccessKeyId ... calling
the GetBucketLocation operation`, before any local state was written).
Dummy credentials will always fail this case; it requires a real,
reachable S3-compatible bucket and working credentials.
**Environment:** Same as MT-TASKS-001, plus a real S3-compatible bucket
(e.g. AWS S3, Backblaze B2 configured as S3, MinIO, etc.) and its access
key/secret that you control and are willing to have TrueNAS open a restic
repository against.
**Preconditions:**

1. Have on hand: a real bucket name, a real access key ID, and a real
   secret access key with permission to read/write that bucket.
2. Pick a small, disposable local path to back up (e.g. a throwaway
   dataset fixture — do **not** point this at anything you care about
   losing, since `cloud_backup.sync` is never called in this test, but the
   task itself will be live and functional).

```
export TF_VAR_s3_access_key_id="<your real access key id>"
export TF_VAR_s3_secret_access_key="<your real secret access key>"
export TF_VAR_restic_password="tf-acc-manual-cloudbackup-022-password"
```

**Config:**

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 0.1"
    }
  }
}

variable "truenas_api_key" {
  type      = string
  sensitive = true
}
variable "s3_access_key_id" {
  type      = string
  sensitive = true
}
variable "s3_secret_access_key" {
  type      = string
  sensitive = true
}
variable "restic_password" {
  type      = string
  sensitive = true
}

provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = var.truenas_api_key
  insecure = true
}

resource "truenas_dataset" "fixture" {
  name = "tank/tf-acc-manual-cloudbackup-022"
}

resource "truenas_cloudsync_credentials" "test" {
  name = "tf-acc-manual-cloudbackup-022-creds"
  provider_config = jsonencode({
    type              = "S3"
    access_key_id     = var.s3_access_key_id
    secret_access_key = var.s3_secret_access_key
  })
}

resource "truenas_cloud_backup" "test" {
  description = "tf-acc-manual-cloudbackup-022"
  path        = truenas_dataset.fixture.mountpoint
  credentials = truenas_cloudsync_credentials.test.id
  attributes  = jsonencode({ bucket = "<YOUR REAL BUCKET NAME>", folder = "tf-acc-manual-022" })
  password    = var.restic_password
  keep_last   = 1
  enabled     = false
}
```

**Steps** (only runnable with real credentials — otherwise skip and record
"not run: no S3-compatible fixture available"):

1. Replace `<YOUR REAL BUCKET NAME>` with your actual bucket. `terraform
   init && terraform apply` (type `yes`). This performs a **real** restic
   repository check (`cloud_backup.ensure_initialized`) against your bucket
   — expect a genuine network round trip, not an instant local apply.
2. Verify via `midclt call cloud_backup.query
   '[["description","=","tf-acc-manual-cloudbackup-022"]]'` — confirm
   `"path"`, `"keep_last": 1`, `"enabled": false`, and that `"password"`
   round-trips in cleartext (an admin-scoped API key session sees the real
   value, not `"********"`).
3. In the UI, confirm **Data Protection → Cloud Backup Tasks** lists it,
   disabled.
4. **Do not** call `cloud_backup.sync` or otherwise trigger a real
   backup/restore in this test — that is out of scope for a CRUD check and
   would consume real storage in your bucket.
5. Edit config: change `keep_last` to `2`. `terraform plan` — confirm
   in-place update (note `absolute_paths` would force a replace if changed;
   `keep_last` does not). `terraform apply`.
6. `terraform import truenas_cloud_backup.import_check <id>` into a scratch
   config; `terraform plan` — confirm clean import. If your API key session
   is not FULL_ADMIN-scoped, expect `password` to read back masked as
   `"********"` instead — in that case treat it as an expected ignore, not
   a bug.
7. `terraform destroy`.
8. Post-destroy verify: `midclt call cloud_backup.query
   '[["description","=","tf-acc-manual-cloudbackup-022"]]'` → `[]`.
   Separately, confirm in your S3 provider's console/CLI that no restic
   repository objects were left behind from step 1's `ensure_initialized`
   call (an empty restic repo skeleton may have been created in the bucket
   — clean it up manually if so, since `cloud_backup.delete` does not touch
   remote bucket contents).

**Expected results:** Create/update/import/destroy all succeed against a
real bucket; a dummy/fake credential predictably fails at apply with
`InvalidAccessKeyId`-style errors before any Terraform state is written
(worth confirming once, deliberately, with a fake key, to see the failure
mode match this description).

**Cleanup:** Manually verify and remove any stray restic repository objects
left in your real bucket (see step 8); unset the `TF_VAR_s3_*` environment
variables.

---

## Filesystem, Apps, and Containers

Conventions used throughout this section:

- All resource names created by these tests use the `tf-acc-manual-` prefix so they are
  trivially distinguishable from anything else on the box and safe to bulk-search for leftovers
  (`midclt call <namespace>.query '[["name","~","tf-acc-manual"]]'`).
- Two boxes are referenced by their literal addresses:
  - **25.10 box** — TrueNAS 25.10 test VM (VM 110 on `pve`), `192.168.1.249`, pool `tank`.
  - **26.0 box** — TrueNAS 26.0+, `192.168.1.68`, pool `tank`.
  Do not substitute placeholders for these; use the literal hostnames/IPs.
- `midclt call` commands are run over SSH on the TrueNAS box itself, e.g.
  `ssh root@192.168.1.68 midclt call container.query '[["name","=","tf-acc-manual-ctr-01"]]'`.
- Every `provider "truenas"` block below is the same shape; fill in a real API key before running:
  ```hcl
  provider "truenas" {
    endpoint = "wss://<BOX_IP>/api/current"
    api_key  = "REPLACE_WITH_API_KEY"
    insecure = true
  }
  ```
  Substitute `<BOX_IP>` with the literal box address named in each case's **Environment** field
  (192.168.1.249 or 192.168.1.68) — this is a credential placeholder to fill in, not a host
  placeholder to leave unresolved.
- "UI" verification steps describe the general TrueNAS navigation; specific menu wording can
  drift slightly between builds. Treat the `midclt call` output as the authoritative check and the
  UI screen as a secondary confirmation that the change is visible where an operator would look
  for it.

### 1. truenas_filesystem_permissions

`truenas_filesystem_permissions` is a path-keyed wrapper around the imperative
`filesystem.setperm`/`filesystem.stat` actions — it is not an object TrueNAS itself tracks, so
`terraform destroy` is documented to be **state-only**: it removes the resource from Terraform
state and emits a warning, but never reverts the path's mode/uid/gid. Both cases below run against
either box; the 26.0 box (`192.168.1.68`) is used for concreteness.

**MT-APPS-001 — Full lifecycle: create, verify, update, import, destroy (state-only), post-destroy persistence check**

**Resource:** truenas_filesystem_permissions
**Type:** Resource
**Environment:** 26.0 box (192.168.1.68), pool tank. No version gate; no Apps gate.
**Preconditions:**
- Provider configured and authenticated against the 26.0 box.
- No pre-existing dataset/user/group named `tf-acc-manual-fsperm-*`.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = "REPLACE_WITH_API_KEY"
  insecure = true
}

resource "truenas_dataset" "fsperm_fixture" {
  name = "tank/tf-acc-manual-fsperm-ds"
}

resource "truenas_user" "fsperm_fixture" {
  username          = "tf-acc-manual-fsperm-user"
  full_name         = "TF Acc Manual Fsperm User"
  password          = "Tf-Acc-Manual-Passw0rd!"
  password_disabled = false
  home              = "/var/empty"
  shell             = "/usr/bin/bash"
  smb               = false
  group_create      = true
}

resource "truenas_group" "fsperm_fixture" {
  name = "tf-acc-manual-fsperm-grp"
}

resource "truenas_filesystem_permissions" "test" {
  path = truenas_dataset.fsperm_fixture.mountpoint
  mode = "0750"
  uid  = truenas_user.fsperm_fixture.uid
  gid  = truenas_group.fsperm_fixture.gid
}
```

**Steps:**
1. `terraform apply`. Confirm the plan shows 4 resources to add.
2. Verify in the UI: Datasets > `tank/tf-acc-manual-fsperm-ds` > Edit Permissions shows owner =
   `tf-acc-manual-fsperm-user`, group = `tf-acc-manual-fsperm-grp`, mode `750`.
3. Verify via API: `midclt call filesystem.stat '"/mnt/tank/tf-acc-manual-fsperm-ds"'` — confirm
   `mode & 07777` (in octal) is `0750`, and `uid`/`gid` match the fixture user/group.
4. `terraform state show truenas_filesystem_permissions.test` — confirm `id` equals `path`
   (`/mnt/tank/tf-acc-manual-fsperm-ds`).
5. Edit the config: change `mode = "0750"` to `mode = "0770"`. `terraform apply` again — confirm
   only `truenas_filesystem_permissions.test` is updated in place (no replacement).
6. Re-run the `filesystem.stat` check from step 3 — confirm mode is now `0770`.
7. `terraform import truenas_filesystem_permissions.import_test /mnt/tank/tf-acc-manual-fsperm-ds`
   against a throwaway config containing an empty `resource "truenas_filesystem_permissions"
   "import_test" {}` block, then `terraform plan` — confirm no diff on `path`/`mode`/`uid`/`gid`,
   and that `recursive`/`traverse` show as unknown/absent (they are apply-time-only and cannot be
   recovered from the API — this is expected, not a bug).
8. Remove only the `truenas_filesystem_permissions.test` resource block from the config (leave the
   dataset/user/group fixtures in place). `terraform apply`.
9. Confirm the apply output includes a **warning** diagnostic ("Filesystem permissions left in
   place"), not an error, and that the apply otherwise succeeds.
10. **Post-destroy verification (the documented behavior under test):** re-run
    `midclt call filesystem.stat '"/mnt/tank/tf-acc-manual-fsperm-ds"'` — confirm the dataset still
    exists and its mode is **still `0770`** (not reverted to whatever it was before Terraform ever
    touched it, and not deleted). Also confirm via
    `midclt call pool.dataset.get_instance '"tank/tf-acc-manual-fsperm-ds"'` that the dataset
    object itself is untouched.
11. Clean up the remaining fixtures (see Cleanup).

**Expected results:**
- Step 1-3: mode/uid/gid applied and correctly read back both via UI and API.
- Step 5-6: in-place update, no resource replacement, new mode takes effect.
- Step 7: import succeeds; `recursive`/`traverse` are not part of the imported state (by design).
- Step 9-10: `terraform destroy`-equivalent step succeeds with a **warning**, and the path's
  mode/uid/gid are **unchanged** afterward — this is the intended, documented Delete semantic, not
  a defect. A QA engineer unfamiliar with this resource should not report step 10's "nothing
  changed after destroy" as a bug.

**Cleanup:** `terraform destroy` on the remaining config (dataset/user/group fixtures), or manually
via `midclt call pool.dataset.delete '"tank/tf-acc-manual-fsperm-ds"' '{"recursive": true}'` plus
deleting the fixture user/group from the UI.

**MT-APPS-002 — Out-of-band drift detection (chmod outside Terraform)**

**Resource:** truenas_filesystem_permissions
**Type:** Resource
**Environment:** 26.0 box (192.168.1.68), pool tank.
**Preconditions:** A dataset fixture exists (reuse `tank/tf-acc-manual-fsperm-ds` from
MT-APPS-001, or create a fresh one).

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = "REPLACE_WITH_API_KEY"
  insecure = true
}

resource "truenas_dataset" "drift_fixture" {
  name = "tank/tf-acc-manual-fsperm-drift"
}

resource "truenas_filesystem_permissions" "test" {
  path = truenas_dataset.drift_fixture.mountpoint
  mode = "0755"
}
```

**Steps:**
1. `terraform apply`. Confirm mode `0755` applied.
2. Outside of Terraform, change the mode via the UI (Datasets > edit permissions, set to `700`) or
   via `midclt call filesystem.setperm '{"path": "/mnt/tank/tf-acc-manual-fsperm-drift", "mode":
   "0700"}'`.
3. `terraform plan` (no apply).

**Expected results:** the plan shows a diff on `mode` (`"0755"` -> `"0700"`), proving Read
drift-checks the live `filesystem.stat` value on every refresh rather than trusting stale state.
Running `terraform apply` at this point re-asserts `0755` (Terraform, not the out-of-band change,
wins on the next apply) — confirm that too.

**Cleanup:** `terraform destroy`, then delete the dataset as in MT-APPS-001.

### 2. truenas_filesystem_acl

`truenas_filesystem_acl` is the ACL sibling of `truenas_filesystem_permissions`. Unlike
permissions, **Delete does mutate the path**: it calls `filesystem.setacl` with
`options.stripacl=true`, which converts the ACL to a trivial, mode-derived one (`filesystem.getacl`
reports `"trivial": true` afterward). This is a deliberate strip-on-destroy, not a no-op.

The pool `tank` has `acltype=POSIX` set LOCAL at the root, so **a plain child dataset defaults to
POSIX1E** — that is the brand testable with zero extra setup (MT-APPS-004). Testing NFS4 requires
creating the dataset with an explicit `acltype=NFSV4` **and** `aclmode=PASSTHROUGH` override
(`truenas_dataset` does not expose `aclmode`, so this must be done via `midclt call` or the UI's
advanced dataset creation options before `truenas_filesystem_acl` is pointed at it) — otherwise
`pool.dataset.create` rejects `acltype=NFSV4` with `EINVAL` ("aclmode may not be set for NFSv4 acl
type") because of the inherited `aclmode=DISCARD`.

**MT-APPS-003 — NFS4 ACL lifecycle on a specially-created dataset**

**Resource:** truenas_filesystem_acl
**Type:** Resource
**Environment:** 26.0 box (192.168.1.68), pool tank.
**Preconditions:**
- Create the NFS4-capable dataset out-of-band first (Terraform's `truenas_dataset` cannot express
  this):
  `midclt call pool.dataset.create '{"name": "tank/tf-acc-manual-facl-nfs4", "acltype": "NFSV4",
  "aclmode": "PASSTHROUGH"}'`

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = "REPLACE_WITH_API_KEY"
  insecure = true
}

resource "truenas_user" "facl_fixture" {
  username          = "tf-acc-manual-facl-user"
  full_name         = "TF Acc Manual Facl User"
  password          = "Tf-Acc-Manual-Passw0rd!"
  password_disabled = false
  home              = "/var/empty"
  shell             = "/usr/bin/bash"
  smb               = false
  group_create      = true
}

resource "truenas_filesystem_acl" "test" {
  path = "/mnt/tank/tf-acc-manual-facl-nfs4"
  entries = jsonencode([
    {
      tag   = "owner@"
      type  = "ALLOW"
      perms = { BASIC = "FULL_CONTROL" }
      flags = { BASIC = "INHERIT" }
    },
    {
      tag   = "group@"
      type  = "ALLOW"
      perms = { BASIC = "MODIFY" }
      flags = { BASIC = "INHERIT" }
    },
    {
      tag   = "USER"
      id    = truenas_user.facl_fixture.uid
      type  = "ALLOW"
      perms = { BASIC = "READ" }
      flags = { BASIC = "INHERIT" }
    },
  ])
}
```

**Steps:**
1. `terraform apply`. Confirm `acltype` reads back `"NFS4"` and `uid`/`gid` are set (owner of the
   dataset).
2. Verify via API: `midclt call filesystem.getacl '"/mnt/tank/tf-acc-manual-facl-nfs4"'` — confirm
   3 ACEs, `acltype: "NFS4"`, `trivial: false`, and `owner@`'s `perms.BASIC` is `"FULL_CONTROL"`.
3. Verify in the UI: Datasets > `tank/tf-acc-manual-facl-nfs4` > Edit ACL shows the 3 entries with
   matching permissions.
4. Edit the config: swap `owner@`'s perms to `"MODIFY"` and `group@`'s to `"FULL_CONTROL"`.
   `terraform apply`.
5. Re-run the `filesystem.getacl` check — confirm `owner@` now reads `"MODIFY"`.
6. `terraform import truenas_filesystem_acl.import_test /mnt/tank/tf-acc-manual-facl-nfs4` against
   an empty resource block, `terraform plan` — confirm no diff except `recursive`/`traverse` being
   absent from imported state (apply-time-only, expected).
7. Remove `truenas_filesystem_acl.test` from the config (leave the user fixture and the raw-created
   dataset). `terraform apply`.
8. Confirm the apply output includes a **warning** ("Filesystem ACL stripped to a trivial,
   mode-derived ACL"), and the apply succeeds.
9. **Post-destroy verification:** `midclt call filesystem.getacl
   '"/mnt/tank/tf-acc-manual-facl-nfs4"'` — confirm `trivial: true` now (the ACL WAS stripped, in
   contrast to `truenas_filesystem_permissions`'s state-only Delete in MT-APPS-001). Also confirm
   `midclt call filesystem.stat '"/mnt/tank/tf-acc-manual-facl-nfs4"'`'s `acl` field is now `false`.

**Expected results:** create/update/import behave like MT-APPS-001's pattern; the key documented
difference to verify is step 9 — destroy here **does** mutate the path (strips to trivial), unlike
`truenas_filesystem_permissions`'s pure state-only Delete. A QA engineer should flag it as a defect
only if `trivial` is still `false` after this step, or if the mode/uid/gid of the path changed
unexpectedly (they must not — only the non-trivial ACL entries are removed).

**Cleanup:** delete the user fixture via `terraform destroy` on the remaining config, then
`midclt call pool.dataset.delete '"tank/tf-acc-manual-facl-nfs4"' '{"recursive": true}'`.

**MT-APPS-004 — POSIX1E ACL on a default (unmodified) dataset, plus MASK-entry negative test**

**Resource:** truenas_filesystem_acl
**Type:** Resource
**Environment:** 26.0 box (192.168.1.68), pool tank (root `acltype=POSIX` inherited by children).
**Preconditions:** none beyond a working provider connection — this is the acltype a plain
`truenas_dataset` gets with no special handling, which is the point of this case.

**Config (positive case):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = "REPLACE_WITH_API_KEY"
  insecure = true
}

resource "truenas_dataset" "facl_posix_fixture" {
  name = "tank/tf-acc-manual-facl-posix"
}

resource "truenas_user" "facl_posix_user" {
  username          = "tf-acc-manual-facl-posix-user"
  full_name         = "TF Acc Manual Facl Posix User"
  password          = "Tf-Acc-Manual-Passw0rd!"
  password_disabled = false
  home              = "/var/empty"
  shell             = "/usr/bin/bash"
  smb               = false
  group_create      = true
}

resource "truenas_filesystem_acl" "test" {
  path = truenas_dataset.facl_posix_fixture.mountpoint
  entries = jsonencode([
    { tag = "USER_OBJ", perms = { READ = true, WRITE = true, EXECUTE = true }, default = false },
    { tag = "USER", id = truenas_user.facl_posix_user.uid, perms = { READ = true, WRITE = false, EXECUTE = false }, default = false },
    { tag = "GROUP_OBJ", perms = { READ = true, WRITE = false, EXECUTE = true }, default = false },
    { tag = "MASK", perms = { READ = true, WRITE = true, EXECUTE = true }, default = false },
    { tag = "OTHER", perms = { READ = false, WRITE = false, EXECUTE = false }, default = false },
  ])
}
```

**Steps:**
1. `terraform apply`. Confirm `acltype` reads back `"POSIX1E"`.
2. Verify via API: `midclt call filesystem.getacl '"/mnt/tank/tf-acc-manual-facl-posix"'` — confirm
   5 entries and `trivial: false`.
3. `terraform plan` immediately after apply — confirm **no diff** (this exercises the entry-order
   normalization: TrueNAS may reorder POSIX1E entries server-side, e.g. to
   USER_OBJ/USER/GROUP_OBJ/MASK/OTHER — the config above is already written in that canonical
   order specifically so this step is clean; if you write entries in a different order, expect a
   perpetual diff on `entries`, which is the documented behavior, not a bug).
4. `terraform destroy` (or remove the resource block and apply). Confirm the warning is emitted and
   `filesystem.getacl` reports `trivial: true` afterward, same as MT-APPS-003 step 9.

**Negative sub-case — missing MASK entry with a named USER/GROUP present:**
5. Re-apply the resource, but delete the `MASK` entry from the `entries` list (keep the `USER`
   entry).
6. `terraform apply` — expect this to **fail** with an error surfaced from `filesystem.setacl`
   containing `EINVAL` and "Named (user or group) POSIX ACL entries require a mask entry to be
   present in the ACL". Confirm Terraform reports this as a clean apply-time error (not a panic or
   an opaque HTTP failure), and that no partial ACL was applied
   (`midclt call filesystem.getacl ...` should still show the prior state or the trivial default,
   never a half-written ACL missing the mask).

**Expected results:** positive case behaves like NFS4's lifecycle; the entry-order note in step 3
and the MASK-required negative test in steps 5-6 are POSIX1E-specific behaviors this case exists
to catch.

**Cleanup:** `terraform destroy` all resources in this case's config.

### 3. truenas_acl_template

**MT-APPS-005 — ACL template CRUD, never touching builtins**

**Resource:** truenas_acl_template
**Type:** Resource
**Environment:** either box (25.10 box 192.168.1.249 or 26.0 box 192.168.1.68). No version gate.
**Preconditions:** none. Before starting, run
`midclt call filesystem.acltemplate.query '[["builtin","=",true]]'` and note there are 9 builtin
templates (NFS4_OPEN, NFS4_RESTRICTED, NFS4_HOME, NFS4_DOMAIN_HOME, POSIX_OPEN, POSIX_RESTRICTED,
POSIX_HOME, NFS4_ADMIN, POSIX_ADMIN) — this test must never create/update/delete any of them.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = "REPLACE_WITH_API_KEY"
  insecure = true
}

resource "truenas_acl_template" "test" {
  name    = "tf-acc-manual-acltemplate"
  acltype = "NFS4"
  comment = "initial comment"
  acl = jsonencode([
    { tag = "owner@", type = "ALLOW", perms = { BASIC = "FULL_CONTROL" }, flags = { BASIC = "INHERIT" } },
    { tag = "group@", type = "ALLOW", perms = { BASIC = "MODIFY" }, flags = { BASIC = "INHERIT" } },
    { tag = "everyone@", type = "ALLOW", perms = { BASIC = "READ" }, flags = { BASIC = "INHERIT" } },
  ])
}
```

**Steps:**
1. `terraform apply`. Confirm `builtin` reads back `false` and `id` is a small non-negative
   integer distinct from the 9 builtin ids.
2. Verify via API: `midclt call filesystem.acltemplate.query '[["name","=","tf-acc-manual-
   acltemplate"]]'` — confirm 3 ACL entries and `comment: "initial comment"`.
3. Verify in the UI: Datasets > Permissions > Manage ACL Templates (or Credentials > ACL
   Templates, depending on build) — confirm the template appears in the user-defined list, not
   mixed into the builtin section.
4. Edit the config: change `comment` to `"updated comment"` and drop the `everyone@` entry (2
   entries left). `terraform apply`.
5. Re-run the query from step 2 — confirm `comment` updated and only 2 entries remain.
6. `terraform import truenas_acl_template.import_test <id-from-step-1>` against an empty resource
   block, `terraform plan` — confirm no diff (`ImportStateVerify`-equivalent).
7. `terraform destroy`.
8. Confirm via `midclt call filesystem.acltemplate.query '[["name","=","tf-acc-manual-
   acltemplate"]]'` that the template is gone, and re-run the builtin count query from
   Preconditions to confirm it is still 9 — the builtins were never touched.

**Expected results:** ordinary CRUD lifecycle; the only thing worth failing this test over besides
the usual create/update/import/destroy correctness is any change to the builtin template count or
contents.

**Cleanup:** none beyond step 7 (destroy already removes the only object created).

### 4. truenas_app

Both cases in this section require the Apps gate: set `TRUENAS_APPS=1` in your own head as a
reminder that this pulls container images from the network — plan for it to be slow (the
`syncthing` image pull can take a couple of minutes depending on the box's link).

**MT-APPS-006 — Catalog app (syncthing) install / stop / destroy lifecycle**

**Resource:** truenas_app
**Type:** Resource
**Environment:** 26.0 box (192.168.1.68) or 25.10 box (192.168.1.249) — Apps must already be
configured (a Docker pool assigned; see truenas_docker_config section for how to check). Apps gate:
this is a slow, network-pulling operation — budget several minutes.
**Preconditions:** Docker/Apps configured on the target box (`midclt call docker.config` shows a
non-null `pool`). No app named `tf-acc-manual-app` already installed.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = "REPLACE_WITH_API_KEY"
  insecure = true
}

resource "truenas_app" "test" {
  name        = "tf-acc-manual-app"
  catalog_app = "syncthing"
  running     = true
}
```

**Steps:**
1. `terraform apply`. This will take a while (image pull). Confirm `running = true`, `state` is
   set, and `version` is populated.
2. Verify in the UI: Apps > Installed Applications > `tf-acc-manual-app` shows status "Running".
3. Verify via API: `midclt call app.query '[["name","=","tf-acc-manual-app"]]'` — confirm
   `state: "RUNNING"`.
4. Edit the config: set `running = false`. `terraform apply`.
5. Confirm `state` reads `"STOPPED"` both in Terraform state and via the `app.query` check.
6. Verify in the UI that the app now shows "Stopped".
7. `terraform destroy`.
8. Confirm via `app.query` that no results are returned for `tf-acc-manual-app`, and that the app
   no longer appears in the UI's Installed Applications list.

**Expected results:** install succeeds and reaches RUNNING; stopping via `running = false` reaches
STOPPED without destroying the app; destroy fully removes it (containers and its dataset under the
ix-apps dataset).

**Cleanup:** step 7 already removes the app. If the apply in step 1 fails partway (e.g. a slow/
flaky image pull), manually clean up via Apps > Installed Applications > delete, or `midclt call
app.delete '"tf-acc-manual-app"'`, before re-running.

**MT-APPS-007 — App datasource lookup**

**Resource:** truenas_app (datasource)
**Type:** Data Source
**Environment:** same as MT-APPS-006.
**Preconditions:** same as MT-APPS-006.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = "REPLACE_WITH_API_KEY"
  insecure = true
}

resource "truenas_app" "test" {
  name        = "tf-acc-manual-app-ds"
  catalog_app = "syncthing"
  running     = true
}

data "truenas_app" "lookup" {
  name = truenas_app.test.name
}
```

**Steps:**
1. `terraform apply`.
2. `terraform state show data.truenas_app.lookup` — confirm `name` matches, `state` is populated,
   and note the datasource does **not** expose `values`/`custom_compose_config_string`/
   `catalog_app`/`running` (these are write-only or desired-state fields not echoed by a read-only
   lookup — their absence from the datasource schema is expected, not a missing feature).
3. `terraform destroy`.

**Expected results:** the datasource resolves to the same app the resource created, with only its
read-only fields populated.

**Cleanup:** step 3 removes the app.

### 5. truenas_app_registry

`app.registry.create` validates `username`/`password`/`uri` against the **real** registry endpoint
synchronously, before persisting anything — a throwaway/fabricated credential set is always
rejected. This is why the automated acceptance test is a permanent, documented skip. This is
therefore a **manual-only** test case: it requires a real, reachable container registry the tester
controls (e.g. a self-hosted `registry:2` instance, or genuine Docker Hub credentials).

**MT-APPS-008 — App registry CRUD against a real, reachable registry**

**Resource:** truenas_app_registry
**Type:** Resource
**Environment:** 26.0 box (192.168.1.68) or 25.10 box (192.168.1.249) with Apps/Docker configured
(this method rejects invalid credentials regardless of Docker being configured, but a fully
unconfigured box may fail earlier on an unrelated "No pool configured for Docker" error — configure
Docker's pool first if you hit that).
**Preconditions:** A real container registry reachable from the TrueNAS box, with real credentials
you control. Do not use fabricated credentials — they will be rejected outright.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = "REPLACE_WITH_API_KEY"
  insecure = true
}

resource "truenas_app_registry" "test" {
  name        = "tf-acc-manual-registry"
  description = "manual test registry entry"
  uri         = "https://REPLACE_WITH_YOUR_REGISTRY_HOST:5000"
  username    = "REPLACE_WITH_REAL_USERNAME"
  password    = "REPLACE_WITH_REAL_PASSWORD_OR_TOKEN"
}
```

**Steps:**
1. `terraform apply`. Because `password` is `Required` + `WriteOnly` + `Sensitive`, this requires
   Terraform >= 1.11 — confirm your CLI version first (`terraform version`).
2. Confirm the apply succeeds (a real, reachable registry with valid credentials is accepted) and
   `id`/`uri`/`username` are populated in state.
3. Verify via API: `midclt call app.registry.query '[["name","=","tf-acc-manual-registry"]]'` —
   confirm the entry exists; note the response's `password` field is masked, never the real
   secret.
4. Verify in the UI: Apps > Settings > Container Registries — confirm the entry appears.
5. Edit the config: change `description`. `terraform apply` — confirm it updates in place.
6. `terraform import truenas_app_registry.import_test <id-from-step-2>` against an empty resource
   block, then `terraform plan --generate-config-out=/tmp/gen.tf` or a manual
   `resource "truenas_app_registry" "import_test" { ... }` block that **omits** `password` (or add
   `password` to `ImportStateVerifyIgnore`-equivalent manual reasoning: the imported state's
   `password` will read as empty/unset since it is never stored — do not treat this as data loss;
   it is the intended write-only behavior). Confirm `terraform plan` shows no diff on
   `name`/`description`/`uri`/`username`.
7. `terraform destroy`.
8. Confirm via `app.registry.query` that the entry is gone.

**Expected results:** create/update/import/destroy behave normally against a real registry;
`password` is never visible in `terraform show`, `terraform state show`, or any `app.registry.*`
API response — this is intended, not a bug to report. If create is attempted with fabricated
credentials or an unreachable URI, expect a clean apply-time error containing `EINVAL` and "Invalid
credentials for registry" rather than an object being created — verify that too as a quick negative
check if you have a spare few minutes (point `uri` at `https://192.0.2.123:5000`, an RFC 5737
TEST-NET-1 address, with dummy credentials).

**Cleanup:** step 7 removes the registry entry. Revoke/rotate the real credentials used if they
were single-purpose for this test.

### 6. truenas_docker_config

`truenas_docker_config` is a **singleton** — there is one Docker configuration per system.
**Never set or change `pool` in any of these tests.** Changing it migrates the Docker dataset to a
different pool, which is a real data-migration operation, not a cosmetic toggle — there is no way
to safely undo it from a test script, and doing so on a box with real running apps can move or
orphan their data. All cases below only ever touch `enable_image_updates` or, where separately
noted, `nvidia`.

**MT-APPS-009 — enable_image_updates set-and-restore (Tier-2 toggle)**

**Resource:** truenas_docker_config
**Type:** Resource
**Environment:** 26.0 box (192.168.1.68) or 25.10 box (192.168.1.249).
**Preconditions:** Docker must already be configured on the target box
(`midclt call docker.config` shows a non-null `pool`) — if it is null, **do not** configure it via
this test; skip this case entirely on an unconfigured box (this mirrors the automated test's own
self-skip behavior).

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = "REPLACE_WITH_API_KEY"
  insecure = true
}

resource "truenas_docker_config" "test" {
  enable_image_updates = true # set to the OPPOSITE of whatever midclt call docker.config
                               # currently reports, see step 0 below
}
```

**Steps:**
0. Before touching Terraform: `midclt call docker.config` and note the current
   `enable_image_updates` value — you will restore it at the end.
1. Set the config's `enable_image_updates` to the opposite of the noted value. `terraform apply`.
2. Verify via API: `midclt call docker.config` — confirm `enable_image_updates` flipped, and that
   `pool`/`dataset` are **unchanged** from step 0.
3. Verify in the UI: Apps > Settings (gear icon) > Advanced Settings — confirm the "Update Apps
   Automatically" (or equivalently worded) toggle matches.
4. Edit the config back to the original value from step 0. `terraform apply`.
5. Re-verify via `docker.config` that it is back to the original value.
6. `terraform import truenas_docker_config.import_test docker_config` against an empty resource
   block, `terraform plan` — confirm no diff.
7. `terraform destroy` — confirm this only removes the resource from state (a warning is emitted:
   "Docker configuration left in place"), and `docker.config` afterward is completely unchanged
   from before this test started.

**Expected results:** only `enable_image_updates` ever changes on the box; `pool`/`dataset`/
`nvidia`/`address_pools`/`cidr_v6`/`registry_mirrors` are identical before and after this entire
test.

**Cleanup:** step 7's destroy plus step 4-5's restore should already leave the box as
found. Re-run `midclt call docker.config` one final time to confirm.

**MT-APPS-010 — nvidia writable on TrueNAS 25.10 (positive)**

**Resource:** truenas_docker_config
**Type:** Resource
**Environment:** 25.10 box (192.168.1.249) only — `nvidia` is writable on TrueNAS 25.10 and earlier.
Do not run this against the 26.0 box (see MT-APPS-011 for that box's behavior).
**Preconditions:** Docker configured on the 25.10 box (non-null `pool`).

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.249/api/current"
  api_key  = "REPLACE_WITH_API_KEY"
  insecure = true
}

resource "truenas_docker_config" "test" {
  nvidia = true # flip to the opposite of whatever midclt call docker.config currently reports
}
```

**Steps:**
0. `midclt call docker.config` on the 25.10 box — note the current `nvidia` value.
1. Set `nvidia` to the opposite value. `terraform apply` — confirm it **succeeds** (no apply-time
   error on this release).
2. Verify via `docker.config` that `nvidia` flipped.
3. Restore `nvidia` to its original value from step 0, `terraform apply`.
4. `terraform destroy` (state-only; warning expected, same as MT-APPS-009).

**Expected results:** setting `nvidia` explicitly succeeds on TrueNAS 25.10, confirming it remains
writable there per the resource's documented version split.

**Cleanup:** step 3's restore plus step 4's destroy.

**MT-APPS-011 — nvidia apply-time error on TrueNAS 26.0+ (negative)**

**Resource:** truenas_docker_config
**Type:** Resource
**Environment:** 26.0 box (192.168.1.68) only — TrueNAS 26.0+ dropped `nvidia` from
`docker.update`'s accepted fields, so setting it explicitly is an apply-time error there, even
though it is still readable.
**Preconditions:** Docker configured on the 26.0 box (non-null `pool`).

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = "REPLACE_WITH_API_KEY"
  insecure = true
}

resource "truenas_docker_config" "test" {
  nvidia = false
}
```

**Steps:**
1. `terraform apply` with `nvidia` explicitly set (either `true` or `false` — either counts as
   "explicitly set" since both are known, non-null values in the plan).
2. Observe the result.

**Expected results:** the apply **fails** with a clear error surfaced from `docker.update`
rejecting the `nvidia` field on this release (do not expect a silent no-op — the resource is
documented to raise a real apply-time error here, distinguishing "the user wrote this" from a
carried-forward computed value via `UseStateForUnknown`). Confirm no other fields were changed by
the failed apply (`midclt call docker.config` should be identical to before step 1). If instead the
apply silently succeeds and ignores `nvidia`, or if it errors while ALSO having mutated
`enable_image_updates`/`pool`/other fields, that is a defect — report it.

**Cleanup:** none needed if the apply fails as expected (nothing was changed). If for any reason
`enable_image_updates` did get toggled by a partially-applied request, restore it manually via
`midclt call docker.update '{"enable_image_updates": <original_value>}'`.

### 7. truenas_docker_network

`truenas_docker_network` is **datasource-only** (26.0+): TrueNAS exposes `docker.network.query`/
`get_instance` but no create/update/delete — Docker networks are owned by Docker/the apps
subsystem, not by this provider.

**MT-APPS-012 — Look up the built-in "bridge" Docker network**

**Resource:** truenas_docker_network (datasource)
**Type:** Data Source
**Environment:** 26.0 box (192.168.1.68). Docker must be configured (the "bridge" network only
exists once the Docker daemon is running).
**Preconditions:** `midclt call docker.config` shows a non-null `pool` on the target box.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = "REPLACE_WITH_API_KEY"
  insecure = true
}

data "truenas_docker_network" "test" {
  name = "bridge"
}
```

**Steps:**
1. `terraform apply` (a datasource-only config; `apply` just performs the read).
2. `terraform state show data.truenas_docker_network.test` — confirm `id`, `driver`, `scope` are
   populated and non-empty.
3. Cross-check via API: `midclt call docker.network.query '[["name","=","bridge"]]'` — confirm the
   `driver`/`scope`/`short_id` match what Terraform read.
4. Inspect `ipam.config` in the Terraform state — confirm it lists at least one subnet.

**Expected results:** the lookup succeeds and matches the API's own `docker.network.query` output
field-for-field. There is no destroy step — nothing was created.

**Cleanup:** none — this test creates no TrueNAS-side objects.

### 8. truenas_catalog_config

`truenas_catalog_config` is a singleton (like `docker_config`) but with a much lower blast radius:
`preferred_trains` only changes which trains are shown/preferred when browsing the app catalog UI —
it does not install, remove, start, or stop anything, so this Tier-2 toggle is judged safe to run
against a production-serving box.

**MT-APPS-013 — preferred_trains set-and-restore**

**Resource:** truenas_catalog_config
**Type:** Resource
**Environment:** either box (26.0 box 192.168.1.68 shown below). No Apps/Docker configuration
required — `catalog.config`/`catalog.update` work even with Docker completely unconfigured.
**Preconditions:** none beyond a working provider connection.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = "REPLACE_WITH_API_KEY"
  insecure = true
}

resource "truenas_catalog_config" "test" {
  preferred_trains = ["stable", "community"] # adjust based on step 0's findings
}
```

**Steps:**
0. `midclt call catalog.config` — note the current `preferred_trains` list; you will restore it.
1. Set the config's `preferred_trains` to a list that differs by at least one entry from step 0
   (e.g. drop or add `"community"`). `terraform apply`.
2. Verify via API: `midclt call catalog.config` — confirm `preferred_trains` matches the new list.
3. Verify in the UI: Apps > Discover Apps > train filter/settings — confirm the preferred trains
   shown match.
4. Also verify `label` and `location` are populated but were **not** and cannot be changed by this
   resource (there is no field for them in the config — confirm `terraform plan` never proposes
   changing them even after a fresh refresh).
5. Restore `preferred_trains` to the original list from step 0. `terraform apply`.
6. `terraform import truenas_catalog_config.import_test catalog_config` against an empty resource
   block, `terraform plan` — confirm no diff.
7. `terraform destroy` — confirm a warning ("Catalog configuration left in place") and that
   `catalog.config` afterward is unchanged from step 5's restored value.

**Expected results:** only `preferred_trains` changes; `label`/`location` are read-only throughout.

**Cleanup:** step 5's restore plus step 7's destroy already leave the box as found.

### 9. truenas_container

Requires **TrueNAS 26.0 or later** — the `container.*` namespace does not exist on 25.10.
Uses the `truenas_container_image` datasource to resolve a current image version rather than
hardcoding one: the upstream registry (images.linuxcontainers.org) **prunes old builds**, so a
version pinned today can 404 on download once pruned. Always resolve `latest_version` fresh at
apply time rather than copy-pasting a version string from a previous run.

**MT-APPS-014 — Full container lifecycle: create (stopped), update, start, stop, import, destroy**

**Resource:** truenas_container
**Type:** Resource
**Environment:** 26.0 box (192.168.1.68), pool tank. Requires TrueNAS 26.0+.
**Preconditions:** none beyond TrueNAS 26.0+ and a working provider connection. No pre-existing
container named `tf-acc-manual-container`.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = "REPLACE_WITH_API_KEY"
  insecure = true
}

data "truenas_container_image" "alpine" {
  name = "alpine:3.22:amd64:default"
}

resource "truenas_container" "test" {
  name        = "tf-acc-manual-container"
  pool        = "tank"
  description = "initial description"
  autostart   = false
  running     = false
  image = {
    name    = "alpine:3.22:amd64:default"
    version = data.truenas_container_image.alpine.latest_version
  }
}
```

**Steps:**
1. `terraform apply`. Confirm `data.truenas_container_image.alpine.latest_version` resolved to a
   non-empty string (do not hardcode this value into the resource's `image.version` — leave the
   datasource reference in place).
2. Confirm `truenas_container.test`: `id`/`dataset` are set, `autostart = false`, `running =
   false`, `status = "STOPPED"`.
3. Verify via API: `midclt call container.query '[["name","=","tf-acc-manual-container"]]'` —
   confirm `status.state: "STOPPED"` and `dataset` starts with `tank/`.
4. Verify in the UI: the containers/instances page shows `tf-acc-manual-container` as stopped.
5. Edit the config: change `description` to `"updated description"`, still `running = false`.
   `terraform apply` — confirm in-place update (no replacement), `description` updated.
6. Edit the config: set `running = true`. `terraform apply` — confirm `status` becomes `"RUNNING"`
   both in Terraform state and via `container.query`.
7. Verify in the UI that the container shows as running.
8. Edit the config: set `running = false` again. `terraform apply` — confirm it stops cleanly and
   `status` returns to `"STOPPED"`.
9. `terraform import truenas_container.import_test <id-from-step-2>` against an empty resource
   block, `terraform plan` — confirm no diff **except** on `image` (the API never echoes back
   `image`, so the imported resource's `image` is null — this is expected and documented; do not
   treat it as a bug, but do confirm `terraform plan` doesn't try to force-replace based on it
   alone in a real (non-empty) import target config).
10. `terraform destroy`.
11. Confirm via `container.query` that the container is gone, and that its backing dataset
    (`tank/.truenas_containers/containers/tf-acc-manual-container`) is also gone
    (`midclt call pool.dataset.query '[["name","=","tank/.truenas_containers/containers/tf-acc-
    manual-container"]]'` returns empty).

**Expected results:** standard resource lifecycle; `pool` and `image` are confirmed immutable
(changing either in a real scenario should force replacement — optionally verify this by editing
`pool` in a fresh throwaway config and confirming `terraform plan` shows "must be replaced" rather
than an in-place update, though this is not required for every run of this test).

**Cleanup:** step 10 already removes the container and its dataset.

### 10. truenas_container_device

Requires **TrueNAS 26.0 or later**. All three cases attach to a stopped
(`running = false`, `autostart = false`) parent `truenas_container` fixture so that attaching/
detaching devices only ever rewrites libvirt domain XML on disk — no live bind-mount, NIC
plug/unplug, or USB passthrough actually occurs while testing. **GPU devices are explicitly out of
scope**: no GPU-hardware-independent way to safely probe `dtype = "GPU"` exists (the API validates
`pci_address`/`gpu_type` against real host GPU inventory), so this provider's device attribute
shapes for GPU are unverified and not covered by any case here — if you have real GPU hardware to
test against, treat it as exploratory, not a pass/fail gate for this release.

**MT-APPS-015 — FILESYSTEM device: full CRUD**

**Resource:** truenas_container_device
**Type:** Resource
**Environment:** 26.0 box (192.168.1.68), pool tank. Requires TrueNAS 26.0+.
**Preconditions:** none beyond TrueNAS 26.0+.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = "REPLACE_WITH_API_KEY"
  insecure = true
}

data "truenas_container_image" "alpine" {
  name = "alpine:3.22:amd64:default"
}

resource "truenas_container" "test" {
  name      = "tf-acc-manual-ctrdev-fs"
  pool      = "tank"
  autostart = false
  running   = false
  image = {
    name    = "alpine:3.22:amd64:default"
    version = data.truenas_container_image.alpine.latest_version
  }
}

resource "truenas_dataset" "test" {
  name = "tank/tf-acc-manual-ds-ctrdev"
}

resource "truenas_container_device" "test" {
  container = truenas_container.test.id
  attributes = jsonencode({
    dtype  = "FILESYSTEM"
    source = truenas_dataset.test.mountpoint
    target = "/data"
  })
}
```

**Steps:**
1. `terraform apply`. Confirm `truenas_container_device.test.id` is set and
   `truenas_container_device.test.container` equals `truenas_container.test.id`.
2. Verify via API: `midclt call container.device.query '[["container","=",
   <container-id-from-step-1>]]'` — confirm one entry with
   `attributes.dtype: "FILESYSTEM"`, `attributes.target: "/data"`, and `attributes.source` matching
   the dataset's mountpoint.
3. **Note the known API quirk**: if `target` were omitted from the config, the live API applies a
   bogus `"/usr/bin/zsh"` literal instead of a sensible default — this is a confirmed server-side
   bug, not a provider defect; the config above deliberately always sets `target` explicitly to
   avoid it. Optionally reproduce this once by trying it in a throwaway config to see it firsthand.
4. Also confirm that omitting `source` entirely is rejected: try a throwaway apply without
   `source` and confirm it fails with `EINVAL` / "The path must reside within a pool mount point"
   — the API's own schema marks `source` optional, but it is required in practice.
5. Edit the config: change `target` to `"/data2"`. `terraform apply` — confirm in-place update.
6. Re-run the `container.device.query` check — confirm `target` updated to `/data2`.
7. `terraform import truenas_container_device.import_test <id-from-step-1>` against an empty
   resource block, `terraform plan` — confirm no diff.
8. `terraform destroy`.
9. Confirm via `container.device.query` that the device is gone, and via `container.query` that
   the parent container and dataset were also removed (they're in the same config).

**Expected results:** full CRUD works as documented; the `target`-omission quirk and
`source`-omission rejection are pre-existing server behaviors to be aware of, not provider bugs.

**Cleanup:** step 8 removes everything created (device, container, dataset are all in this config).

**MT-APPS-016 — NIC device: create, update, delete**

**Resource:** truenas_container_device
**Type:** Resource
**Environment:** 26.0 box (192.168.1.68), pool tank. Requires TrueNAS 26.0+.
**Preconditions:** run `midclt call container.device.nic_attach_choices` first and pick a real
bridge/interface name from the result (e.g. `truenasbr0`) — do not fabricate one.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = "REPLACE_WITH_API_KEY"
  insecure = true
}

data "truenas_container_image" "alpine" {
  name = "alpine:3.22:amd64:default"
}

resource "truenas_container" "test" {
  name      = "tf-acc-manual-ctrdev-nic"
  pool      = "tank"
  autostart = false
  running   = false
  image = {
    name    = "alpine:3.22:amd64:default"
    version = data.truenas_container_image.alpine.latest_version
  }
}

resource "truenas_container_device" "test" {
  container = truenas_container.test.id
  attributes = jsonencode({
    dtype                   = "NIC"
    nic_attach              = "truenasbr0" # replace with a real choice from nic_attach_choices
    type                    = "E1000"
    trust_guest_rx_filters  = false
  })
}
```

**Steps:**
1. `terraform apply`. Confirm the device is created with `dtype: "NIC"`.
2. Verify via API: `midclt call container.device.query '[["container","=",
   <container-id>]]'` — confirm `attributes.nic_attach` matches, `attributes.type: "E1000"`, and
   `attributes.mac` is auto-generated (non-null) even though it was not set in config.
3. Edit the config: change `type` to `"VIRTIO"`. `terraform apply`. **This exercises update
   behavior beyond what the automated suite covers** (its NIC probe evidence is create/delete
   only) — pay close attention here. Confirm whether the update succeeds cleanly in place, and
   record the result either way; if it instead fails or forces a replacement, that is new
   information worth reporting back (not necessarily a bug, but undocumented behavior to capture).
4. Re-check via `container.device.query` that `attributes.type` reflects the change (if step 3
   succeeded).
5. `terraform destroy`.
6. Confirm via `container.device.query`/`container.query` that both the device and its parent
   container are gone.

**Expected results:** create/delete work as documented (probed live); the update step (3-4) is the
main thing this manual case adds beyond the automated coverage — document its actual outcome.

**Cleanup:** step 5 removes the device and container together.

**MT-APPS-017 — USB device: create, update, delete**

**Resource:** truenas_container_device
**Type:** Resource
**Environment:** 26.0 box (192.168.1.68), pool tank. Requires TrueNAS 26.0+ **and** at least one
real USB device physically attached to the box (the API validates `vendor_id`/`product_id` against
real host hardware — fabricated values are rejected).
**Preconditions:** run `midclt call container.device.usb_choices` and pick a real device's
`vendor_id`/`product_id` (hex strings, e.g. `"0x046b"`/`"0xff10"`) from the result.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = "REPLACE_WITH_API_KEY"
  insecure = true
}

data "truenas_container_image" "alpine" {
  name = "alpine:3.22:amd64:default"
}

resource "truenas_container" "test" {
  name      = "tf-acc-manual-ctrdev-usb"
  pool      = "tank"
  autostart = false
  running   = false
  image = {
    name    = "alpine:3.22:amd64:default"
    version = data.truenas_container_image.alpine.latest_version
  }
}

resource "truenas_container_device" "test" {
  container = truenas_container.test.id
  attributes = jsonencode({
    dtype = "USB"
    usb = {
      vendor_id  = "0x046b"  # replace with a real vendor_id from usb_choices
      product_id = "0xff10"  # replace with a real product_id from usb_choices
    }
  })
}
```

**Steps:**
1. `terraform apply`. Confirm the device is created with `dtype: "USB"`.
2. Verify via API: `midclt call container.device.query '[["container","=",
   <container-id>]]'` — confirm `attributes.usb.vendor_id`/`product_id` match.
3. Negative check: confirm that omitting both `device` and `usb` is rejected — try a throwaway
   apply with `attributes = jsonencode({ dtype = "USB" })` and confirm it fails with `EINVAL` /
   "Either device or product_id and vendor_id must be specified" (matches the FILESYSTEM
   `source`-omission pattern in MT-APPS-015: the API's schema marks these optional, but real usage
   requires one of them).
4. Edit the config: try swapping to a different real USB device's `vendor_id`/`product_id` (if a
   second one is available), or attempt an update with the same values to confirm idempotency.
   **This exercises update behavior beyond the automated suite's create/delete-only USB probe** —
   record the actual outcome.
5. `terraform destroy`.
6. Confirm via `container.device.query`/`container.query` that both the device and its parent
   container are gone.

**Expected results:** create/delete work as documented; step 3's negative check should fail
cleanly; step 4's update behavior is new ground for this manual case to document.

**Cleanup:** step 5 removes the device and container together.

### 11. truenas_container_image

Requires **TrueNAS 26.0 or later**. Datasource-only — the upstream LXC image registry is not
managed by this provider.

**MT-APPS-018 — Look up an existing image and resolve latest_version**

**Resource:** truenas_container_image (datasource)
**Type:** Data Source
**Environment:** 26.0 box (192.168.1.68). Requires TrueNAS 26.0+.
**Preconditions:** none.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = "REPLACE_WITH_API_KEY"
  insecure = true
}

data "truenas_container_image" "alpine" {
  name = "alpine:3.22:amd64:default"
}
```

**Steps:**
1. `terraform apply`.
2. `terraform state show data.truenas_container_image.alpine` — confirm `id` equals `name`
   (`"alpine:3.22:amd64:default"`), `versions` is a non-empty list, and `latest_version` equals the
   **last** element of `versions` (oldest-to-newest by build timestamp — confirm the ordering looks
   chronological, e.g. `20260718_13:00` < `20260719_13:00` < `20260722_17:16`-style strings).
3. Cross-check: `midclt call container.image.query_registry` (no arguments — the method rejects any
   filter argument) and manually find the `alpine:3.22:amd64:default` entry in the full 60+ entry
   result; confirm its `versions` array matches what Terraform read.
4. Re-run this specific test a day or more later (or any time after this initial run) and confirm
   `latest_version` may have changed — the upstream registry prunes and adds builds continuously,
   so a fixed expected value is **not** meaningful here; the assertion is "well-formed, non-empty,
   and equal to the last list entry", not "equals some specific string".

**Expected results:** the datasource always resolves without hardcoding, and `latest_version` is
consistently the newest (last) entry.

**Cleanup:** none — this test creates no TrueNAS-side objects.

**MT-APPS-019 — Not-found negative test**

**Resource:** truenas_container_image (datasource)
**Type:** Data Source
**Environment:** 26.0 box (192.168.1.68). Requires TrueNAS 26.0+.
**Preconditions:** none.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = "REPLACE_WITH_API_KEY"
  insecure = true
}

data "truenas_container_image" "missing" {
  name = "this-image-does-not-exist:0:amd64:default"
}
```

**Steps:**
1. `terraform apply` (or `terraform plan`, since this will fail during the datasource read either
   way).
2. Observe the error.

**Expected results:** a clean error diagnostic containing "not found" (case-insensitive) — not a
crash, panic, or raw unhandled API error/stack trace.

**Cleanup:** none — nothing is created regardless of outcome.

### 12. truenas_lxc_config

`truenas_lxc_config` is a singleton (like `docker_config`/`catalog_config`), requires **TrueNAS
TrueNAS 26.0 or later**, and follows the same `pool`-is-dangerous rule as `docker_config`: changing
`preferred_pool` selects which ZFS pool LXC uses for instance/image datasets, which is a real
migration concern on a box already using LXC, not a cosmetic toggle.

**MT-APPS-020 — v4_network set-and-restore (Tier-2 toggle, preferred_pool untouched)**

**Resource:** truenas_lxc_config
**Type:** Resource
**Environment:** 26.0 box (192.168.1.68). Requires TrueNAS 26.0+.
**Preconditions:** `midclt call lxc.config` shows `preferred_pool: null` on the target box (LXC not
yet in active use there). If `preferred_pool` is already set, **skip this case** — do not run a
network-CIDR change against a box with LXC actively configured without separately confirming it's
safe; this manual case is scoped the same way the automated test self-skips.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = "REPLACE_WITH_API_KEY"
  insecure = true
}

resource "truenas_lxc_config" "test" {
  v4_network = "172.201.0.0/24" # pick something different from the current value; see step 0
}
```

**Steps:**
0. `midclt call lxc.config` — note the current `v4_network` (and re-confirm `preferred_pool` is
   still `null`).
1. Set `v4_network` in the config to a value different from step 0's (e.g. `172.201.0.0/24`, or
   `172.202.0.0/24` if that happens to already be current). `terraform apply`.
2. Verify via API: `midclt call lxc.config` — confirm `v4_network` updated and `preferred_pool` is
   still `null` (this resource never touches it when the field is left out of the config).
3. Verify in the UI: the LXC/Instances network settings page shows the updated CIDR.
4. Restore `v4_network` to the original value from step 0. `terraform apply`.
5. `terraform import truenas_lxc_config.import_test lxc_config` against an empty resource block,
   `terraform plan` — confirm no diff.
6. `terraform destroy` — confirm a warning ("LXC configuration left in place") and that
   `lxc.config` afterward is unchanged from step 4's restored value.

**Expected results:** only `v4_network` changes; `preferred_pool`/`bridge`/`v6_network` remain
as found, throughout.

**Cleanup:** step 4's restore plus step 6's destroy already leave the box as found.

**MT-APPS-021 — preferred_pool change (cautionary, manual-only)**

This case deliberately does what MT-APPS-020 and the automated suite both refuse to do: set
`preferred_pool`. Only run it on a box where you have explicitly confirmed it is acceptable to
change where LXC instances/images land — this is a real migration-adjacent operation, not a
cosmetic setting, and is why it is manual-only with no committed automated equivalent.

**Resource:** truenas_lxc_config
**Type:** Resource
**Environment:** 26.0 box (192.168.1.68), or a dedicated/disposable test box — **do not** run this
against a box with LXC instances you care about without understanding the consequences first.
**Preconditions:** Explicit sign-off that changing `preferred_pool` on this box is acceptable.
Note the current value first: `midclt call lxc.config`.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  api_key  = "REPLACE_WITH_API_KEY"
  insecure = true
}

resource "truenas_lxc_config" "test" {
  preferred_pool = "tank" # WARNING: this changes where new LXC instances/image datasets are
                            # created going forward; read the case description above before
                            # running this
}
```

**Steps:**
1. `terraform apply`.
2. Verify via API: `midclt call lxc.config` — confirm `preferred_pool` is now `"tank"` (or whatever
   value you set).
3. Verify in the UI: the LXC/Instances settings page shows the new pool selection.
4. Confirm (do not just assume) that no existing LXC instances were moved, deleted, or corrupted by
   this change — `preferred_pool` governs where **new** instances land, not a migration of
   existing ones; if the box had any LXC instances before this test, check they are still present
   and functional via `midclt call container.query` and/or the UI.
5. If you noted an original `preferred_pool` value in Preconditions that differs from what you set,
   restore it now: edit the config back and `terraform apply`.
6. `terraform destroy` (state-only; warning expected).

**Expected results:** `preferred_pool` changes as configured; nothing else on the box is
silently migrated or destroyed as a side effect. If step 4 turns up any unexpected instance
disruption, treat that as a serious finding, not a minor one.

**Cleanup:** step 5's restore (if applicable) plus step 6's destroy. Confirm final state with one
more `midclt call lxc.config`.

---

## Network, System, Services, and Alerts

These are manual test cases: a human runs `terraform apply`/`import`/`destroy` from the CLI and cross-checks results against the TrueNAS UI and `midclt call <method>` on the box itself. None of this runs through the Go acceptance-test harness (`go test`, `TF_ACC=1`). Some resources control the network path or the management UI/SSH session the tester is using — those cases are marked **Manual-only DANGEROUS** and must only be run against a disposable TrueNAS instance you are prepared to lose remote access to (console/IPMI access required as a fallback).

Conventions used below:
- Resource/value names use the `tf-acc-manual-` prefix (distinct from the automated suite's `tf-acc-` prefix) so artifacts left behind by a human tester are easy to identify and clean up.
- "TEST-NET" addresses are the RFC 5737 documentation ranges: `192.0.2.0/24` (TEST-NET-1), `198.51.100.0/24` (TEST-NET-2), `203.0.113.0/24` (TEST-NET-3). These are guaranteed never to be routable, so they're safe to reference from config without risk of contacting a real host.
- Every `Config:` block includes the provider block. Set `TRUENAS_ENDPOINT` and `TRUENAS_API_KEY` in the environment before running `terraform apply` (or replace the endpoint literal), for example:
  ```
  export TRUENAS_ENDPOINT="wss://192.168.1.68/api/current"
  export TRUENAS_API_KEY="<api key>"
  ```
- "Singleton" resources (`network_config`, `system_general`, `system_advanced`, `ssh_config`, `ftp_config`, `snmp_config`, `ups_config`, `mail`, `alert_policy`, `audit_config`) always exist on the box — Terraform "creates" one by adopting the current config, and `terraform destroy` on most of them makes **no API call at all**: it only removes Terraform state and leaves the live TrueNAS configuration untouched. Where that's true it is called out explicitly in "Expected results" so a tester doesn't mistake it for a bug.
- "Tier-2 disruptive" singleton cases mutate box-wide, non-idempotent state (mail routing, SNMP location string, audit retention, etc.) and follow a set-and-restore pattern: read the current value first, apply a test value, verify, apply the original value back, verify restored, then import/destroy.

### 1. truenas_network_config

**MT-SYSTEM-001 — Datasource read of the live network configuration**

**Resource:** truenas_network_config
**Type:** Datasource
**Environment:** Any TrueNAS box (read-only, safe on production).
**Preconditions:** TrueNAS reachable over the WebSocket API; API key or username/password available.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

data "truenas_network_config" "current" {}

output "hostname" {
  value = data.truenas_network_config.current.hostname
}

output "ipv4gateway" {
  value = data.truenas_network_config.current.ipv4gateway
}
```

**Steps:**
1. `terraform init`
2. `terraform apply -auto-approve`
3. `terraform output hostname` and `terraform output ipv4gateway` — note the values.
4. On the TrueNAS host (or via SSH), run `midclt call network.configuration.config` and confirm `hostname` and `ipv4gateway` match the Terraform outputs.
5. In the UI, go to **Network** and confirm the hostname/gateway shown there also match.
6. `terraform destroy -auto-approve`.

**Expected results:** `terraform apply` succeeds with no diff on a second `terraform plan` (idempotent read). Datasource values match both `midclt call network.configuration.config` and the UI. Destroy removes only the datasource state (there is nothing to "restore" for a datasource).

**Cleanup:** None — read-only.

---

**MT-SYSTEM-002 — Manual-only DANGEROUS: network_config singleton set-and-restore**

**Resource:** truenas_network_config
**Type:** Manual-only DANGEROUS
**Environment:** Disposable TrueNAS instance ONLY (VM/scratch box you can reinstall or reach via console/IPMI). Never run against a production or shared box.
**Preconditions:** Console or IPMI/BMC access to the box, independent of the network path Terraform will use, is available and tested working BEFORE starting. `midclt call network.configuration.config` succeeds and its output has been saved to a file for manual comparison.

**⚠ Safety:** `hostname`, `ipv4gateway`, `ipv6gateway`, and `nameserver1/2/3` on this resource directly control how the box is reached on the network and how it resolves names. A wrong `ipv4gateway`/`ipv6gateway` immediately cuts outbound/inbound routed connectivity to the box (including the Terraform runner's own connection, unless it happens to sit on the same L2 segment). A wrong `hostname` can break certificate SAN matching, AD/LDAP binding, and any service keyed on hostname. This test therefore only ever touches `httpproxy` (a cosmetic field with no connectivity impact) — do not extend it to touch the four risky fields above without console/IPMI access as a fallback. Recovery if you do lock yourself out: use console/IPMI to log in locally and run `midclt call network.configuration.update '{"ipv4gateway": "<known-good-gateway>"}'` (or the equivalent UI screen) to restore connectivity, then re-run `terraform plan` to reconcile state.

**Config (step A — read baseline, no changes):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

data "truenas_network_config" "current" {}

output "orig_httpproxy" {
  value = data.truenas_network_config.current.httpproxy
}
```

**Config (step B — set test value):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_network_config" "test" {
  httpproxy = "http://198.51.100.10:3128"
}
```

**Config (step C — restore, replace the httpproxy value with the ORIGINAL value read in step A, e.g.):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_network_config" "test" {
  httpproxy = ""
}
```

**Steps:**
1. Apply step A's config; record `orig_httpproxy` (this is the value you must restore later — commonly `""`).
2. Apply step B's config. `terraform plan` should show `httpproxy` changing to `http://198.51.100.10:3128` and nothing else.
3. Verify: `midclt call network.configuration.config` shows the new `httpproxy`; UI **Network > Global Configuration** shows the same value. Confirm you can still reach the box (SSH/UI) — if you cannot, use console/IPMI immediately.
4. Apply step C's config with `httpproxy` set back to the value recorded in step 1.
5. Verify `midclt call network.configuration.config` shows `httpproxy` restored.
6. `terraform import truenas_network_config.test network_config` (any string works as the import ID — it normalizes to `"network_config"`); confirm `terraform plan` shows no diff afterward.
7. `terraform destroy -auto-approve`.

**Expected results:** Only `httpproxy` changes on the box at any point; `hostname`/gateways/nameservers are never touched by this case. Destroy makes no API call — a follow-up `midclt call network.configuration.config` after destroy shows the box's config is unchanged (still the restored `httpproxy`), confirming `terraform destroy` on this singleton only forgets Terraform state.

**Cleanup:** Confirm `httpproxy` on the box matches the pre-test baseline from step 1.

---

### 2. truenas_network_interface

**MT-SYSTEM-003 — Datasource read of a physical interface (enp7s0)**

**Resource:** truenas_network_interface
**Type:** Datasource
**Environment:** Any TrueNAS box that has a NIC literally named `enp7s0` (check first — see Preconditions). Safe on production.
**Preconditions:** Run `midclt call interface.query '[["id","=","enp7s0"]]'` first. If it returns an empty array, this specific case does not apply to your box — substitute any `PHYSICAL` interface name returned by `midclt call interface.query '[["type","=","PHYSICAL"]]'` in its place.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

data "truenas_network_interface" "enp7s0" {
  name = "enp7s0"
}

output "iface_type" {
  value = data.truenas_network_interface.enp7s0.type
}
```

**Steps:**
1. `terraform init && terraform apply -auto-approve`.
2. Confirm `terraform output iface_type` is `"PHYSICAL"`.
3. Cross-check `midclt call interface.get_instance enp7s0` and the UI **Network > Interfaces** listing for `enp7s0` show the same MTU/type/aliases as the datasource attributes.
4. `terraform destroy -auto-approve`.

**Expected results:** Read-only lookup succeeds with no drift on repeated `terraform plan`. No commit/checkin cycle is triggered (datasource reads never stage or commit interface changes).

**Cleanup:** None — read-only.

---

**MT-SYSTEM-004 — Manual-only DANGEROUS: BRIDGE interface create/commit/checkin/destroy**

**Resource:** truenas_network_interface
**Type:** Manual-only DANGEROUS
**Environment:** Disposable TrueNAS instance ONLY. Console/IPMI access required as a fallback.
**Preconditions:** Console or IPMI access confirmed working. No interface named `br199` already exists (`midclt call interface.query '[["id","=","br199"]]'` returns `[]`). Understand that TrueNAS stages interface changes globally — a pending/uncommitted change from ANY interface can affect ALL interfaces on the box, not just the one this test manages.

**⚠ Safety:** `interface.create`/`interface.update`/`interface.delete` only *stage* a network change; the change only takes effect when `interface.commit` runs, which this resource issues automatically after every Create/Update/Delete with `checkin_timeout: 60` and `rollback: true`. That arms a 60-second auto-rollback timer: if `interface.checkin` (also issued automatically, over the same WebSocket connection) doesn't complete within 60 seconds, TrueNAS reverts the staged change on its own. This protects you IF the broken change severs the specific connection Terraform is using — but if the change breaks a *different* path than the one currently connected, checkin can still succeed and the bad change stays committed. If `interface.commit` itself errors, the provider calls `interface.rollback`; if that also fails, a staged change can be left pending on the box, requiring manual intervention. **Recovery:** (1) wait 60 seconds for the box's own auto-rollback if checkin is failing; (2) if you're still locked out after that, use console/IPMI to log in locally and check **Network** for a pending-changes/rollback banner, or run `midclt call interface.rollback` directly at the console; (3) as a last resort, reboot the box — staged-but-uncommitted interface changes do not survive a reboot.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_network_interface" "test" {
  name = "br199"
  type = "BRIDGE"
}
```

**Steps:**
1. `terraform init && terraform apply -auto-approve`.
2. Watch the apply for errors mentioning "commit applied but checkin failed" — if you see that message, DO NOT walk away; monitor whether the box reachable within 60 seconds.
3. Verify in UI **Network > Interfaces** that `br199` exists with type BRIDGE and no pending-changes banner is shown. Verify via `midclt call interface.get_instance br199`.
4. Confirm no OTHER interface shows a pending/uncommitted state (`midclt call interface.query` and scan for any `state` fields indicating pending changes).
5. `terraform import truenas_network_interface.test br199`; confirm `terraform plan` shows no diff.
6. `terraform destroy -auto-approve`.
7. Verify via `midclt call interface.query '[["id","=","br199"]]'` that the bridge is gone, and that the commit/checkin cycle for the delete also completed cleanly (no lingering pending-changes banner in the UI).

**Expected results:** Bridge is created, committed, and checked in within the 60-second window on both create and destroy. No other interface on the box is affected. If reachability is lost at any point, the box self-reverts within 60 seconds without operator action.

**Cleanup:** Confirm `br199` does not exist (`midclt call interface.query '[["id","=","br199"]]'` returns `[]`) and that no pending network changes remain (UI shows no rollback banner).

---

### 3. truenas_static_route

**MT-SYSTEM-005 — CRUD lifecycle: create, update, import, destroy a static route**

**Resource:** truenas_static_route
**Type:** CRUD
**Environment:** Any TrueNAS box. Safe — the destination uses a non-routable documentation range so nothing in production traffic is affected.
**Preconditions:** None beyond API reachability. The `gateway` value below (`192.168.1.1`) must be an address TrueNAS considers reachable/plausible from one of its local interfaces — substitute an address on your own box's LAN subnet if `192.168.1.0/24` isn't in use on your test host. (Unlike `destination`, `gateway` is validated against local reachability by TrueNAS, so a TEST-NET address here will typically be rejected.)

**Config (create):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_static_route" "test" {
  destination = "198.51.100.0/24"
  gateway     = "192.168.1.1"
  description = "tf-acc-manual-route-01"
}
```

**Config (update — description only):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_static_route" "test" {
  destination = "198.51.100.0/24"
  gateway     = "192.168.1.1"
  description = "tf-acc-manual-route-01-updated"
}
```

**Steps:**
1. `terraform init && terraform apply -auto-approve` with the create config.
2. Verify: `midclt call staticroute.query '[["destination","=","198.51.100.0/24"]]'` returns one record with matching `gateway`/`description`; UI **Network > Static Routes** shows the same route.
3. `terraform apply -auto-approve` with the update config (description change only).
4. Verify the description changed on the box without the route being deleted/recreated (the numeric `id` in `terraform show` state should be unchanged).
5. `terraform import truenas_static_route.imported <id>` (use the numeric ID from step 2 or `terraform state show truenas_static_route.test`); confirm `terraform plan` shows no diff for the imported resource.
6. `terraform destroy -auto-approve`.
7. Verify `midclt call staticroute.query '[["destination","=","198.51.100.0/24"]]'` returns `[]`.

**Expected results:** Route created, updated in place (no destroy/recreate), imported cleanly, and fully removed on destroy — this is a true CRUD resource, unlike the singletons elsewhere in this section.

**Cleanup:** Confirm no leftover route matching `198.51.100.0/24` remains (`midclt call staticroute.query '[["destination","=","198.51.100.0/24"]]'` returns `[]`).

---

### 4. truenas_system_general

**MT-SYSTEM-006 — Datasource read of system_general**

**Resource:** truenas_system_general
**Type:** Datasource
**Environment:** Any TrueNAS box. Safe on production.
**Preconditions:** None.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

data "truenas_system_general" "current" {}

output "timezone" {
  value = data.truenas_system_general.current.timezone
}
```

**Steps:**
1. `terraform init && terraform apply -auto-approve`.
2. Confirm `terraform output timezone` matches **System Settings > General** in the UI and `midclt call system.general.config`.
3. `terraform destroy -auto-approve`.

**Expected results:** Read-only, idempotent, no drift on repeated plans.

**Cleanup:** None.

---

**MT-SYSTEM-007 — Manual-only DANGEROUS: system_general singleton set-and-restore**

**Resource:** truenas_system_general
**Type:** Manual-only DANGEROUS
**Environment:** Disposable TrueNAS instance ONLY. Console/IPMI access required as a fallback.
**Preconditions:** Console/IPMI access confirmed working. `midclt call system.general.config` output saved for comparison.

**⚠ Safety:** `ui_address`, `ui_allowlist`, `ui_port`, `ui_httpsport`, and `ui_v6address` on this resource control how (and whether) the management UI/API can be reached over the network. Changing any of them incorrectly — e.g. binding the UI to an address that doesn't exist on the box, or setting an allowlist that excludes your own subnet — can immediately cut off management access, INCLUDING the WebSocket connection Terraform itself uses, meaning the apply may never even report success/failure cleanly. This case only ever touches `ui_consolemsg` (a cosmetic login-screen toggle with zero network impact). Do not extend it to the fields above without console/IPMI access as a fallback. Recovery if locked out: use console/IPMI to log in locally and either revert via the local TUI/console menu, or run `midclt call system.general.update '{"ui_port": 80}'` (or whatever field needs fixing) directly at the console.

**Config (step A — read baseline):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

data "truenas_system_general" "current" {}

output "orig_ui_consolemsg" {
  value = data.truenas_system_general.current.ui_consolemsg
}
```

**Config (step B — set test value, e.g. baseline was `false`):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_system_general" "test" {
  ui_consolemsg = true
}
```

**Config (step C — restore to the baseline recorded in step A):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_system_general" "test" {
  ui_consolemsg = false
}
```

**Steps:**
1. Apply step A; record `orig_ui_consolemsg`.
2. Apply step B. `terraform plan` should show ONLY `ui_consolemsg` changing.
3. Verify `midclt call system.general.config` shows the new value; confirm management UI/API access is still intact.
4. Apply step C with the value from step 1.
5. Verify restored via `midclt call system.general.config`.
6. `terraform import truenas_system_general.test system_general` (any import ID normalizes to `"system_general"`); confirm no diff.
7. `terraform destroy -auto-approve`.

**Expected results:** Only `ui_consolemsg` changes at any point. Destroy makes no API call — `midclt call system.general.config` after destroy still shows the restored value, confirming destroy only forgets Terraform state.

**Cleanup:** Confirm `ui_consolemsg` on the box matches the step-1 baseline.

---

### 5. truenas_system_advanced

**MT-SYSTEM-008 — Datasource read of system_advanced**

**Resource:** truenas_system_advanced
**Type:** Datasource
**Environment:** Any TrueNAS box. Safe on production.
**Preconditions:** None.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

data "truenas_system_advanced" "current" {}

output "sysloglevel" {
  value = data.truenas_system_advanced.current.sysloglevel
}
```

**Steps:**
1. `terraform init && terraform apply -auto-approve`.
2. Confirm `terraform output sysloglevel` matches `midclt call system.advanced.config` and **System Settings > Advanced** in the UI.
3. `terraform destroy -auto-approve`.

**Expected results:** Read-only, idempotent.

**Cleanup:** None.

---

**MT-SYSTEM-009 — Singleton set-and-restore (DISRUPTIVE): system_advanced motd**

**Resource:** truenas_system_advanced
**Type:** Singleton
**Environment:** Non-production strongly recommended (this mutates a box-wide setting, though the field used here — `motd` — is cosmetic). Full DisruptiveCheck equivalent: treat this as you would a `TRUENAS_DISRUPTIVE=1` automated run.
**Preconditions:** `midclt call system.advanced.config` succeeds; note the current `motd` value.

**⚠ Safety:** This case only touches `motd` (the login-screen message of the day). It deliberately never touches `syslogservers`, `serialconsole`, `serialport`, `serialspeed`, `consolemenu`, `sed_user`, or `sed_passwd` — changing those on a live system can disrupt console/serial access or SED (self-encrypting drive) unlock behavior. Do not extend this case to those fields on a shared box.

**Config (step A — read baseline):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

data "truenas_system_advanced" "current" {}

output "orig_motd" {
  value = data.truenas_system_advanced.current.motd
}
```

**Config (step B — set test value):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_system_advanced" "test" {
  motd = "tf-acc-manual-motd-01"
}
```

**Config (step C — restore, using the value recorded in step A):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_system_advanced" "test" {
  motd = "<orig_motd value from step A>"
}
```

**Steps:**
1. Apply step A; record `orig_motd`.
2. Apply step B; verify `midclt call system.advanced.config` shows `motd` = `"tf-acc-manual-motd-01"` and the UI login screen / **System Settings > Advanced** reflects it.
3. Apply step C with the original value substituted in.
4. Verify restored via `midclt call system.advanced.config`.
5. `terraform import truenas_system_advanced.test system_advanced`; confirm no diff (state should show `sed_passwd` empty/null since it's write-only and never read back).
6. `terraform destroy -auto-approve`.

**Expected results:** Only `motd` changes. Destroy makes no API call — the restored `motd` persists on the box after destroy.

**Cleanup:** Confirm `motd` on the box matches the step-1 baseline.

---

### 6. truenas_tunable

**MT-SYSTEM-010 — CRUD: SYSCTL tunable set to its current value (zero behavior change), verify orig_value capture and restore**

**Resource:** truenas_tunable
**Type:** CRUD
**Environment:** Any TrueNAS box.
**Preconditions:** Read the current live value of the sysctl BEFORE creating the resource: `midclt call system.info.system_manufacturer` is not it — instead run, at the console/SSH, `sysctl fs.suid_dumpable` (or `midclt call tunable.query '[["var","=","fs.suid_dumpable"]]'` if already managed elsewhere) and record the value. No `truenas_tunable` resource for `fs.suid_dumpable` should already exist (`midclt call tunable.query '[["var","=","fs.suid_dumpable"]]'` returns `[]`).

**Config (use the CURRENT value found in Preconditions as `value`, e.g. `"0"`):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_tunable" "test" {
  var     = "fs.suid_dumpable"
  value   = "0"
  type    = "SYSCTL"
  comment = "tf-acc-manual-tunable-01"
  enabled = true
}
```

**Steps:**
1. `terraform init && terraform apply -auto-approve`.
2. Run `terraform state show truenas_tunable.test` and confirm `orig_value` equals the value you recorded in Preconditions (TrueNAS captures the pre-existing value server-side at creation time — this is NOT something Terraform itself manages).
3. Confirm the live sysctl value is unchanged (`sysctl fs.suid_dumpable` still shows the same value, since you set `value` to match the current value — zero behavior change).
4. Verify in UI **System Settings > Tunables** the entry appears with the same var/value/comment.
5. Update `comment` to `"tf-acc-manual-tunable-01-updated"` and re-apply; confirm `value`/`var`/`type` are unchanged (in-place update, `id` unchanged).
6. `terraform import truenas_tunable.imported <id>` (numeric id from `terraform state show`); confirm no diff.
7. `terraform destroy -auto-approve`.
8. Run `sysctl fs.suid_dumpable` again and confirm it still shows the ORIGINAL value from Preconditions — TrueNAS's `tunable.delete` restores `orig_value` automatically on destroy.

**Expected results:** `orig_value` in state matches the pre-existing kernel value from Preconditions. The live sysctl value never actually changes (since the test sets `value` equal to the current value). On destroy, TrueNAS's own restore-on-delete behavior (not any Terraform-side bookkeeping) puts `orig_value` back, and the sysctl value after destroy still matches the Preconditions baseline.

**Cleanup:** Confirm `midclt call tunable.query '[["var","=","fs.suid_dumpable"]]'` returns `[]` and `sysctl fs.suid_dumpable` matches the original baseline.

---

### 7. truenas_boot_environment

**MT-SYSTEM-011 — CRUD: clone the active boot environment, toggle keep, import, destroy (NEVER activate)**

**Resource:** truenas_boot_environment
**Type:** CRUD
**Environment:** Any TrueNAS box with sufficient boot-pool free space for a BE clone.
**Preconditions:** Identify the currently-active boot environment: `midclt call boot.environment.query '[["active","=",true]]'` and note its `id` — this is the `source` for the clone. Confirm enough boot-pool space exists (**System > Boot** in the UI shows usage).

**⚠ Safety:** NEVER set `activated = true` in this test, and never call `midclt call boot.environment.activate` against the clone created here. Activating a boot environment changes what the box boots into on next reboot — an unintended activation on a production box could leave it booting into an unexpected, possibly non-functional environment. This resource also has no "deactivate" operation: once activated, the only way to undo it in Terraform is to activate a DIFFERENT boot environment, which is out of scope for this case. Additionally, this resource refuses to destroy any boot environment that is currently active or activated (a provider-side guard, independent of the TrueNAS API) — so if you accidentally activate the clone, `terraform destroy` will hard-fail with an explicit error until you activate a different BE first.

**Config (create — replace `source` with the value found in Preconditions):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_boot_environment" "test" {
  name   = "tf-acc-manual-be01"
  source = "<active BE id from Preconditions>"
  keep   = true
}

data "truenas_boot_environment" "lookup" {
  name = truenas_boot_environment.test.name
}

output "clone_dataset" {
  value = data.truenas_boot_environment.lookup.dataset
}
```

**Steps:**
1. `terraform init && terraform apply -auto-approve`.
2. Verify: `name = "tf-acc-manual-be01"`, `source` matches the active BE, `id = "tf-acc-manual-be01"`, `keep = "true"`, `active = "false"` (the clone is NOT the booted BE), `dataset` and `used_bytes` are set. Confirm the datasource `clone_dataset` output is populated too.
3. Cross-check UI **System > Boot** shows `tf-acc-manual-be01` in the list, NOT marked active/activated.
4. Update `keep = false` and re-apply; confirm only `keep` changed.
5. `terraform import truenas_boot_environment.imported tf-acc-manual-be01`; confirm `terraform plan` shows a diff ONLY on `source` (source is client-side only and can't be recovered by import — this is expected, not a bug).
6. `terraform destroy -auto-approve`.
7. Verify `midclt call boot.environment.query '[["id","=","tf-acc-manual-be01"]]'` returns `[]`, and that the ORIGINAL active boot environment (from Preconditions) is still present and still active/unaffected.

**Expected results:** Clone created without touching the source BE's active/activated state at any point. Destroy actually removes the clone (unlike the singleton resources — this one has a true delete). `active` and `used_bytes` are never protected by `UseStateForUnknown`, so they may legitimately show drift on refresh if another BE is activated out-of-band during the test — that's expected behavior, not a defect.

**Cleanup:** Confirm `tf-acc-manual-be01` no longer exists and boot-pool usage has returned to its pre-test level.

---

### 8. truenas_service

**MT-SYSTEM-012 — Toggle a stopped, non-critical service (FTP) enabled-at-boot, then restore**

**Resource:** truenas_service
**Type:** CRUD (existing-object toggle — services are never actually created or deleted, only enabled/disabled and started/stopped)
**Environment:** Any TrueNAS box where the FTP service is currently stopped and disabled.
**Preconditions:** `midclt call service.query '[["service","=","ftp"]]'` and confirm the returned record shows `"state": "STOPPED"` and `"enable": false`. If FTP is already enabled/running on your box, pick a different currently-stopped, currently-disabled service and substitute its name (do not use this specific case against a service you rely on).

**Config (enable at boot; leave running state untouched):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_service" "test" {
  name    = "ftp"
  enabled = true
}
```

**Config (restore — disable at boot again):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_service" "test" {
  name    = "ftp"
  enabled = false
}
```

**Steps:**
1. `terraform init && terraform apply -auto-approve` with the enable config.
2. Verify `midclt call service.query '[["service","=","ftp"]]'` shows `"enable": true`, and `"state"` is STILL `"STOPPED"` — this case deliberately never sets `running`, so the live running state must be untouched. Confirm the same in UI **System > Services** (the boot-enable toggle is on; the service is not running).
3. `terraform import truenas_service.imported ftp`; confirm no diff.
4. Apply the restore config (`enabled = false`).
5. Verify `midclt call service.query '[["service","=","ftp"]]'` shows `"enable": false` again, still `"state": "STOPPED"`.
6. `terraform destroy -auto-approve`.
7. Verify via `midclt call service.query '[["service","=","ftp"]]'` that `"enable"` is STILL `false` after destroy (this resource's destroy behavior is a best-effort disable+stop, not a true delete — since there's no real object to delete).

**Expected results:** Only the boot-time autostart flag (`enabled`) changes; the live running state (`running`, mapped to `service.start`/`service.stop`) is never touched by this case. FTP ends the test in the same disabled/stopped state it started in.

**Cleanup:** Confirm `midclt call service.query '[["service","=","ftp"]]'` shows `"enable": false, "state": "STOPPED"` — the original baseline.

---

### 9. truenas_ntp_server

**MT-SYSTEM-013 — CRUD: add an extra (unreachable) NTP server with force, never touch the stock servers**

**Resource:** truenas_ntp_server
**Type:** CRUD
**Environment:** Any TrueNAS box.
**Preconditions:** `midclt call system.ntpserver.query` and record the IDs/addresses of the box's existing (stock) NTP servers — typically the three `*.debian.pool.ntp.org` entries. Do not modify or delete any of those in this case.

**⚠ Safety:** `force = true` is required here because the address used (`203.0.113.1`, TEST-NET-3) is intentionally unreachable — `force` bypasses TrueNAS's reachability validation on create/update. It is NOT related to protecting the minimum server count; nothing in this resource stops you from deleting one of the stock servers if you target it by ID, so double-check the `id` you're operating on at every step against the addresses recorded in Preconditions.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_ntp_server" "test" {
  address = "203.0.113.1"
  iburst  = false
  burst   = false
  prefer  = false
  force   = true
}
```

**Steps:**
1. `terraform init && terraform apply -auto-approve`.
2. Verify `midclt call system.ntpserver.query '[["address","=","203.0.113.1"]]'` returns the new entry; confirm the stock servers recorded in Preconditions are still present and unmodified (`midclt call system.ntpserver.query`).
3. Check UI **System Settings > General > NTP Servers** shows the new entry alongside the untouched stock entries.
4. Update `prefer = true` and re-apply in place; confirm `id` is unchanged.
5. `terraform import truenas_ntp_server.imported <id>` (numeric id from state); confirm no diff except `force`, which is write-only and never read back — expect it to show as empty/unset post-import, which is correct.
6. `terraform destroy -auto-approve`.
7. Verify `midclt call system.ntpserver.query '[["address","=","203.0.113.1"]]'` returns `[]`, and the stock servers from Preconditions are still present, unmodified.

**Expected results:** Only the test NTP server is created/updated/deleted. The box's stock NTP servers are never referenced by ID or address at any point.

**Cleanup:** Confirm `midclt call system.ntpserver.query` matches the Preconditions baseline (only the stock servers remain).

---

### 10. truenas_ssh_config

**MT-SYSTEM-014 — Datasource read of ssh_config**

**Resource:** truenas_ssh_config
**Type:** Datasource
**Environment:** Any TrueNAS box. Safe on production.
**Preconditions:** None.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

data "truenas_ssh_config" "current" {}

output "tcpport" {
  value = data.truenas_ssh_config.current.tcpport
}
```

**Steps:**
1. `terraform init && terraform apply -auto-approve`.
2. Confirm `terraform output tcpport` matches `midclt call ssh.config` and **Services > SSH** (configure) in the UI.
3. `terraform destroy -auto-approve`.

**Expected results:** Read-only, idempotent.

**Cleanup:** None.

---

**MT-SYSTEM-015 — Manual-only DANGEROUS: ssh_config singleton set-and-restore**

**Resource:** truenas_ssh_config
**Type:** Manual-only DANGEROUS
**Environment:** Disposable TrueNAS instance ONLY. Console/IPMI access, or an out-of-band SSH session on a second terminal that you keep open through the entire test, required as a fallback.
**Preconditions:** Console/IPMI access confirmed working. `midclt call ssh.config` output saved for comparison. Keep a second, already-authenticated SSH session open to the box for the duration of this test as an extra safety net.

**⚠ Safety:** `tcpport`, `passwordauth`, `bindiface`, and `kerberosauth` on this resource directly control how (and whether) management SSH access works. Changing `tcpport` moves sshd to a different port (your existing session survives, but new connections need the new port); disabling `passwordauth` locks out anyone relying on password auth (key-based sessions are unaffected); restricting `bindiface` can make sshd stop listening on the interface you're connecting through, dropping ALL SSH access including your existing session on many sshd configurations; `kerberosauth` changes affect any Kerberos-authenticated sessions. This case only ever touches `options` (raw sshd_config text appended verbatim — treated as low-risk/additive by the codebase's own test authors) using an innocuous comment line. Do not extend it to the four fields above without console/IPMI access AND an already-open, independent SSH session as fallback. Recovery if locked out: use console/IPMI (or the still-open second SSH session, if it survived) to run `midclt call ssh.update '{"tcpport": 22}'` (or whichever field needs reverting) and restart the SSH service if needed (`midclt call service.restart ssh`).

**Config (step A — read baseline):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

data "truenas_ssh_config" "current" {}

output "orig_options" {
  value = data.truenas_ssh_config.current.options
}
```

**Config (step B — append a comment-only line, low risk / additive):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_ssh_config" "test" {
  options = "# tf-acc-manual-ssh-01\n"
}
```

**Config (step C — restore, using the value from step A):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_ssh_config" "test" {
  options = "<orig_options value from step A>"
}
```

**Steps:**
1. Apply step A; record `orig_options`.
2. Apply step B. `terraform plan` should show ONLY `options` changing.
3. Verify `midclt call ssh.config` shows the appended comment line; confirm your existing SSH session AND a brand-new SSH connection attempt both still succeed on the original port.
4. Apply step C with the original value substituted in.
5. Verify restored via `midclt call ssh.config`.
6. `terraform import truenas_ssh_config.test ssh_config`; confirm no diff.
7. `terraform destroy -auto-approve`.

**Expected results:** Only `options` changes at any point; SSH access is never interrupted. Destroy makes no API call — `midclt call ssh.config` after destroy still shows the restored value.

**Cleanup:** Confirm `options` on the box matches the step-1 baseline and that both the pre-existing and a fresh SSH session work normally.

---

### 11. truenas_ftp_config

**MT-SYSTEM-016 — Datasource read of ftp_config**

**Resource:** truenas_ftp_config
**Type:** Datasource
**Environment:** Any TrueNAS box. Safe on production.
**Preconditions:** None.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

data "truenas_ftp_config" "current" {}

output "port" {
  value = data.truenas_ftp_config.current.port
}
```

**Steps:**
1. `terraform init && terraform apply -auto-approve`.
2. Confirm `terraform output port` matches `midclt call ftp.config` and **Services > FTP** (configure) in the UI.
3. `terraform destroy -auto-approve`.

**Expected results:** Read-only, idempotent.

**Cleanup:** None.

---

**MT-SYSTEM-017 — Singleton set-and-restore (DISRUPTIVE): ftp_config banner**

**Resource:** truenas_ftp_config
**Type:** Singleton
**Environment:** Non-production recommended.
**Preconditions:** `midclt call ftp.config` succeeds; note the current `banner` value.

**⚠ Safety:** This case only touches `banner` (the text shown to FTP clients on connect — cosmetic). It deliberately never touches `port`, `defaultroot`, `onlyanonymous`, or `onlylocal` — those fields can disrupt FTP access for anyone depending on the service. Do not extend this case to those fields on a box where FTP is in active use.

**Config (step A — read baseline):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

data "truenas_ftp_config" "current" {}

output "orig_banner" {
  value = data.truenas_ftp_config.current.banner
}
```

**Config (step B — set test value):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_ftp_config" "test" {
  banner = "tf-acc-manual-banner-01"
}
```

**Config (step C — restore):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_ftp_config" "test" {
  banner = "<orig_banner value from step A>"
}
```

**Steps:**
1. Apply step A; record `orig_banner`.
2. Apply step B; verify `midclt call ftp.config` shows the new banner and UI **Services > FTP** reflects it.
3. Apply step C with the original value substituted in; verify restored.
4. `terraform import truenas_ftp_config.test ftp_config`; confirm no diff (no secret fields to ignore on this resource).
5. `terraform destroy -auto-approve`.

**Expected results:** Only `banner` changes at any point. Destroy makes no API call — `midclt call ftp.config` after destroy still shows the restored banner.

**Cleanup:** Confirm `banner` on the box matches the step-1 baseline.

---

### 12. truenas_snmp_config

**MT-SYSTEM-018 — Datasource read of snmp_config**

**Resource:** truenas_snmp_config
**Type:** Datasource
**Environment:** Any TrueNAS box. Safe on production.
**Preconditions:** None.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

data "truenas_snmp_config" "current" {}

output "community" {
  value = data.truenas_snmp_config.current.community
}
```

**Steps:**
1. `terraform init && terraform apply -auto-approve`.
2. Confirm `terraform output community` matches `midclt call snmp.config` and **Services > SNMP** (configure) in the UI.
3. `terraform destroy -auto-approve`.

**Expected results:** Read-only, idempotent. Note the datasource never exposes `v3_password`/`v3_privpassphrase` (write-only fields do not exist on the datasource model at all).

**Cleanup:** None.

---

**MT-SYSTEM-019 — Singleton set-and-restore (DISRUPTIVE): snmp_config location**

**Resource:** truenas_snmp_config
**Type:** Singleton
**Environment:** Non-production recommended.
**Preconditions:** `midclt call snmp.config` succeeds; note the current `location` value.

**⚠ Safety:** This case only touches `location` (a free-text physical-location string, cosmetic). `v3_password` and `v3_privpassphrase` are write-only Sensitive fields — they are accepted on write but never read back into state, so `ImportStateVerify` will always show them empty after import; that is expected, not a bug. Do not attempt to verify their values via `terraform state show`; check SNMPv3 behavior against actual SNMP client traffic if needed.

**Config (step A — read baseline):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

data "truenas_snmp_config" "current" {}

output "orig_location" {
  value = data.truenas_snmp_config.current.location
}
```

**Config (step B — set test value):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_snmp_config" "test" {
  location = "tf-acc-manual-location-01"
}
```

**Config (step C — restore):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_snmp_config" "test" {
  location = "<orig_location value from step A>"
}
```

**Steps:**
1. Apply step A; record `orig_location`.
2. Apply step B; verify `midclt call snmp.config` shows the new location and UI **Services > SNMP** reflects it.
3. Apply step C with the original value substituted in; verify restored.
4. `terraform import truenas_snmp_config.test snmp_config`; confirm no diff EXCEPT `v3_password`/`v3_privpassphrase`, which will show empty (write-only, never read back — expected).
5. `terraform destroy -auto-approve`.

**Expected results:** Only `location` changes at any point. Destroy makes no API call — `midclt call snmp.config` after destroy still shows the restored location.

**Cleanup:** Confirm `location` on the box matches the step-1 baseline.

---

### 13. truenas_ups_config

**MT-SYSTEM-020 — Precondition check + singleton set-and-restore (DISRUPTIVE): ups_config description**

**Resource:** truenas_ups_config
**Type:** Singleton
**Environment:** Non-production recommended.
**Preconditions:** **Self-skip check (mandatory):** run `midclt call ups.config` and inspect `driver` and `port`. If EITHER is an empty string, UPS is unconfigured on this box — SKIP this case entirely (`ups.update` requires non-empty `driver`/`port` on every call, so there is no way to restore the box to an "unconfigured" state once you write anything; running this case anyway will leave permanent UPS configuration residue on a box that had none). Only proceed if both `driver` and `port` are non-empty; record all of: `identifier`, `mode`, `remotehost`, `remoteport`, `driver`, `port`, `options`, `optionsupsd`, `description`, `shutdown`, `shutdowntimer`, `shutdowncmd`, `monuser`, `extrausers`, `rmonitor`, `powerdown`, `hostsync`, `nocommwarntime` — the restore step needs the FULL set, not just `description`, because `ups.update` requires several fields on every call.

**⚠ Safety:** This case only drives `description` (cosmetic) through Terraform, but because `ups.update` requires `driver`+`port` (and effectively re-sends the full config) on every call, an incomplete restore payload can unintentionally clear other UPS settings. Use the full baseline recorded in Preconditions when restoring — do not restore `description` alone.

**Config (step A — read baseline):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

data "truenas_ups_config" "current" {}

output "orig_description" {
  value = data.truenas_ups_config.current.description
}
output "orig_driver" {
  value = data.truenas_ups_config.current.driver
}
output "orig_port" {
  value = data.truenas_ups_config.current.port
}
```

**Config (step B — set test value):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_ups_config" "test" {
  description = "tf-acc-manual-ups-01"
}
```

**Config (step C — restore, using the value from step A):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_ups_config" "test" {
  description = "<orig_description value from step A>"
}
```

**Steps:**
1. Run the Preconditions self-skip check. If `driver`/`port` are empty, STOP — do not proceed.
2. Apply step A; record all baseline fields listed in Preconditions.
3. Apply step B; verify `midclt call ups.config` shows the new description AND that `driver`/`port`/`mode`/all other fields are unchanged from the step-2 baseline (this is the real risk to watch for — the merge-based update should preserve everything else, but confirm it).
4. Check UI **Services > UPS** (configure) reflects the new description.
5. Apply step C with the original description substituted in; verify restored and all other fields still match baseline.
6. `terraform import truenas_ups_config.test ups_config`; confirm no diff except `monpwd` (write-only, expected empty).
7. `terraform destroy -auto-approve`.

**Expected results:** Only `description` changes at any point; every other UPS field remains at its baseline value throughout, confirming the provider's merge-before-update logic works correctly. Destroy makes no API call.

**Cleanup:** Confirm the full UPS config on the box matches the step-2 baseline recorded in Preconditions.

---

### 14. truenas_mail

**MT-SYSTEM-021 — Precondition check + singleton set-and-restore (DISRUPTIVE): mail fromname**

**Resource:** truenas_mail
**Type:** Singleton
**Environment:** Non-production recommended.
**Preconditions:** **Self-skip check (mandatory):** run `midclt call mail.config` and inspect `fromemail`. If it is an empty string, mail is unconfigured on this box — SKIP this case entirely (`mail.update` requires a non-empty `fromemail` on every call, so there is no way to restore the box to an "unconfigured" state once you write anything). Only proceed if `fromemail` is non-empty; record `fromemail`, `fromname`, `outgoingserver`, `port`, `security`, `smtp`, `user`.

**⚠ Safety:** This case only drives `fromname` (cosmetic display name) through Terraform, but `mail.update` requires `fromemail` on every call and effectively re-sends the full config, so use the FULL baseline recorded in Preconditions when restoring, not `fromname` alone.

**Config (step A — read baseline):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

data "truenas_mail" "current" {}

output "orig_fromname" {
  value = data.truenas_mail.current.fromname
}
output "orig_fromemail" {
  value = data.truenas_mail.current.fromemail
}
```

**Config (step B — set test value):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_mail" "test" {
  fromname = "tf-acc-manual-mail-01"
}
```

**Config (step C — restore, using the value from step A):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_mail" "test" {
  fromname = "<orig_fromname value from step A>"
}
```

**Steps:**
1. Run the Preconditions self-skip check. If `fromemail` is empty, STOP — do not proceed.
2. Apply step A; record baseline fields.
3. Apply step B; verify `midclt call mail.config` shows the new `fromname` AND that `fromemail`/`outgoingserver`/`port`/`security`/`smtp`/`user` are unchanged from baseline.
4. Check UI **System Settings > General > Email** (or equivalent Email/Alert settings screen) reflects the new from-name.
5. Apply step C with the original `fromname` substituted in; verify restored and all other fields still match baseline.
6. `terraform import truenas_mail.test mail`; confirm no diff except `pass` (write-only, expected empty).
7. `terraform destroy -auto-approve`.

**Expected results:** Only `fromname` changes at any point; every other mail field remains at baseline throughout. Destroy makes no API call.

**Cleanup:** Confirm the full mail config on the box matches the step-2 baseline.

---

### 15. truenas_alert_service

**MT-SYSTEM-022 — CRUD: create, update, import, destroy a Mail-type alert service**

**Resource:** truenas_alert_service
**Type:** CRUD
**Environment:** Any TrueNAS box. Safe — this creates an independent notification-target object, it does not touch the box-wide mail/SMTP configuration itself.
**Preconditions:** None beyond API reachability.

**Config (create):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_alert_service" "test" {
  name  = "tf-acc-manual-alert-01"
  level = "WARNING"
  attributes = jsonencode({
    type  = "Mail"
    email = "tf-acc-manual-alert-01@example.com"
  })
}
```

**Config (update — bump level in place):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_alert_service" "test" {
  name  = "tf-acc-manual-alert-01"
  level = "ERROR"
  attributes = jsonencode({
    type  = "Mail"
    email = "tf-acc-manual-alert-01@example.com"
  })
}
```

**Steps:**
1. `terraform init && terraform apply -auto-approve` with the create config.
2. Verify `midclt call alertservice.query '[["name","=","tf-acc-manual-alert-01"]]'` returns one record with `level = "WARNING"` and `attributes.type = "Mail"`, `attributes.email` matching. Confirm the same in UI **System Settings > Alert Settings** (Alert Services list).
3. Apply the update config; confirm `level` changed to `"ERROR"` in place (same `id`).
4. `terraform import truenas_alert_service.imported <id>` (numeric id from state); confirm plan shows a diff only on `attributes` (server-side JSON normalization can reorder/reformat keys — this is expected and matches the automated suite's own `ImportStateVerifyIgnore` on this field).
5. `terraform destroy -auto-approve`.
6. Verify `midclt call alertservice.query '[["name","=","tf-acc-manual-alert-01"]]'` returns `[]`.

**Expected results:** True CRUD lifecycle — the alert service object is genuinely created, updated in place, and deleted (unlike the singleton resources in this section).

**Cleanup:** Confirm no leftover alert service named `tf-acc-manual-alert-01` remains.

---

### 16. truenas_alert_policy

**MT-SYSTEM-023 — Singleton set-and-restore (DISRUPTIVE, whole-object risk): alert_policy classes**

**Resource:** truenas_alert_policy
**Type:** Singleton
**Environment:** Non-production recommended.
**Preconditions:** `midclt call alertclasses.config` succeeds; save the FULL `classes` JSON object returned — you must merge into it, never replace it wholesale.

**⚠ Safety:** The `classes` attribute represents the box's ENTIRE set of per-alert-class overrides. Any class name omitted from the JSON you apply reverts to its TrueNAS default the moment this resource is applied — there is no "partial update" for this resource. `terraform destroy` on this resource is also NOT a no-op like the other singletons in this section: it actively calls `alertclasses.update` with `classes = {}}`, resetting ALL classes to defaults. If your box has meaningful custom alert-class overrides already configured (e.g. custom levels/policies for specific classes), you MUST carry them forward in every `classes` value you apply in this test, and you must NOT casually destroy this resource at the end unless resetting all classes to default is actually what you want. This case is written to always include the full baseline plus one added override, specifically to avoid dropping existing overrides.

**Config (step A — read baseline):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

data "truenas_alert_policy" "current" {}

output "orig_classes" {
  value = data.truenas_alert_policy.current.classes
}
```

**Config (step B — apply baseline classes PLUS one added override; substitute the FULL JSON object from step A's `orig_classes` output in place of `<...baseline classes from step A...>` below, keyed by the SAME class names, and add the `UPSBatteryLow` key on top):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_alert_policy" "test" {
  classes = jsonencode(merge(
    <...baseline classes object from step A, as an HCL map...>,
    {
      UPSBatteryLow = {
        level  = "WARNING"
        policy = "DAILY"
      }
    }
  ))
}
```

**Config (step C — restore, using ONLY the baseline classes object from step A, with no added key):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_alert_policy" "test" {
  classes = jsonencode(<...baseline classes object from step A...>)
}
```

**Steps:**
1. Apply step A; save the complete `orig_classes` JSON somewhere durable (a file) before proceeding — you need it for both step B and step C, and for recovery if anything goes wrong.
2. Apply step B, with the baseline merged in plus the `UPSBatteryLow` override. Confirm via `midclt call alertclasses.config` that EVERY class present in the step-A baseline is still present and unchanged, and that `UPSBatteryLow` now shows `level=WARNING, policy=DAILY`.
3. Check UI **System Settings > Alert Settings** (alert classes / policy screen) reflects the change for `UPSBatteryLow` only.
4. Apply step C (baseline only, override removed); confirm `midclt call alertclasses.config` matches the original step-A baseline.
5. `terraform import truenas_alert_policy.test alert_policy`; confirm no diff.
6. Decide deliberately whether to `terraform destroy`: doing so resets ALL classes to TrueNAS defaults, which may differ from your step-A baseline if the box had custom overrides. If you do destroy, immediately re-apply step C's config afterward to put the baseline back, since destroy is NOT a no-op for this resource (unlike the other singletons in this doc).

**Expected results:** No alert class other than `UPSBatteryLow` is ever changed. The baseline is fully restored by the end of the test regardless of whether `terraform destroy` was run.

**Cleanup:** Confirm `midclt call alertclasses.config` matches the original step-A baseline, including for any classes never mentioned in this test's HCL.

---

### 17. truenas_reporting_exporter

**MT-SYSTEM-024 — CRUD: GRAPHITE exporter to a TEST-NET destination, disabled throughout**

**Resource:** truenas_reporting_exporter
**Type:** CRUD
**Environment:** Any TrueNAS box. Safe — the destination is a non-routable documentation address and the exporter is kept disabled, so no metrics traffic is ever actually sent.
**Preconditions:** None beyond API reachability.

**Config (create, enabled = false throughout):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_reporting_exporter" "test" {
  name    = "tf-acc-manual-exporter-01"
  enabled = false
  attributes = {
    destination_ip   = "192.0.2.50"
    destination_port = 2003
    namespace        = "tfaccmanual"
  }
}
```

**Config (update — change destination_port in place, still disabled):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_reporting_exporter" "test" {
  name    = "tf-acc-manual-exporter-01"
  enabled = false
  attributes = {
    destination_ip   = "192.0.2.50"
    destination_port = 2004
    namespace        = "tfaccmanual"
  }
}
```

**Steps:**
1. `terraform init && terraform apply -auto-approve` with the create config.
2. Verify `midclt call reporting.exporters.query '[["name","=","tf-acc-manual-exporter-01"]]'` returns one record with `enabled = false`, `attributes.destination_ip = "192.0.2.50"`, `attributes.destination_port = 2003`, `attributes.namespace = "tfaccmanual"`, and server-side defaults filled in: `attributes.prefix = "scale"`, `attributes.update_every = 1`, `attributes.buffer_on_failures = 10`, `attributes.send_names_instead_of_ids = true`, `attributes.matching_charts = "*"`. Confirm the same in UI **Reporting > Configuration** (Exporters).
3. Apply the update config; confirm `destination_port` changed to `2004` in place, `enabled` still `false`.
4. `terraform import truenas_reporting_exporter.imported <id>` (numeric id from state); confirm no diff.
5. `terraform destroy -auto-approve`.
6. Verify `midclt call reporting.exporters.query '[["name","=","tf-acc-manual-exporter-01"]]'` returns `[]`.

**Expected results:** True CRUD lifecycle, exporter genuinely created/updated/deleted. Because `enabled` stays `false` for the whole test, TrueNAS never actually attempts to send metrics to the TEST-NET address.

**Cleanup:** Confirm no leftover exporter named `tf-acc-manual-exporter-01` remains.

---

### 18. truenas_audit_config

**MT-SYSTEM-025 — Datasource read of audit_config**

**Resource:** truenas_audit_config
**Type:** Datasource
**Environment:** Any TrueNAS box. Safe on production.
**Preconditions:** None.

**Config:**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

data "truenas_audit_config" "current" {}

output "retention" {
  value = data.truenas_audit_config.current.retention
}
output "quota_fill_warning" {
  value = data.truenas_audit_config.current.quota_fill_warning
}
```

**Steps:**
1. `terraform init && terraform apply -auto-approve`.
2. Confirm the outputs match `midclt call audit.config` and **System Settings > Audit** in the UI. Also confirm the read-only `space` and `enabled_services` nested attributes populate (these are computed-only and driven by other resources, e.g. SMB share audit settings — this resource itself has no top-level enable/disable toggle).
3. `terraform destroy -auto-approve`.

**Expected results:** Read-only, idempotent.

**Cleanup:** None.

---

**MT-SYSTEM-026 — Singleton set-and-restore (DISRUPTIVE): audit_config quota_fill_warning**

**Resource:** truenas_audit_config
**Type:** Singleton
**Environment:** Non-production recommended (mutates box-wide audit retention/quota settings, though this is the least risky of the Tier-2 singletons — its `terraform destroy` is a true no-op, unlike `alert_policy`).
**Preconditions:** `midclt call audit.config` succeeds; note the current `quota_fill_warning` value (valid range 5-80).

**⚠ Safety:** This case only touches `quota_fill_warning` (a percentage threshold for a warning alert — it does not change retention, quota size, or any enable/disable behavior; there is no top-level audit on/off switch on this resource at all, since auditing is enabled per-service elsewhere). Choose a test value different from the baseline and within the valid 5-80 range.

**Config (step A — read baseline):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

data "truenas_audit_config" "current" {}

output "orig_quota_fill_warning" {
  value = data.truenas_audit_config.current.quota_fill_warning
}
```

**Config (step B — set test value; use 60, or 50 if your baseline happens to already be 60):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_audit_config" "test" {
  quota_fill_warning = 60
}
```

**Config (step C — restore, using the value from step A):**
```hcl
provider "truenas" {
  endpoint = "wss://192.168.1.68/api/current"
  insecure = true
}

resource "truenas_audit_config" "test" {
  quota_fill_warning = <orig_quota_fill_warning value from step A>
}
```

**Steps:**
1. Apply step A; record `orig_quota_fill_warning`.
2. Apply step B; verify `midclt call audit.config` shows `quota_fill_warning = 60` (or `50`) and that `retention`, `reservation`, `quota`, `quota_fill_critical` are unchanged from baseline. Confirm UI **System Settings > Audit** reflects it.
3. Apply step C with the original value substituted in; verify restored.
4. `terraform import truenas_audit_config.test audit_config`; confirm no diff.
5. `terraform destroy -auto-approve`.
6. Verify `midclt call audit.config` after destroy STILL shows the restored `quota_fill_warning` — this resource's destroy makes no API call at all (the safest destroy behavior of any Tier-2 singleton in this section).

**Expected results:** Only `quota_fill_warning` changes at any point. Destroy is confirmed to be a true no-op against the live box.

**Cleanup:** Confirm `quota_fill_warning` on the box matches the step-1 baseline.

---

## Virtualization and HA / Enterprise

These cases are executed by hand with the Terraform CLI plus the TrueNAS UI and `midclt call` (run over SSH on the TrueNAS box itself, or via `ssh root@<host> midclt call ...`). They are NOT part of this repo's Go acceptance-test harness (`go test ./internal/resources/...`), although several mirror what that harness does automatically under `TRUENAS_HA=1`/`TRUENAS_DISRUPTIVE=1`.

Every `Config` block below is self-contained. Declare the shared provider variables once per working directory (e.g. in a `variables.tf` next to whichever case's config you're running):

```hcl
variable "truenas_endpoint" {
  type        = string
  description = "e.g. wss://192.168.1.249/api/current"
}

variable "truenas_api_key" {
  type      = string
  sensitive = true
}

provider "truenas" {
  endpoint = var.truenas_endpoint
  api_key  = var.truenas_api_key
  insecure = true
}
```

Supply `truenas_endpoint`/`truenas_api_key` (and any other variable a case declares) via `terraform.tfvars` (untracked/not committed) or `-var` flags — never hardcode secrets into a `.tf` file. All example resource names use the `tf-acc-manual-` convention (or, where the underlying TrueNAS API rejects non-alphanumeric names, e.g. `truenas_vm`, a plain alphanumeric `tfaccmanual...` form) so they're never mistaken for the Go harness's own `tf-acc-`/`tfacc`-prefixed fixtures.

### 1. truenas_vm

**MT-HA-001 — Create, update, import, and destroy a stopped VM**

**Resource:** truenas_vm
**Type:** CRUD
**Environment:** Any TrueNAS system capable of hosting a VM (Enterprise HA not required for this resource).
**Preconditions:** Terraform >= required provider version installed; box reachable; sufficient free memory (>= 512 MiB) for a throwaway VM.
**Config:**
```hcl
resource "truenas_vm" "manual_test" {
  name        = "tfaccmanualvm001" # vm.create requires an alphanumeric-only name — no "-" or "_"
  description = "MT-HA-001 manual test VM"
  memory      = 536870912 # 512 MiB, bytes
  vcpus       = 1
  autostart   = false # never let this VM start automatically at boot
  running     = false # create it stopped — this test never boots a guest
}
```
**Steps:**
1. `terraform init && terraform apply`.
2. UI: Virtualization (or "Instances") page shows `tfaccmanualvm001`, State = STOPPED, Autostart = No.
3. `midclt call vm.query '[["name","=","tfaccmanualvm001"]]'` — confirm `memory=536870912`, `vcpus=1`, `autostart=false`, `status.state="STOPPED"`.
4. `terraform state show truenas_vm.manual_test` — confirm `running = false` and `status = "STOPPED"`.
5. Update: change `description` to `"MT-HA-001 manual test VM (updated)"` and `vcpus` to `2`; `terraform apply` again. Confirm the plan only touches `description`/`vcpus` (memory/name/autostart/running unchanged). Re-verify via UI and `vm.query`.
6. Import: `terraform state rm truenas_vm.manual_test`, then `terraform import truenas_vm.manual_test <numeric id from step 3/4>`. Run `terraform plan` — expect "No changes."
7. Destroy: `terraform destroy`.
8. `midclt call vm.query '[["name","=","tfaccmanualvm001"]]'` — expect `[]`.
**Expected results:** VM is created stopped and non-autostart throughout; update applies cleanly with a minimal diff; import reconstructs identical state; destroy removes the VM entirely, with no orphaned devices or leftover UI entry.
**Cleanup:** None beyond the destroy in step 7 — confirm no `tfaccmanualvm001` entry remains in the UI.

### 2. truenas_vm_device

**MT-HA-002 — Create a DISPLAY (SPICE) device on a fixture VM**

**Resource:** truenas_vm_device
**Type:** CRUD
**Environment:** Any TrueNAS system capable of hosting a VM.
**Preconditions:** Same as MT-HA-001. Uses its own dedicated fixture VM — never attach to a pre-existing VM.
**Config:**
```hcl
variable "vm_display_password" {
  type      = string
  sensitive = true
  # 8+ chars; the API requires a password on every DISPLAY device.
}

resource "truenas_vm" "manual_test" {
  name      = "tfaccmanualvmdev002"
  memory    = 536870912
  vcpus     = 1
  autostart = false
  running   = false
}

resource "truenas_vm_device" "manual_test" {
  vm = truenas_vm.manual_test.id
  attributes = jsonencode({
    dtype      = "DISPLAY"
    type       = "SPICE"
    bind       = "0.0.0.0"
    resolution = "1024x768"
    wait       = false
    web        = false
    password   = var.vm_display_password
    port       = 15900 # SPICE port
    web_port   = 15901 # web/noVNC port — must differ from "port"
  })
}
```
**Steps:**
1. `terraform apply`.
2. UI: Virtualization > `tfaccmanualvmdev002` > Devices tab shows a Display device, type SPICE, bind 0.0.0.0, port 15900, web port 15901.
3. `midclt call vm.device.query '[["vm","=",<vm id>]]'` — confirm one DISPLAY device with the attributes above and an auto-assigned `order`.
4. Update: add `order = 5` to the `truenas_vm_device` block; `terraform apply`. Confirm only `order` changes.
5. Import: `terraform state rm truenas_vm_device.manual_test`, then `terraform import truenas_vm_device.manual_test <device id>`. `terraform plan` should show no changes except possibly `attributes` (the API may reorder/normalize JSON keys — this is expected and is why `attributes` is excluded from strict import verification).
6. Destroy: `terraform destroy` (removes both the device and the fixture VM — Terraform destroys in dependency order, device first).
7. `midclt call vm.device.query '[["vm","=",<vm id>]]'` and `midclt call vm.query '[["name","=","tfaccmanualvmdev002"]]'` — both expect `[]`.
**Expected results:** Device attaches to its own fixture VM only; password and distinct SPICE/web ports are accepted; update changes only `order`; destroy removes device and VM cleanly.
**Cleanup:** None beyond step 6 — verify no `tfaccmanualvmdev002` VM or orphaned device remains.

### 3. truenas_vmware

**MT-HA-003 — Manual-only: VMware snapshot integration against a real vCenter/ESXi host**

**Resource:** truenas_vmware
**Type:** Manual-only
**Environment:** Any TrueNAS system, PLUS a real, reachable vCenter server or ESXi host the tester controls, with a real datastore name that exists on it. This resource's `create` synchronously validates hostname/username/password against that live endpoint — an unreachable or fake host is rejected outright (`ENETUNREACH`/`ETIMEDOUT`) and nothing is persisted, which is why this case cannot be automated in this environment and is documented-skip in the Go harness (`internal/resources/vmware/acceptance_test.go`).
**Preconditions:** Real vCenter/ESXi credentials and a real datastore name; a disposable dataset/filesystem path on the TrueNAS box to use as `filesystem` (pre-create it via the UI or `truenas_dataset` if available — `truenas_vmware` does not create it).
**⚠ Safety:** Use genuine credentials for a host you are authorized to test against. Do not point this at a production vCenter/ESXi environment unless you intend a real, disposable integration.
**Config:**
```hcl
variable "vmware_hostname" {
  type        = string
  description = "Real, reachable vCenter or ESXi IP/FQDN"
}
variable "vmware_username" {
  type = string
}
variable "vmware_password" {
  type      = string
  sensitive = true
}
variable "vmware_datastore" {
  type        = string
  description = "Must already exist on the vCenter/ESXi host"
}

resource "truenas_vmware" "manual_test" {
  datastore  = var.vmware_datastore
  filesystem = "tank/tf-acc-manual-vmware" # pre-existing disposable dataset
  hostname   = var.vmware_hostname
  username   = var.vmware_username
  password   = var.vmware_password # write-only — never stored in state; requires Terraform >= 1.11
}
```
**Steps:**
1. `terraform apply`. Expect success (no `ENETUNREACH`/`ETIMEDOUT`) since the host is real and reachable.
2. UI: Data Protection > VMware Snapshots shows the new entry.
3. `midclt call vmware.query '[["datastore","=","<your datastore>"]]'` — confirm `hostname`/`username`/`datastore`/`filesystem` match, and `state.state` reads `PENDING` (no snapshot operation performed yet) or later `SUCCESS` once a snapshot task runs.
4. Update: change `datastore` (or `hostname`) to a different valid value on the same vCenter/ESXi; re-supply `password` in the same apply (write-only values are never persisted server-side by Terraform, so every apply that touches this resource must resupply it via the variable). `terraform apply`; verify the change via `vmware.query`.
5. Import: `terraform state rm truenas_vmware.manual_test`, then `terraform import truenas_vmware.manual_test <id>`. `ImportStateVerify` for `password` is expected to differ — this is by design (write-only, never read back).
6. Destroy: `terraform destroy`.
7. `midclt call vmware.query '[["id","=",<id>]]'` — expect `[]`.
**Expected results:** Create/update succeed against a real, reachable endpoint; `state` reflects live snapshot-integration status; destroy removes the integration cleanly. (For contrast, and NOT to be run as part of this case: pointing `hostname` at an unreachable or fake host produces an immediate `[EINVAL] ... Failed to connect: [ENETUNREACH]`/`[ETIMEDOUT]` error with nothing persisted — expected, documented behavior, not a bug.)
**Cleanup:** Destroy in step 6; remove the disposable `tank/tf-acc-manual-vmware` dataset if it was created solely for this test.

### 4. truenas_failover_config

**MT-HA-004 — Datasource read of live failover status**

**Resource:** truenas_failover_config
**Type:** Datasource
**Environment:** Enterprise HA system. Gate: `TRUENAS_HA=1` semantics apply conceptually — for a manual run, confirm the box is licensed for Enterprise HA (`midclt call failover.licensed` returns `true`) before proceeding; this case is read-only regardless.
**Preconditions:** Box is an HA pair (or at least `failover.licensed=true`).
**Config:**
```hcl
data "truenas_failover_config" "status" {}

output "failover_status" {
  value = data.truenas_failover_config.status
}
```
**Steps:**
1. `terraform apply` (data-source-only — no state mutation).
2. Confirm `id = "failover_config"`; `disabled`, `master`, `timeout` are all set.
3. Confirm `status` and `node` are set (e.g. `status = "MASTER"`, `node = "A"` or `"B"`); `disabled_reasons` is a list (empty `[]` on a healthy pair).
4. Cross-check via UI (System Settings > Failover) and `midclt call failover.status`, `midclt call failover.node`, `midclt call failover.config` — values must match the datasource output.
**Expected results:** All fields populate; `disabled_reasons` is `[]` on a healthy, licensed HA pair with failover enabled.
**Cleanup:** None — read-only.

**MT-HA-005 — Singleton Tier-2 "timeout" set-and-restore**

**Resource:** truenas_failover_config
**Type:** Singleton
**Environment:** Enterprise HA system, fully disposable — this test mutates a live, box-wide singleton.
**Preconditions:** `failover.licensed=true`; explicit authorization to mutate this box's failover config.
**⚠ Safety:** Only `timeout` is ever exercised in this case. NEVER set `disabled` or `master` here — see MT-HA-006 for the deliberate, high-risk real-failover exercise; setting `master` to a value that differs from the node's live state is itself a real failover trigger, and setting `disabled=true` administratively disables HA.
**Config:**
```hcl
variable "failover_timeout" {
  type = number
}

resource "truenas_failover_config" "manual_test" {
  timeout = var.failover_timeout
  # "disabled" and "master" intentionally left unset — do not add them here.
}
```
**Steps:**
1. `midclt call failover.config` — record the current `timeout` value as `ORIG`. Write down (on paper/in a scratch file, before touching Terraform) the restore command: `midclt call failover.update '{"timeout": ORIG}'`.
2. `terraform apply -var="failover_timeout=<ORIG + 1>"`.
3. Verify via UI and `midclt call failover.config` that only `timeout` changed; `disabled`/`master` are unchanged from their pre-test values.
4. `terraform apply -var="failover_timeout=<ORIG>"` — restore via Terraform.
5. Verify `timeout == ORIG` again via UI/`failover.config`.
6. Import: `terraform state rm truenas_failover_config.manual_test`, then `terraform import truenas_failover_config.manual_test failover_config`. `terraform plan` — expect no changes.
7. `terraform destroy` — expect a WARNING diagnostic ("Failover configuration left in place") and no API call; confirm via `failover.config` that the live config is untouched by the destroy itself.
**Expected results:** Only `timeout` round-trips; `disabled`/`master` never change; destroy leaves the live singleton alone (state-only removal).
**Cleanup:** Confirm step 5's restore succeeded (re-run `midclt call failover.config` one more time) before ending the session — if step 4 failed for any reason, run the command recorded in step 1 directly.

**MT-HA-006 — Manual-only DANGEROUS: real controlled failover exercise**

**Resource:** truenas_failover_config
**Type:** Manual-only DANGEROUS
**Environment:** A fully disposable Enterprise HA pair that you are explicitly authorized to disrupt. Do not run this against any box hosting real workloads.
**Preconditions:** Written/explicit authorization from the environment owner; both controllers reachable independently (e.g. via separate IPs/IPMI) so you can observe both sides through the event; no other testing in flight against this pair.
**⚠ Safety:** This case triggers a REAL failover — the same operational impact as an actual hardware/network failure on one controller, including a brief service interruption while resources migrate to the peer. This is the highest-risk case in this section. Do not run it casually, and do not run it against shared infrastructure.
**Config:** None required (this exercise is driven by direct API calls / UI actions and the `truenas_failover_config` datasource for polling, not by applying a mutating resource config). Reuse MT-HA-004's datasource config to poll status.
**Steps:**
1. Record pre-failover baseline on BOTH controllers: `midclt call failover.status`, `midclt call failover.node`, `midclt call failover.config`, plus general health (`midclt call pool.query`, `midclt call service.query`, `zpool status`).
2. Choose a trigger:
   - Option A (software-initiated): `midclt call failover.become_passive` on the CURRENT master node — this demotes it to passive and promotes the peer to master.
   - Option B (hardware-style): power-cycle or reboot the OTHER (currently passive) controller via IPMI/UI, simulating a controller failure.
3. Immediately begin polling `terraform apply` against the MT-HA-004 datasource config (or plain `midclt call failover.status` / `failover.node`) every few seconds on the surviving/newly-active node.
4. Continue polling until `status` reads `MASTER` on the newly active node AND `disabled_reasons` (the datasource's list, backed by `failover.disabled.reasons`) has fully cleared back to `[]`. This can take up to a few minutes.
5. **⚠ KNOWN QUIRK — do not use `master` as your liveness signal.** Even after `disabled_reasons` has fully cleared and `failover.status` reports `MASTER` on the newly active node, the `master` boolean (both the raw `failover.config` field and the `truenas_failover_config` resource attribute) has been observed to still read `false` on that node. Rely exclusively on `status`/`node` (the datasource's `status`/`node` attributes, i.e. `failover.status`/`failover.node`) to determine which node is actually active — never on `master`.
6. Verify system health post-failover: pools imported (`pool.query`/`zpool status`), services running, shares reachable, and one node reports `MASTER` (no split-brain).
7. If you rebooted a controller in step 2 (Option B), wait for it to rejoin; verify it comes back reporting `status = "BACKUP"` with no blocking `disabled_reasons`.
8. Optionally fail back to the original master (repeat step 2's chosen method against the new master) to restore the pair's original layout — treat this as a second full exercise of steps 3–7, not a formality.
9. Record the final state and confirm with the environment owner that the pair is healthy before ending the session.
**Expected results:** Failover completes; the newly active node reports `MASTER` via `status`/`node` once `disabled_reasons` clears; no split-brain; the peer rejoins cleanly as `BACKUP` if rebooted.
**Cleanup:** Leave the pair in a known-good, mutually agreed state (failed back to original layout if that's what the environment owner expects); document the full before/after transcript for the record.

### 5. truenas_ipmi_lan

**MT-HA-007 — Datasource read of a BMC/IPMI LAN channel**

**Resource:** truenas_ipmi_lan
**Type:** Datasource
**Environment:** Enterprise HA (BMC-equipped) system.
**Preconditions:** Know at least one valid channel number — discover via UI (System Settings > IPMI) or `midclt call ipmi.lan.channels`.
**Config:**
```hcl
variable "ipmi_channel" {
  type = number
}

data "truenas_ipmi_lan" "test" {
  channel = var.ipmi_channel
}
```
**Steps:**
1. `terraform apply -var="ipmi_channel=<discovered channel>"`.
2. Confirm `dhcp`, `ipaddress`, `netmask`, `gateway`, `mac_address`, `vlan`, `vlan_priority` are all set (or `vlan`/`vlan_priority` null/0 if tagging is disabled).
3. Cross-check by logging into the BMC's own web UI directly at `ipaddress` and confirming the same address/MAC is shown there.
**Expected results:** Values match what the BMC itself reports.
**Cleanup:** None — read-only.

**MT-HA-008 — Singleton per-channel "vlan" set-and-restore**

**Resource:** truenas_ipmi_lan
**Type:** Singleton
**Environment:** Enterprise HA (BMC-equipped) system, fully disposable for BMC network changes.
**Preconditions:** Confirm with your network admin that the switch port the BMC is on will trunk whatever test VLAN you pick — otherwise the BMC may become unreachable over its own network path (see Safety).
**⚠ Safety:** This configures the out-of-band BMC network interface, not the host OS network. **REGISTER THE RESTORE PROCEDURE FIRST, before making any change.** Never leave the BMC unreachable at the end of this test. `dhcp`/`ipaddress`/`netmask`/`gateway` are resent verbatim, unchanged, on every step below — only `vlan` is exercised. The Terraform provider's own connection to TrueNAS runs over the host's separate management network, so `ipmi.lan.update` remains callable to fix a bad BMC network change even if the BMC itself becomes unreachable by IP — but plan the restore anyway.
**Config:**
```hcl
variable "ipmi_channel" {
  type = number
}
variable "ipmi_dhcp" {
  type = bool
}
variable "ipmi_ipaddress" {
  type    = string
  default = null
}
variable "ipmi_netmask" {
  type    = string
  default = null
}
variable "ipmi_gateway" {
  type    = string
  default = null
}
variable "ipmi_vlan" {
  type = number
}

resource "truenas_ipmi_lan" "manual_test" {
  channel   = var.ipmi_channel
  dhcp      = var.ipmi_dhcp
  ipaddress = var.ipmi_dhcp ? null : var.ipmi_ipaddress
  netmask   = var.ipmi_dhcp ? null : var.ipmi_netmask
  gateway   = var.ipmi_dhcp ? null : var.ipmi_gateway
  vlan      = var.ipmi_vlan
}
```
**Steps:**
1. `midclt call ipmi.lan.query '{"query-filters": [["channel","=",<channel>]]}'` — record `ip_address_source`, `ip_address`, `subnet_mask`, `default_gateway_ip_address`, `vlan_id` as the ORIGINAL values.
2. **Before any mutating apply**, write down the restore command: `midclt call ipmi.lan.update <channel> '{"dhcp": <orig dhcp>, "ipaddress": "<orig ip>", "netmask": "<orig netmask>", "gateway": "<orig gateway>", "vlan": <orig vlan or null>}'` (omit `ipaddress`/`netmask`/`gateway` entirely if `dhcp` is true).
3. `terraform apply` with `ipmi_vlan` set to a different value (e.g. original + 1, or `100` if VLAN tagging was previously disabled) and `ipmi_dhcp`/`ipmi_ipaddress`/`ipmi_netmask`/`ipmi_gateway` set to the ORIGINAL values from step 1 (unchanged).
4. UI: System Settings > IPMI shows the new VLAN tag for this channel.
5. `midclt call ipmi.lan.query ...` — confirm `vlan_id` matches the new value and `ip_address`/`subnet_mask`/`default_gateway_ip_address` are unchanged. Note: immediately after a static-IP update, `ip_address`/`subnet_mask` may transiently read `0.0.0.0` for a few seconds before settling — this is normal BMC LAN controller behavior, not a failure; wait ~3 seconds and re-check (retry up to ~8 times, 3s apart, before treating it as a real problem).
6. Import: `terraform state rm truenas_ipmi_lan.manual_test`, then `terraform import truenas_ipmi_lan.manual_test <channel>`. `ImportStateVerify` should pass except for `password`/`apply_remote` (write-only / call-time-only, expected to differ).
7. Restore: run the command recorded in step 2 directly (or re-apply Terraform with `ipmi_vlan` set back to the original value/`null` as appropriate).
8. Re-verify via `ipmi.lan.query` (with the same settle-time tolerance as step 5) that `vlan_id`/`ip_address`/`subnet_mask`/`default_gateway_ip_address` all match the ORIGINAL values from step 1.
**Expected results:** Only `vlan` changes across the whole exercise; the BMC remains reachable at its original IP throughout (dhcp/ip/netmask/gateway never touched); final state matches the pre-test original.
**Cleanup:** Confirm step 8 succeeded before ending the session. If any step failed leaving the BMC in an unknown state, run the restore command from step 2 immediately and verify BMC reachability (ping it / load its web UI) before moving on.

**MT-HA-009 — Diagnostic: "vlan" cannot be cleared to null via Terraform**

**Resource:** truenas_ipmi_lan
**Type:** CRUD
**Environment:** Enterprise HA (BMC-equipped) system — reuses the channel from MT-HA-008, which must currently have a non-null `vlan` in Terraform state.
**Preconditions:** MT-HA-008 (or an equivalent prior apply) has left `truenas_ipmi_lan.manual_test` in state with a non-null `vlan`.
**⚠ Safety:** This case intentionally attempts (and expects to fail) a vlan-clearing apply — no BMC network values are changed.
**Config:**
```hcl
resource "truenas_ipmi_lan" "manual_test" {
  channel   = var.ipmi_channel
  dhcp      = var.ipmi_dhcp
  ipaddress = var.ipmi_dhcp ? null : var.ipmi_ipaddress
  netmask   = var.ipmi_dhcp ? null : var.ipmi_netmask
  gateway   = var.ipmi_dhcp ? null : var.ipmi_gateway
  # "vlan" intentionally omitted — the resource previously managed a non-null vlan.
}
```
**Steps:**
1. With state from MT-HA-008 still showing a non-null `vlan`, edit the config to remove the `vlan = var.ipmi_vlan` line entirely (as above) and `terraform apply`.
2. Expect Terraform to return an ERROR diagnostic (not a silent clear, not a crash) explaining that `vlan` cannot be distinguished as "explicitly cleared" vs. "never configured," and instructing you to set `vlan` explicitly to its current value to proceed with unrelated changes.
3. `midclt call ipmi.lan.query ...` — confirm `vlan_id` is UNCHANGED (the failed apply made no API call that touched vlan).
4. To actually clear the VLAN tag (demonstrating the real, out-of-band path): `midclt call ipmi.lan.update <channel> '{"dhcp": <dhcp>, ..., "vlan": null}'` directly.
5. `terraform apply -refresh-only` — confirm Terraform reconciles its state to the new `vlan = null` without a diagnostic.
6. Restore `vlan` back to the value tested in MT-HA-008 the same way (direct `ipmi.lan.update` + `terraform apply -refresh-only`, or a normal `terraform apply` with `vlan` set explicitly).
**Expected results:** Terraform never silently clears `vlan`; the documented diagnostic appears; the only supported way to clear it is a direct API/UI call followed by `terraform apply -refresh-only`.
**Cleanup:** Ensure `vlan` ends the test at the same value it had going in (verify via `ipmi.lan.query`).

**MT-HA-010 — Manual-only DANGEROUS: BMC password rotation**

**Resource:** truenas_ipmi_lan
**Type:** Manual-only DANGEROUS
**Environment:** Enterprise HA (BMC-equipped) system, fully disposable.
**Preconditions:** You have (or can quickly get) an alternate path to the BMC — physical access, a known-good IPMI/Redfish credential, or a vendor reset procedure — in case the new password is lost or the change doesn't take.
**⚠ Safety:** `password` is write-only and NEVER read back by the API (`ipmi.lan.query` has no password field at all, masked or otherwise) — there is no automated way to verify the change succeeded other than logging in with the new password. An incorrect or forgotten password change can lock out legitimate BMC administrative access. Verify login BEFORE ending the session.
**Config:**
```hcl
variable "ipmi_channel" {
  type = number
}
variable "ipmi_new_password" {
  type      = string
  sensitive = true
  # 8-16 chars, ASCII upper/lower/digit/special per the API's own schema.
}

resource "truenas_ipmi_lan" "manual_test" {
  channel   = var.ipmi_channel
  dhcp      = var.ipmi_dhcp   # resend the channel's existing dhcp/ip/netmask/gateway unchanged
  ipaddress = var.ipmi_dhcp ? null : var.ipmi_ipaddress
  netmask   = var.ipmi_dhcp ? null : var.ipmi_netmask
  gateway   = var.ipmi_dhcp ? null : var.ipmi_gateway
  password  = var.ipmi_new_password
  # apply_remote intentionally omitted/false — never push this to the peer controller's BMC.
}
```
**Steps:**
1. Record the CURRENT BMC admin password somewhere safe (you will need it if you must roll back, and to prove you had working access before the change).
2. `terraform apply -var="ipmi_new_password=<new password>"` (plus the channel's existing dhcp/ip/netmask/gateway values, unchanged).
3. IMMEDIATELY attempt to log into the BMC (its own web UI, or `ipmitool -I lanplus -H <bmc ip> -U <user> -P <new password> chassis status`) using the NEW password. Do this before doing anything else.
4. If login succeeds: done — record success and move on.
5. If login FAILS: do not panic-retry blindly. Use your alternate access path (physical/Redfish/vendor reset) to reset the BMC password back to a known value, then confirm access is restored before ending the session.
**Expected results:** The BMC accepts the new password for authentication; no other channel setting (IP/netmask/gateway/vlan) changes.
**Cleanup:** If this was a pure test (not an intended permanent change), reset the BMC password back to its original value using the same procedure, and reconfirm login with the original password before ending the session.

### 6. truenas_enclosure

**MT-HA-011 — Datasource query by id (ids drift across boots)**

**Resource:** truenas_enclosure
**Type:** Datasource
**Environment:** Enterprise HA / enclosure-equipped system.
**Preconditions:** None beyond a reachable box with enclosure hardware (a system with no enclosure license/hardware returns an empty `enclosure2.query` — skip this case there).
**Config:**
```hcl
variable "enclosure_id" {
  type        = string
  description = "Look this up fresh each time — see step 1. Do not hardcode a value across runs."
}

data "truenas_enclosure" "test" {
  id = var.enclosure_id
}
```
**Steps:**
1. Look up a current enclosure id FRESH, every time — never reuse a value recorded from a previous session or a different boot: UI Storage > Enclosures, or `midclt call enclosure2.query` with no filter, and take the `id` of the first result.
2. `terraform apply -var="enclosure_id=<id from step 1>"`.
3. Confirm `name`, `label`, `model`, `vendor`, `product`, `controller`, `status` (a list, e.g. `["OK"]`) are all populated; confirm `front_slots`/`rear_slots`/etc. match the physical (or virtual) chassis.
4. Negative check: `terraform apply -var="enclosure_id=tf-acc-manual-nonexistent-enclosure-id"` — expect a clear "not found" error, not a crash or empty-object read.
**Expected results:** Real id resolves to full hardware detail; a bogus id fails cleanly with a "not found" error.
**Cleanup:** None — read-only. Note for next time: re-probe the id again after any reboot of this box — enclosure ids are not guaranteed stable across boots.

### 7. truenas_enclosure_label

**MT-HA-012 — Set label, verify, destroy restores the original label**

**Resource:** truenas_enclosure_label
**Type:** CRUD
**Environment:** Enterprise HA / enclosure-equipped system.
**Preconditions:** A valid enclosure id (see MT-HA-011, step 1).
**Config:**
```hcl
variable "enclosure_id" {
  type = string
}

resource "truenas_enclosure_label" "manual_test" {
  id    = var.enclosure_id
  label = "tf-acc-manual-enclosure-label"
}
```
**Steps:**
1. `midclt call enclosure2.query '[["id","=","<id>"]]'` — record the CURRENT `label` as ORIGINAL, before touching Terraform.
2. `terraform apply -var="enclosure_id=<id>"`.
3. UI: Storage > Enclosures shows the new label `tf-acc-manual-enclosure-label` for this enclosure; `name` (the fixed hardware name) is unchanged.
4. `midclt call enclosure2.query '[["id","=","<id>"]]'` — confirm `label` matches, `name` unchanged.
5. Import: `terraform state rm truenas_enclosure_label.manual_test`, then `terraform import truenas_enclosure_label.manual_test <id>`. `terraform plan` — expect no changes. (This capture-at-import-time also becomes the new "original" the resource will restore to on a subsequent destroy — confirm this is what you intend before proceeding.)
6. `terraform destroy`.
7. `midclt call enclosure2.query '[["id","=","<id>"]]'` — confirm `label` has been restored to the value recorded in step 1 (the ORIGINAL), not left at `tf-acc-manual-enclosure-label`.
**Expected results:** Label sets and reads back correctly; import captures the current label for its own restore-on-destroy bookkeeping; destroy independently restores the enclosure's original (pre-test) label, verified via a live API read, not just Terraform state.
**Cleanup:** Step 7 IS the cleanup verification — if it does not match, manually restore via `midclt call enclosure.label.set <id> "<ORIGINAL>"`.

### 8. truenas_truecommand_config

**MT-HA-013 — Datasource read (safe, no connection attempt)**

**Resource:** truenas_truecommand_config
**Type:** Datasource
**Environment:** Any TrueNAS system (Enterprise license required for TrueCommand; HA not required).
**Preconditions:** None.
**Config:**
```hcl
data "truenas_truecommand_config" "test" {}
```
**Steps:**
1. `terraform apply`.
2. Confirm `id = "truecommand_config"`, `enabled`, `status`, `status_reason` are set.
3. If `enabled` already reads `true`, this box has a real, pre-existing TrueCommand connection — treat it as read-only context and do NOT proceed to MT-HA-014 against this box without separately confirming that's intended.
**Expected results:** Fields populate; no connection attempt is made by a read.
**Cleanup:** None — read-only.

**MT-HA-014 — Singleton Tier-2 "api_key" set-and-restore, "enabled" always false**

**Resource:** truenas_truecommand_config
**Type:** Singleton
**Environment:** Any TrueNAS system, fully disposable for this box-wide singleton.
**Preconditions:** MT-HA-013 confirms `enabled=false` beforehand.
**⚠ Safety:** `enabled` is ALWAYS explicitly `false` in this test — NEVER set it to `true`; doing so starts a real connection attempt to a TrueCommand instance using `api_key`. `api_key` is Sensitive but NOT write-only — it is stored in Terraform state in plaintext-readable form (masked only in CLI output), so treat any state file touched by this test as sensitive.
**Config:**
```hcl
variable "truecommand_api_key" {
  type      = string
  sensitive = true
  # 16 characters
}

resource "truenas_truecommand_config" "manual_test" {
  enabled = false
  api_key = var.truecommand_api_key
}
```
**Steps:**
1. `midclt call truecommand.config` — record the current `api_key` (may be `null`) as ORIGINAL.
2. `terraform apply -var="truecommand_api_key=abcd1234abcd1234"` (any schema-valid 16-character throwaway string).
3. Confirm `status` stays `DISABLED` / `status_reason` stays `"Truecommand service is disabled."` — i.e. no outbound connection was attempted, since `enabled` stayed `false`.
4. `midclt call truecommand.config` — confirm `api_key` round-tripped verbatim to the throwaway value.
5. Import: `terraform state rm truenas_truecommand_config.manual_test`, then `terraform import truenas_truecommand_config.manual_test truecommand_config`. `terraform plan` — expect no changes.
6. Restore: HCL cannot express clearing `api_key` back to `null` (an omitted attribute and an explicit null are indistinguishable to Terraform) — restore directly: `midclt call truecommand.update '{"api_key": <ORIGINAL, possibly null>}'`.
7. `midclt call truecommand.config` — confirm `api_key` matches ORIGINAL again.
**Expected results:** `api_key` round-trips; `enabled` never changes from `false`; `status` never leaves `DISABLED` during this test.
**Cleanup:** Step 7 verifies the restore — if it doesn't match, re-run the command from step 6.

### 9. truenas_tn_connect_config

**MT-HA-015 — Datasource read only (safe case)**

**Resource:** truenas_tn_connect_config
**Type:** Datasource
**Environment:** Any TrueNAS system.
**Preconditions:** None.
**Config:**
```hcl
data "truenas_tn_connect_config" "test" {}
```
**Steps:**
1. `terraform apply`.
2. Confirm `id = "tn_connect_config"`, `enabled`, `status`, `status_reason`, `registration_details` (a JSON string, `"{}"` when not enrolled) are set.
3. Note expected release-specific nulls, NOT bugs: `tier` / `last_heartbeat_failure_datetime` are populated only on TrueNAS 26.0+ (null on 25.10); `ips` / `interfaces` / `interfaces_ips` / `use_all_interfaces` are populated only on TrueNAS 25.10 (null on 26.0+).
4. If `enabled` already reads `true`, this box is genuinely enrolled with TrueNAS Connect — this is expected on some boxes and this read-only case does not disturb it.
**Expected results:** Fields populate per the box's release; enrollment state is reported accurately without being touched.
**Cleanup:** None — read-only.

**MT-HA-016 — Manual-only DANGEROUS: enabling TrueNAS Connect (starts real cloud enrollment)**

**Resource:** truenas_tn_connect_config
**Type:** Manual-only DANGEROUS
**Environment:** A fully disposable TrueNAS system explicitly authorized for TrueNAS Connect cloud-enrollment testing. Never run this against a box that is not already confirmed `enabled=false` (verify via MT-HA-015 first), and never against production.
**Preconditions:** MT-HA-015 confirms `enabled=false` and `status="DISABLED"` before proceeding. Explicit authorization to enroll this system with the TrueNAS Connect cloud service (iX account/heartbeat infrastructure).
**⚠ Safety:** Setting `enabled=true` is a REAL, EXTERNAL side effect — this system registers with the TrueNAS Connect account service and begins periodic heartbeat reporting to iX cloud infrastructure. This is not a local configuration toggle. `enabled` is the ONLY writable field on this resource; every other attribute is read-only status.
**Config:**
```hcl
resource "truenas_tn_connect_config" "manual_test" {
  enabled = true
}
```
**Steps:**
1. `midclt call tn_connect.config` — confirm `enabled=false`/`status="DISABLED"` before proceeding. Do not continue if this box is already enrolled.
2. `terraform apply`.
3. UI: locate the TrueNAS Connect enrollment page (System Settings > General, or wherever this release surfaces it) — `status` should move from `DISABLED` toward `CONFIGURED`. Completing the actual claim/binding to an iX Portal account may require an additional out-of-band UI step (a claim token / registration URI flow) — this resource's `enabled` toggle only starts the process, it does not drive the full claim flow itself.
4. `midclt call tn_connect.config` — confirm `status`/`registration_details` reflect the new enrollment state.
5. When finished testing: `terraform apply` with `enabled = false` again.
6. `midclt call tn_connect.config` — confirm `status` returns to `DISABLED`.
7. Check whether any registration/account artifact persists on the iX Portal side — disabling locally does not necessarily fully de-register the system from the TrueNAS Connect cloud account; removing it there, if a real enrollment completed, may require a separate manual step in the iX Portal account admin UI, outside the scope of Terraform.
**Expected results:** Enabling starts real enrollment (status transitions away from `DISABLED`); disabling returns local `status` to `DISABLED`; any residual cloud-side account artifact is flagged for manual follow-up rather than assumed cleaned up.
**Cleanup:** Confirm step 6 restored `enabled=false`/`status=DISABLED` locally. If a real iX Portal account binding occurred in step 3, follow up in the iX Portal itself to remove/de-register it — do not consider this test fully cleaned up until that is confirmed or explicitly waived by the environment owner.

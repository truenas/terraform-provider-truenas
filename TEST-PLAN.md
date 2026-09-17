# terraform-provider-truenas — Project Test Plan

**Audience:** QA and engineering
**Scope:** the entire provider — 85 resources, 87 data sources, the WebSocket
JSON-RPC client, and the acceptance-test infrastructure
**Companion documents:** `TESTING.md` (operator quick reference), `SCRAM.md`
(authentication implementation + test description), `API-COVERAGE.md`
(coverage status vs. the middleware API surface)

---

## 1. Purpose

This document is the reference for how the provider is tested: the
philosophy behind the test design, every test level and gate, the live
environments and what may run where, the per-domain coverage matrix, safety
rules that protect production systems, and the runbooks used for routine
and release verification. TESTING.md tells an operator which commands to
run; this document explains the whole system and the reasoning, so a new
engineer or an auditor can evaluate and extend it.

## 2. Test philosophy

1. **No mocks, anywhere.** There are no mock or simulated TrueNAS servers in
   the repository — not in the acceptance suite and not in the client
   package. Unit tests are pure-function tests. Every test that talks to a
   server talks to a real TrueNAS system. This policy is absolute and was a
   deliberate project decision: middleware behavior (validation quirks,
   translation layers, job semantics, version drift) is the very thing mocks
   get wrong.
2. **Probe before code.** Every wire assumption — field names, job flags,
   response shapes, secret read-back behavior, error text — is verified
   against the live API (`go run ./cmd/debug_api/ methods <method>`) before
   the code that depends on it is written. Probe evidence is recorded in
   code comments and schema descriptions, not just in reports. Every wire
   mismatch this project ever hit came from trusting a stale or assumed
   shape.
3. **Evidence over assertion.** A test that cannot run in an environment
   must skip with an explanation, never silently pass. Documented-skip tests
   carry the decisive probe transcript in the test file itself. Restores are
   verified by reading the system back, not assumed.
4. **Secrets are modeled from observed behavior.** Whether a secret field is
   WriteOnly (never in state) or Sensitive (in state, masked) is decided by
   probing what the API actually returns — some "secrets" (certificates'
   private keys, keytabs, SSH private keys) are returned intact and are
   modeled Sensitive; others (user passwords, bind passwords, registry
   passwords) never come back and are WriteOnly.
5. **Singleton payloads are config-driven.** Optional+Computed fields on
   singleton resources are only sent to the API when the user explicitly
   configured them (sourced from `req.Config`, never from the plan, which
   echoes prior state). This prevents silent reverts of out-of-band changes
   and is enforced by unit tests in every singleton package.

## 3. Test levels

| Level | Count (at time of writing) | Needs a box? | Command |
|---|---|---|---|
| Unit | 1103 test functions / 90 packages | No | `make test` / `go test ./...` |
| Live client tests | `TestLive*` in `internal/client` | Yes (any) | env-gated, run with acceptance env |
| Acceptance Tier 1 (safe CRUD) | bulk of 125 acceptance functions | Yes | `make testacc-safe` |
| Acceptance Tier 2 (disruptive-lite singletons) | 24 packages | Yes | `make testacc-disruptive` |
| Special-gated suites | DS (directory services), HA, Apps | Yes (specific) | env gates below |
| Documented-skip / manual | 8 packages | n/a | in-file rationale + enable instructions |
| Scripted one-off verifications | recorded in reports | Yes | not committed as tests |

### 3.1 Unit tests (no TrueNAS required)

Every resource package carries:
- **Payload builders** — verbatim wire keys; unset optionals omitted;
  three-way nullable handling (null/unknown → omit, explicit zero → JSON
  null where applicable); create-only keys absent from updates;
  conditional-required preflight diagnostics.
- **Response mappers** — probed response shapes, including dual-shape
  decoders where releases differ; secrets never populated from responses.
- **Schema shape** — Required/Optional/Computed per attribute, plan
  modifiers, Sensitive/WriteOnly flags, validators.
- **Datasource reflection** — `tfsdk` tags must match, field for field, the
  datasource schema attribute set (catches a bug class invisible to all
  other unit tests).
- **Safety contracts** where applicable — e.g. `twofactor_auth` and
  `tn_connect_config` unit tests prove `enabled` is never emitted unless
  explicitly configured.

The `internal/client` package's unit tests are pure functions only (error
classification, version comparison, SCRAM message construction/validation).
All client interaction behavior is tested live (`TestLive*`): call
round-trips, real not-found mapping, context cancellation, auth
(password, plain API key, SCRAM positive and negative), job-poll bailout,
reconnect-after-transport-drop.

### 3.2 Acceptance Tier 1 — safe CRUD

Contract for every Tier 1 test:
1. Create with `tf-acc-`-prefixed randomized names (`acctest.RandName`),
   fixtures (datasets, users, containers) created in the same config via
   resource references.
2. Update in place, verifying the changed attribute.
3. `ImportState` with `ImportStateVerify` (write-only/non-echoed fields in
   `ImportStateVerifyIgnore`, each justified by probe evidence).
4. Destroy plus `CheckDestroy` that queries the live API and fails if the
   object survived.

Tier 1 never references pre-existing objects on any box.

### 3.3 Acceptance Tier 2 — singleton set-and-restore

Singletons cannot be created/destroyed. The pattern:
1. Read the current live config via the API before any change.
2. Register an API-level restore in `t.Cleanup` (via `acctest.RestoreCall`,
   which retries across transport drops) **before** the first mutation.
3. Apply a cosmetic change, verify, re-apply the original (update-back is
   exercised too).
4. Unconfigured-singleton self-skips where the API cannot restore an
   unconfigured state (mail, ups).

### 3.4 Special-gated suites

| Gate | Env vars | What it protects |
|---|---|---|
| Directory services | `TRUENAS_DS=1` + `TRUENAS_DS_ALLOWED_ENDPOINT` (must equal the endpoint; fatal otherwise) | Domain joins change box authentication; only disposable boxes may join. AD/LDAP/IPA joins, keytabs. |
| HA / Enterprise | `TRUENAS_HA=1` + `TRUENAS_HA_ALLOWED_ENDPOINT` + live `failover.licensed` probe (clean skip when unlicensed) | failover, IPMI, enclosure tests run only on the designated HA system |
| Apps | `TRUENAS_APPS=1` | container-image pulls are slow/heavy |
| ACME | `TRUENAS_ACME=1` + `TRUENAS_ACME_DIRECTORY` + `TRUENAS_ACME_CHALLTESTSRV` + `TRUENAS_ACME_CA_PEM` (all three fatal if missing) | live end-to-end ACME issuance (`TestAccCertificate_acmeIssuance`) drives a real DNS-01 order against a Pebble ACME CA; needs the Pebble + challtestsrv environment |
| Disruptive | `TRUENAS_DISRUPTIVE=1` | enables Tier 2 |

### 3.5 Documented-skip and manual tests

Unconditionally skipped tests each carry the decisive probe transcript and
re-enable instructions in the test file:

| Package | Why it cannot run here |
|---|---|
| `app_registry` | `app.registry.create` validates credentials against the real container registry; no registry fixture |
| `cloud_backup` | create validates the credential/bucket against real cloud storage |
| `vmware` | create validates against a live vCenter/ESXi (ENETUNREACH/ETIMEDOUT transcripts on both releases) |
| `tn_connect_config` (write path) | only writable field is `enabled`; enabling starts cloud enrollment; production box is already enrolled |
| `twofactor_auth` (enabled flip) | committed test mutates `window` only; the enabled flip was verified once as a scripted one-off |
| `pool` (creation) | needs dedicated blank disks; datasource + manual creation procedure |
| `network_interface` (bridge write) | global commit/checkin cycle can cut management access; enp7s0 datasource test runs |
| `directoryservices` on non-DS boxes | gated as above |
| Never-run tier (`network_config`, `system_general`, `ssh_config`, `system_dataset` write paths) | can cut management access or migrate system state; each has in-file rationale and manual instructions |

### 3.6 Scripted one-off verifications (not committed)

High-risk behaviors verified once, with full transcripts in task reports:
- **Real HA failover** — `failover.become_passive` executed on the
  disposable HA pair; status transitions polled through the event; system
  verified healthy after; found and documented the "master flag unreliable
  post-failover" API quirk.
- **2FA global enable** — proved API-key auth is unaffected by
  `auth.twofactor` `enabled=true`, then restored.
- **TrueCommand api_key-only update** — proved side-effect-free with
  `enabled=false` on both releases before the Tier 2 test was allowed to
  exist.

## 4. Test environments

The plan requires six environment roles. Addresses, credentials, and realm
names are supplied at run time through the environment variables in §3.4 and
TESTING.md — nothing in the suite is tied to a particular lab.

| Role | Requirements | Used for | Disposable? |
|---|---|---|---|
| Primary test box | TrueNAS on the older supported release line (currently 25.10.x); a scratch pool | Tier 1/Tier 2, full regression, DS-test target | Yes — required (DS joins change box auth) |
| Cross-release box | TrueNAS on the newer release line (currently 26.0); a scratch pool | Cross-release verification; newer-release-only features (LXC, containers, webshare) | Safe-tier only if it serves real workloads; never DS joins; never touch its pre-existing objects |
| Enterprise HA pair | HA-licensed TrueNAS system | failover/IPMI/enclosure/HA-gated suites; the one-off real-failover exercise | Yes — required for the failover exercise |
| Samba AD domain controller | Any host running Samba as an AD DC, DNS answering for its realm, reachable from the primary box | ACTIVEDIRECTORY joins, kerberos realm/keytab tests (keytabs exported from the DC) | Yes |
| OpenLDAP server | slapd with RFC2307 schema, TLS (ldaps + StartTLS), seeded posixAccount/posixGroup entries | LDAP service-type joins and user-visibility checks | Yes |
| FreeIPA server | FreeIPA with its own DNS for its realm | IPA service-type joins | Yes |
| Pebble ACME CA | letsencrypt/pebble (real ACME server) + pebble-challtestsrv (DNS-01), reachable from the test box on the same subnet | live ACME issuance test (`TRUENAS_ACME` gate): the box orders a cert, runs the DNS-01 shell authenticator that publishes to challtestsrv, Pebble validates + issues | Yes |

Release-coverage rule: every resource is verified on both 25.10 and 26.0
unless the namespace is release-specific, in which case a version gate
(`client.VersionAtLeast`) produces a clean diagnostic on the other release
and the acceptance test skips via a live version probe. Current
26.0-only surfaces: `lxc_config`, `container`, `container_device`,
`container_image`, `docker_network`, `webshare`, `webshare_config`.
HA-only: `failover_config`, `ipmi_lan`, `enclosure`, `enclosure_label`
(gated by license probe, not version).

Hard-learned environment facts baked into the suite:
- Auth rate limit ~20 logins/60s/IP → packages run sequentially with
  30-second sleeps; parallel suites will trip it (observed: burst runs
  produce `[EBUSY] Rate Limit Exceeded` and `not connected` cascades that
  read like real failures — always re-run failures individually and paced
  before believing them).
- The 25.10 introspection API under-reports some namespaces
  (`core.get_methods` listed 3 of 13 failover methods); verify by direct
  call, never by listing alone.
- BMC (IPMI) settings can transiently read `0.0.0.0` for ~seconds after an
  update; the resource and tests poll for convergence (bounded).
- The container image registry prunes old builds; never hardcode image
  versions (use the `truenas_container_image` datasource).

## 5. Coverage matrix (by domain)

Tier legend: **T1** = full CRUD contract, **T2** = set-and-restore,
**DS/HA/Apps** = special gate, **doc-skip** = documented skip, **manual** =
documented manual procedure, **26.0** = version-gated.

| Domain | Resources | Tier / gates | Notes |
|---|---|---|---|
| Storage | pool, dataset, zvol, snapshot, periodic_snapshot, scrub_task, resilver_config, system_dataset | T1 (pool: datasource + manual create; system_dataset: never-run write) | scrub_task self-skips when the pool already has a schedule (one per pool) |
| Shares | nfs, smb, webshare (26.0), nfs_config, smb_config, webshare_config (26.0) | T1 + T2 | SMB exercises 26.0 purpose/options mapping |
| iSCSI | global, portal, initiator, auth, extent, target, targetextent | T1 + end-to-end wiring test | never touches a box's pre-existing portal/target/extent objects |
| NVMe-oF | global, subsys, port, namespace, host, host_subsys, port_subsys | T1 + end-to-end | test ports 14420/14421 disabled; id=1 objects untouchable |
| Accounts & access | user, group, api_key, privilege, twofactor_auth | T1 + T2 | api_key test re-authenticates a fresh client with the created key (SCRAM on 26.0); 2FA committed test never flips enabled |
| Directory services | directoryservices (AD/LDAP/IPA), kerberos_config/realm/keytab, idmap (via AD block) | DS gate; T1/T2 for kerberos | real joins against all three server types; destroy disables (never leaves); keytab tests use a real DC-exported keytab |
| Certificates | certificate, acme_dns_authenticator | T1 + ACME gate | in-test Go stdlib self-signed certs; live end-to-end ACME issuance covered by `TestAccCertificate_acmeIssuance` (DNS-01 shell authenticator against a Pebble CA); only renewal polling still uncovered |
| Keychain & replication | keychain_ssh_keypair, keychain_ssh_connection, replication (+SSH), replication_config | T1 | loopback SSH replication on the 25.10 VM (real SSH transport, single box) |
| Tasks | cronjob, init_shutdown_script, rsync_task, cloudsync, cloudsync_credentials, cloud_backup | T1 (cloud_backup doc-skip) | task commands are `/usr/bin/true`, enabled=false |
| Filesystem | filesystem_permissions, filesystem_acl, acl_template | T1 | path-keyed wrap-an-action pattern; destroy semantics documented per probe (permissions persist; ACL strips) |
| Apps & containers | app (Apps gate), app_registry (doc-skip), docker_config, docker_network (ds-only), catalog_config, container (26.0), container_device (26.0), container_image (ds-only, 26.0), lxc_config (26.0) | T1/T2/26.0 | docker pool never set/changed by tests; container fixtures never autostart; GPU device type deliberately unmodeled (no hardware evidence) |
| Virtualization | vm, vm_device, vmware (doc-skip) | T1 | VMs created stopped |
| Network | network_config (never-run write), network_interface (manual write / ds test), static_route | T1/manual | TEST-NET addresses only |
| System | system_general (never-run write), system_advanced, tunable, boot_environment, service, ntp_server | T1 + T2 | boot_environment clones the active BE, never activates; tunable restores orig_value |
| Service configs | ssh_config (never-run write), ftp_config, snmp_config, ups_config, mail | T2 | mail/ups self-skip when unconfigured |
| Alerts & reporting | alert_service, alert_policy, reporting_exporter, audit_config | T1 + T2 | exporter targets TEST-NET, disabled |
| HA / Enterprise | failover_config, ipmi_lan, enclosure (ds-only), enclosure_label, truecommand_config, tn_connect_config | HA gate + T2 | real failover verified once (scripted); enclosure label restore-on-destroy verified live; truecommand/tn_connect never enable enrollment |

Data-source coverage: every resource has a matching datasource except the
three datasource-only surfaces (enclosure, docker_network,
container_image); each carries the reflection test and, where a live read
is possible, a datasource acceptance test.

## 6. Safety rules (non-negotiable)

1. Production-serving systems (currently the 26.0 box) get safe-tier tests
   only. Never: DS joins, docker/LXC pool changes, touching pre-existing
   objects (portal/target/extent id=1, NVMe subsys id=1, enp7s0, existing
   certificates including the UI cert id=1, builtin ACL templates,
   builtin privileges, existing registries/containers/shares).
2. Allowed-endpoint guards (`TRUENAS_DS_ALLOWED_ENDPOINT`,
   `TRUENAS_HA_ALLOWED_ENDPOINT`) make dangerous suites fail closed: the
   gate fatals unless the operator explicitly names the endpoint.
3. Restores are registered before mutations and verified after
   (`acctest.RestoreCall` retries across transport drops).
4. Cloud-enrollment toggles (`tn_connect_config.enabled`,
   `truecommand_config.enabled`) are never set true by committed code paths;
   unit tests enforce the payload contract.
5. Every created object is `tf-acc-`-prefixed and randomized; `CheckDestroy`
   proves cleanup; leftover sweeps use `cmd/debug_api` queries.
6. Test keys are minted per plan and revoked at plan end, with
   auth-rejection proof.

## 7. Credentials and secrets policy

- No credentials are ever committed. API keys, passwords, and keytabs live
  in environment variables at run time; generated infrastructure passwords
  live in root-only files on the hypervisor host, outside the repository.
- Scratch/reports (`.superpowers/`, session scratchpads) are git-ignored;
  keys that appear there are revoked when their plan closes, making stale
  copies inert.
- One credential incident occurred and is part of the record: a real
  domain-admin keytab was committed inside a unit test; caught by the final
  whole-branch review, replaced with synthetic bytes, and the domain
  password rotated (old keytab in git history is a dead credential). The
  lesson is codified: secret-bearing test fixtures must be synthetic, and
  reviews grep diffs for credential material.

## 8. Runbooks

```sh
# Unit only (no box)
make test

# Tier 1 safe suite (box required)
export TRUENAS_ENDPOINT=wss://<box>/api/current TRUENAS_API_KEY=<key> TRUENAS_TEST_POOL=tank
make testacc-safe

# Tier 2 singletons
make testacc-disruptive

# Directory services (disposable box + DS servers; see TESTING.md for env)
TRUENAS_DS=1 TRUENAS_DS_ALLOWED_ENDPOINT=$TRUENAS_ENDPOINT \
TRUENAS_DS_DOMAIN=<realm> TRUENAS_DS_USER=<domain admin> TRUENAS_DS_PASSWORD=... \
go test ./internal/resources/directoryservices/ -run TestAcc -v -count=1 -timeout 30m

# HA suite (licensed HA system only)
TF_ACC=1 TRUENAS_HA=1 TRUENAS_HA_ALLOWED_ENDPOINT=$TRUENAS_ENDPOINT TRUENAS_DISRUPTIVE=1 \
go test ./internal/resources/failover_config/ ./internal/resources/ipmi_lan/ \
        ./internal/resources/enclosure/ ./internal/resources/enclosure_label/ -v -count=1

# Full regression (sequential, paced — ~50 min)
#   scripts iterate all packages with sleep 30 between; run detached.
```

Operational rules: sequential packages, 30s pacing; long sweeps detached
(`setsid`) with log polling; failed packages re-run individually and paced
before being treated as real failures.

## 9. Load testing

Load tests verify the provider degrades gracefully under TrueNAS API rate
limiting — across authentication, read, and mutating calls, under realistic
Terraform parallelism and concurrent runs — and characterize the observed
limits. Tests run only against a designated disposable box and require
explicit opt-in.

**Safety gate:** All load tests check `TRUENAS_LOAD=1` **and**
`TRUENAS_LOAD_ALLOWED_ENDPOINT` exactly equal `TRUENAS_ENDPOINT`, else they
fatal. This mirrors the `HACheck`/`DSCheck` pattern and prevents accidental
hammering of shared or production boxes. Tests self-skip without the gate set,
so the default suite stays green.

**Environment configuration:**

| Variable | Default | Meaning |
|---|---|---|
| `TRUENAS_LOAD` | unset | Master gate; must be set to enable load tests |
| `TRUENAS_LOAD_ALLOWED_ENDPOINT` | — | Must equal `TRUENAS_ENDPOINT`; test fatals if mismatch |
| `LOAD_AUTH_CONNS` | 40 | Concurrent login attempts in `TestLoad_AuthBurst` |
| `LOAD_CALL_CONCURRENCY` | 50 | Concurrent API calls in `TestLoad_CallSaturation` |
| `LOAD_RESOURCES` | 150 | Number of datasets in `TestLoad_TerraformApply` |
| `LOAD_PARALLELISM` | 20 | terraform apply `-parallelism` value |
| `LOAD_CONCURRENT_APPLIES` | 6 | Simultaneous independent applies in `TestLoad_ConcurrentApplies` |
| `LOAD_DURATION` | 10m | Loop duration for `TestLoad_Sustained` |
| `LOAD_JOB_COUNT` | 30 | Datasets pre-created for job-saturation stress in `TestLoad_JobSaturation` |
| `LOAD_MIXED_PER_TYPE` | 10 | Number of each resource type in `TestLoad_MixedResources` |
| `TRUENAS_TEST_POOL` | `tank` | Pool where load-test objects are created |

**Seven tests:**

- **`TestLoad_AuthBurst`** — launches `LOAD_AUTH_CONNS` concurrent `Connect()`
  calls to prove that the client's `WithRetry` backoff (5s, 10s, 20s, 30s)
  absorbs the ~20 login/minute authentication ceiling. Asserts all logins
  eventually succeed; reports observed login rate and aggregate backoff
  absorbed.

- **`TestLoad_CallSaturation`** — fires `LOAD_CALL_CONCURRENCY` concurrent
  operations across read (`CallRead`), write (`Call`), and job-backed
  (`CallJob`) paths against a shared connection. Reads self-heal via retry;
  writes and jobs may surface rate-limits. The test records surfaced errors as
  findings (a characterization output, not a failure) to inform whether
  write/job-path retry needs to be added; it only fails on genuine errors or
  cleanup issues.

- **`TestLoad_TerraformApply`** — applies and destroys a `LOAD_RESOURCES`
  dataset config at `LOAD_PARALLELISM` parallelism. Asserts the apply
  converges (exit 0), all datasets are created and queryable, destroy
  succeeds, and no `Rate Limit Exceeded` message reaches the user.

- **`TestLoad_ConcurrentApplies`** — runs `LOAD_CONCURRENT_APPLIES` independent
  applies in separate workspaces simultaneously, stacking authentication
  pressure as if multiple CI pipelines were running. Asserts all runs converge
  and destroy cleanly.

- **`TestLoad_Sustained`** — runs apply/destroy in a loop for `LOAD_DURATION`,
  proving the provider sustains repeated cycles without cumulative
  degradation, object leaks, or per-iteration slowdown.

- **`TestLoad_JobSaturation`** — pre-creates `LOAD_JOB_COUNT` (default 30)
  datasets, then deletes them all concurrently via `CallJob` (each submits a
  job and polls `core.get_jobs`), stressing the job and poll path against the
  client's concurrency limiter. Passes when no errors surface.

- **`TestLoad_MixedResources`** — drives eight resource types (dataset, zvol,
  snapshot, smb_share, nfs_share, user, group, cronjob), `LOAD_MIXED_PER_TYPE`
  (default 10) each, through apply → in-place update → destroy at
  `LOAD_PARALLELISM`.

**Reports and cleanup:**

Every test creates objects prefixed `tf-load-`. Characterization reports
(Markdown + JSON) land in `results/` (gitignored), showing login ceiling,
per-call throttle onset, backoff totals, throughput, and the count of
surfaced rate-limit errors in write/job paths. A standalone cleanup target
(e.g. via `make loadtest-sweep`) removes datasets, zvols, snapshots, SMB
shares, NFS shares, users, groups, and cronjobs prefixed `tf-load-`.

**Running the suite:**

Use `scripts/run-loadtest.sh`. It reads `TRUENAS_ENDPOINT` / `TRUENAS_API_KEY`
/ `TRUENAS_USERNAME` (/ `TRUENAS_TEST_POOL`) from the environment — or from a
git-ignored `.load-test.env` at the repo root that it sources — and sets the
`TRUENAS_LOAD` gate with the allowed-endpoint guard pointed at your box, so a
run can never target anything but the configured disposable box.

```sh
# One-time: create a git-ignored .load-test.env at the repo root:
#   TRUENAS_ENDPOINT=wss://<disposable-box>/api/current
#   TRUENAS_API_KEY=<id>-<secret>
#   TRUENAS_USERNAME=<key owner>
#   TRUENAS_TEST_POOL=tank

scripts/run-loadtest.sh all         # everything (client + Terraform)
scripts/run-loadtest.sh client      # auth-burst + call-saturation + job-saturation
scripts/run-loadtest.sh tf          # Terraform apply / concurrent / sustained / mixed
scripts/run-loadtest.sh jobsat      # just TestLoad_JobSaturation
scripts/run-loadtest.sh mixed       # just TestLoad_MixedResources (needs terraform)
scripts/run-loadtest.sh sweep       # delete stranded tf-load- objects

# Scale knobs are env overrides, e.g.:
LOAD_MIXED_PER_TYPE=5 LOAD_PARALLELISM=25 scripts/run-loadtest.sh mixed
```

The underlying `make` targets (`loadtest`, `loadtest-client`, `loadtest-tf`,
`loadtest-sweep`) still work if you set `TRUENAS_LOAD=1` and
`TRUENAS_LOAD_ALLOWED_ENDPOINT=$TRUENAS_ENDPOINT` yourself.

## 10. Regression cadence and release verification

- **Per change:** unit suite + the affected packages live on the primary
  box; both releases when the change touches shared client/schema code.
- **Per plan (batch of resources):** full sequential regression on the
  25.10 VM (zero failures required), new packages verified on both
  releases, special gates run on their designated systems, docs counts
  recounted from HEAD.
- **Per TrueNAS release:** re-probe method schemas for every namespace the
  provider touches (`cmd/debug_api methods`), diff against recorded shapes,
  gate or dual-shape as needed, then full regression. History shows each
  release moves something (legacy endpoint translation, smb purpose rework,
  cloudsync credential shapes, iscsi field changes, introspection gaps).
- **Client protocol:** any change to `internal/client` re-runs the
  `TestLive*` suite against both releases (SCRAM tests exercise 26.0,
  fallback on 25.10).

## 10. Known middleware findings (upstream)

Documented in reports and worth tracking with iX:
1. `kerberos.update` crashes server-side (`list index out of range`) on any
   `appdefaults_aux`/`libdefaults_aux` line not shaped `key = value`.
2. Stale `kerberos_realm` survives `directoryservices` service-type
   switches (datastore compress/extend asymmetry); provider carries a
   two-call reset workaround.
3. 25.10 `core.get_methods` under-reports namespaces (failover: 3 of 13).
4. `failover.config.master` is not a reliable live-state indicator
   immediately after a failover event.
5. `container.image.query_registry` can list versions whose artifacts the
   upstream registry has already pruned (404 on use).
6. ACME registration reuse hinges on a trailing slash: `acme.registration`
   stores the directory URI slash-normalized but the reuse-lookup matches the
   raw URI passed to `certificate.create`, so a slash-less directory re-registers
   and then fails "already exists" on the second issuance; there is no public
   `acme.registration.delete`. The ACME test normalizes the directory to a
   trailing slash so the one account is reused (see MIDDLEWARE-FINDINGS.md).

## 11. Backlog / future coverage

- FC (`fc`, `fcport`): needs FC-capable hardware (`fc.capable=false` on the
  available HA system).
- JBOF: needs a licensed shelf (`jbof.licensed=0`).
- RDMA: needs capable NICs (namespace empty).
- ACME renewal polling: live end-to-end issuance is now covered
  (`TestAccCertificate_acmeIssuance`, ACME gate, against a Pebble CA — see §10
  finding 6); only the `renew_days`-driven auto-renew cycle remains untested.
- cloud_backup / app_registry / vmware live write paths: need real cloud
  bucket / container registry / vCenter fixtures respectively.
- container_device GPU type: needs GPU hardware.
- Next-gen ZFS namespaces (`zfs.resource*`, `zpool*`): deliberately not
  adopted while the stable `pool.*` namespaces remain primary; revisit on
  deprecation signals.

## 12. Maintenance of this plan

Update this document when: a new environment joins or leaves the lab; a new
gate or tier is added; a release-verification pass changes recorded wire
shapes; the coverage matrix gains a domain. Counts (test functions,
packages) live in TESTING.md and are recounted mechanically at each docs
pass — this plan intentionally describes structure, not counts, except in
§3's snapshot table.

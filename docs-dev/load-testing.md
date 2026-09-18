# Load Testing

What the provider's load tests do, what TrueNAS limits they exercise, how they
assert, and what they do not yet cover. For the runbook (env, commands), see
TEST-PLAN.md §9 and `scripts/run-loadtest.sh`.

## Why

Terraform providers that talk to TrueNAS get tripped up by two server-side
limits when a plan touches many resources or many runs hit one box at once.
These tests reproduce both under load, prove the provider degrades gracefully
(the user never sees the limit), and characterize the behavior. They also
drove the client hardening described under "What these tests changed."

## The two TrueNAS limits under test

1. **Auth login rate limit.** TrueNAS throttles authentication at roughly
   20 logins per minute. Exceeding it returns JSON-RPC `code 16
   [EBUSY] Rate Limit Exceeded` on `auth.login` / `auth.login_with_api_key`
   / `auth.login_ex`. Every Terraform run opens a fresh connection and logs
   in once, so a fan-out of parallel runs (CI) trips it.
2. **Concurrent-call cap.** TrueNAS caps in-flight RPCs per WebSocket
   connection at about 20. Exceeding it returns `code -32000 "Maximum number
   of concurrent calls (20) has exceeded"`. The provider uses one connection
   per run, so Terraform parallelism above ~20 (or many multi-call resources)
   trips it.

Neither is the other: `code 16` is auth-only; `code -32000` is per-connection
concurrency. The tests treat them separately.

## Two layers

- **Client-level** (`internal/client/load_test.go`, external `client_test`
  package) — drives `internal/client` directly with many goroutines. Fast,
  isolates the limit + retry/queue behavior, no Terraform involved.
- **Terraform end-to-end** (`test/load/`) — generates HCL, builds the
  provider, and drives real `terraform apply/destroy` through `dev_overrides`.
  Slower, but exercises the whole provider stack the way a user does.

Shared machinery lives in `internal/loadtest/`: the `LoadCheck` safety gate,
a concurrency-safe `Metrics` collector, a Markdown+JSON `Report` writer, and
`Sweep` (cleans every `tf-load-` object across all resource types).

## The tests

### Client-level

**`TestLoad_AuthBurst`** — launches `LOAD_AUTH_CONNS` (default 40) concurrent
logins within a one-minute window, deliberately crossing the ~20/min auth
limit. Each login goes through the client's `WithRetry` (jittered backoff).
Passes only when every connection eventually authenticates — i.e. the auth
throttle is absorbed by retry, never surfaced. Reports rate-limit hits,
retries, and the failed count.

**`TestLoad_CallSaturation`** — on one connection, fires
`LOAD_CALL_CONCURRENCY` (default 50) concurrent operations across all three
call entrypoints: `CallRead` (`system.version_short`), `Call`
(`pool.snapshot.create/delete`), and `CallJob` (`pool.dataset.delete`). With
50 in flight and a 20 cap, this is the concurrency-cap stressor. Passes when
no path surfaces a failure. (Before the concurrency limiter was added, ~30 of
50 failed with `-32000`.)

**`TestLoad_JobSaturation`** — pre-creates `LOAD_JOB_COUNT` (default 30)
datasets, then deletes them all concurrently with `CallJob`. Each delete
submits a job and then polls `core.get_jobs` every 2s, so this holds many
jobs open at once and adds heavy poll-call pressure — and every poll call
counts against the same 20-concurrent cap. Passes when all jobs complete with
no surfaced error. This exercises the job path plus the poll/cap interaction
that a job-heavy real apply (many replications, deletes) produces.

**`TestLoadSweep`** — the standalone `make loadtest-sweep` entry point;
deletes any stranded `tf-load-` objects after a crashed run. Not a stressor,
a cleanup tool.

### Terraform end-to-end

**`TestLoad_TerraformApply`** — generates `LOAD_RESOURCES` (default 150)
datasets and applies at `-parallelism=LOAD_PARALLELISM` (default 20), then
destroys. Asserts the apply converges, no `Rate Limit Exceeded` reaches the
user, and destroy is clean. Captures `TF_LOG` for retry evidence.

**`TestLoad_ConcurrentApplies`** — runs `LOAD_CONCURRENT_APPLIES` (default 6)
independent applies at once, in separate workspaces, to stack auth pressure
the way several CI pipelines against one box would. Every run must converge
and destroy cleanly.

**`TestLoad_Sustained`** — loops apply/destroy for `LOAD_DURATION`
(default 10m), then asserts zero leaked `tf-load-` objects remain. Catches
cumulative degradation, connection/session drift, and cleanup leaks over time.

**`TestLoad_MixedResources`** — the realistic-config stressor. Generates
`LOAD_MIXED_PER_TYPE` (default 10) instances each of eight resource types with
wired dependencies — `truenas_dataset`, `truenas_zvol`, `truenas_snapshot`,
`truenas_smb_share`, `truenas_nfs_share`, `truenas_user`, `truenas_group`,
`truenas_cronjob` — then drives them through **apply → in-place update →
destroy**. The update leg re-applies the same resources with a changed
attribute (so it is a real in-place update, not a replace), exercising the
Read and Update paths across many resource types under load, not just
Create/Delete of one type. Runs at parallelism deliberately set above the old
cap in verification.

## Methodology

- **No mocks.** Every live test hits a real, disposable TrueNAS box.
- **Safety gate.** All live tests call `loadtest.LoadCheck(t)`: they run only
  when `TRUENAS_LOAD=1` and `TRUENAS_LOAD_ALLOWED_ENDPOINT` equals
  `TRUENAS_ENDPOINT`, so a run can never target anything but the box you
  designate. Without the gate the tests self-skip, so the normal unit suite
  and CI stay green with no box.
- **Object hygiene.** Every created object is prefixed `tf-load-`. `Sweep`
  queries and deletes datasets/zvols (and their snapshots), SMB/NFS shares,
  cronjobs, users, and groups by that prefix; it runs in `t.Cleanup` and as a
  standalone target, so a crashed run strands nothing.
- **Two deliverables per run.** *Graceful degradation* — the assertions:
  no limit reaches the user, everything converges, clean destroy, no leaks.
  *Characterization* — a Markdown+JSON report per run under `results/`
  (git-ignored) with attempts, rate-limit hits, retries, backoff, throughput,
  and a sample error string per failure class.
- **Scale is env-parameterized** (see TEST-PLAN.md §9), so the same tests run
  as a quick smoke pass or a full-scale soak.

## What these tests changed (provider hardening)

Load testing surfaced both limits reaching the user; the fixes live in
`internal/client`:

- **In-flight concurrency limiter** — `Client.sem` (default 16,
  `WithMaxConcurrentCalls` to override) queues calls in `Call` so the client
  never exceeds the server's ~20 cap; excess work waits instead of being
  rejected with `-32000`. `-32000` is also classified transient so any that
  slip through are retried.
- **Auth backoff jitter + headroom** — `WithRetry` jitters each backoff
  (`5/10/20/30/45/60s` + a random fraction) so parallel logins de-synchronize
  instead of retrying in lockstep.

The load tests are the acceptance harness for both: `CallSaturation` /
`MixedResources` verify the concurrency fix, `AuthBurst` verifies the auth
fix. See `internal/client/{client,auth,callread}.go` and the unit tests in
`callread_internal_test.go` / `hardening_internal_test.go`.

## Coverage

Covered:
- Auth login rate limit (`code 16`) under a concurrent-login storm.
- Per-connection concurrent-call cap (`code -32000`) under call and job
  saturation.
- The job submit + `core.get_jobs` poll path under saturation.
- A multi-resource-type config (eight types) with dependencies, through
  Create, in-place Update, and Delete.
- Sustained apply/destroy over time with a leak check.
- Multiple concurrent Terraform runs against one box.

Not yet covered (deliberately deferred):
- **Cross-release** — verified against TrueNAS 26.0 only; 25.10 limits/codes
  could differ.
- **Fault injection / HA failover under load** — network drops mid-load,
  box restart, or a controlled failover during a run.
- **Long soak** — beyond the default 10-minute sustained window.
- **Read/plan-time load** — `terraform plan`/refresh on very large existing
  state.
- **Per-method / boundary characterization** — individual methods may carry
  their own limits; the tests do not map the specific thresholds.

## Results reference

Reports land in `internal/client/results/` and `test/load/results/`
(git-ignored). A passing client-level report has no `## Failures by class`
section; a failing one lists per-class counts and a sample error. Full-scale
verification (2026-08-12, TrueNAS 26.0): all seven load tests pass — auth
absorbed, zero `-32000`, sustained run leak-free, mixed apply/update/destroy
clean.

# TrueNAS Middleware Findings

Server-side issues found while building and testing
terraform-provider-truenas. None are provider bugs — each is a behavior in
the TrueNAS middleware itself, recorded here so they can be filed
upstream with iX. Where the provider carries a workaround, it is noted.

Releases referenced: TrueNAS 25.10.3.1, 25.10.4 (Enterprise HA),
26.0.0-BETA.2, 26.0.0-BETA.3 and 28.0. Each finding lists the release(s) where it was observed.

---

## 1. `kerberos.update` crashes on a malformed aux-parameter line

**Severity:** functional — an unhandled server-side exception, not a
validation error.
**Observed on:** TrueNAS 25.10.3.1.
**Namespace/method:** `kerberos.update` (fields `appdefaults_aux`,
`libdefaults_aux`).

### Summary
`kerberos.update` accepts free-text `appdefaults_aux` / `libdefaults_aux`
blocks. Any line that does not parse as `key = value` — for example a bare
comment line, or a lone token — makes the call fail with a raw
`[EFAULT] list index out of range` and a Python traceback, instead of a
clean validation error identifying the bad line.

### Reproduction
1. `midclt call kerberos.config` to capture the current config for restore.
2. `midclt call kerberos.update '{"appdefaults_aux": "# a comment"}'`
   (any line lacking a `=`).
3. The call fails with `[EFAULT] list index out of range` (server splits
   the line on `=` and indexes `[1]` without a bounds check).
4. Restore the original `appdefaults_aux` value.

### Expected
A validation error naming the offending line, e.g. "appdefaults_aux line N
is not of the form key = value".

### Provider handling
The provider's `truenas_kerberos_config` Tier-2 acceptance test avoids
free-text marker strings and toggles a real recognized key
(`no_addresses`) instead, specifically to sidestep this crash. Documented
in `internal/resources/kerberos_config/acceptance_test.go`.

---

## 2. Stale `kerberos_realm` survives a directory-services service-type switch

**Severity:** functional — blocks a subsequent directory-services join.
**Observed on:** TrueNAS 25.10.3.1 and 26.0.0-BETA.2.
**Namespace/method:** `directoryservices.update` (interaction with
`kerberos_realm`).

### Summary
After a system has been joined to Active Directory and is then reconfigured
to a different `service_type` (e.g. LDAP or IPA), a stale `kerberos_realm`
value is left behind on the directory-services record. No normal
`directoryservices.update` payload clears it, and its presence causes a
spurious trailing `kinit()` failure on the next join even though the new
bind itself succeeds. Root cause traced to an asymmetry between the
`compress()` and `extend()` methods in the middleware's
`directoryservices_/datastore.py`: the realm is not nulled on the way out.

### Reproduction
1. Join the box to AD (`directoryservices.update` with
   `service_type=ACTIVEDIRECTORY`, `enable=true`), reach HEALTHY.
2. Reconfigure to LDAP or IPA (`service_type` changed, new credential).
3. Observe the update job fail late with an `[EFAULT]` referencing kerberos
   credentials, despite the LDAP/IPA bind succeeding, because the old
   `kerberos_realm` is still attached.

### Expected
Switching `service_type` clears any `kerberos_realm` that no longer applies,
in a single update.

### Provider handling
`truenas_directoryservices` carries a two-call `resetStaleServiceType`
workaround: it issues an intermediate update to clear the stale service-type
state before applying the new configuration. In
`internal/resources/directoryservices/{resource.go,model.go}`; a single
atomic server-side fix would let the workaround be removed.

---

## 3. `core.get_methods` under-reports namespaces on 25.10

**Severity:** correctness — introspection disagrees with what is callable.
**Observed on:** TrueNAS 25.10.4.
**Namespace/method:** `core.get_methods` (observed for `failover.*`).

### Summary
`core.get_methods` returned only 3 of the 13 `failover.*` methods that are
in fact directly callable on the same box (the other 10 succeed when called
by name). Any tooling that trusts the method listing to decide whether a
method exists will draw the wrong conclusion. The provider's rule, adopted
because of this, is to verify a method's existence by a direct call, never
by the listing alone.

### Reproduction
1. `midclt call core.get_methods` (or the equivalent introspection) and
   filter for `failover.`; count the entries.
2. Directly call listed-as-absent methods (e.g. `failover.status`,
   `failover.node`, `failover.disabled.reasons`) — they succeed.

### Expected
`core.get_methods` lists every callable method.

---

## 4. `failover.config.master` is unreliable immediately after a failover

**Severity:** correctness — a state field that misreports transiently.
**Observed on:** TrueNAS 25.10.4 (Enterprise HA pair).
**Namespace/method:** `failover.config` (`master` field).

### Summary
Following a real, controlled failover (triggered via
`failover.become_passive` on the active controller), the newly-active node
reported `failover.status == "MASTER"` and the pair fully stabilized, yet
`failover.config.master` remained `false` on that node. The `master` flag in
`failover.config` is therefore not a dependable live-state indicator right
after a failover event; `failover.status` and `failover.node` are.

### Reproduction
1. On an HA pair, note `failover.status` / `failover.node` /
   `failover.config.master` on the active controller.
2. Trigger `failover.become_passive` on the active controller.
3. Poll `failover.status` on the surviving/newly-active node until it
   reports `MASTER` and `failover.disabled.reasons` is empty.
4. Read `failover.config.master` on that node — observed still `false`.

### Expected
Once a node is the stable master, `failover.config.master` reflects it.

### Provider handling
`truenas_failover_config`'s datasource surfaces `status` / `node` /
`disabled_reasons` as the authoritative fields; the `master` attribute's
schema description documents that it is not reliable immediately
post-failover. In `internal/resources/failover_config/{schema.go,model.go}`.

---

## 5. `container.image.query_registry` lists pruned image versions

**Severity:** reliability — the listing and actual availability drift apart.
**Observed on:** TrueNAS 26.0.0-BETA.2.
**Namespace/method:** `container.image.query_registry` →
`container.create`.

### Summary
`container.image.query_registry` returns, per image, a list of versions
drawn from a registry cache. Some listed versions have already been pruned
from the upstream registry (`images.linuxcontainers.org`), so calling
`container.create` with a listed-but-pruned version fails with a
`[EFAULT] Failed to download image ... 404`. A version being present in the
query result is not a guarantee it can be pulled.

### Reproduction
1. `midclt call container.image.query_registry` and pick an older listed
   version of an image (e.g. an `alpine:3.22:amd64:default` build).
2. `container.create` with `image = {name, version}` for that version.
3. If the build was pruned upstream, the create job fails with a 404
   download error, even though the version appeared in the listing.

### Expected
Either the listing reflects only pullable versions, or the create error
distinguishes "pruned upstream" from other failures.

### Provider handling
The `truenas_container_image` datasource exposes `latest_version` (the
newest listed build) so configurations resolve a current version at plan
time rather than hardcoding one that may be pruned. The datasource
description documents the prune caveat. In
`internal/resources/container_image/`.

---

## 6. ACME registration reuse depends on a trailing slash in the directory URI

**Severity:** correctness — a repeatable operation fails on its second run
for a cosmetic URI difference; no clean recovery path.
**Observed on:** TrueNAS 26.0.0-BETA.2.
**Namespace/method:** `acme.registration.do_create` /
`acme.get_acme_client_and_key_payload` (via `certificate.create`
`CERTIFICATE_CREATE_ACME`).

### Summary
`acme.registration.do_create` normalizes the directory URI it stores by
appending a trailing slash, and dedups new registrations on that normalized
value. But the reuse-lookup in `acme_svc.get_acme_client_and_key_payload`
queries by the **raw** `acme_directory_uri` passed to `certificate.create`.
So for a directory URI given without a trailing slash:

- Run 1 stores `…/dir/`.
- Run 2's reuse-query for `…/dir` misses the stored `…/dir/`, so it tries to
  register again; `do_create`'s own dedup (on the normalized `…/dir/`) then
  raises `A registration with the specified directory uri already exists`,
  failing the second issuance.

Given `…/dir/` (trailing slash) both the reuse-query and the stored value
agree, the one ACME account is reused, and issuance is repeatable (verified:
two consecutive issuances, one registration row reused, no error).

Compounding it, `acme.registration` has **no public delete** (`do_delete`
does not exist), so a mismatched/stale registration can only be removed via
the datastore.

### Reproduction
1. `certificate.create` `CERTIFICATE_CREATE_ACME` with
   `acme_directory_uri` = `https://<pebble>:14000/dir` (no slash) → succeeds.
2. Delete the issued cert, then repeat step 1 → fails
   `A registration with the specified directory uri already exists`.
3. Repeat with `.../dir/` (trailing slash) throughout → both runs succeed.

### Expected
The reuse-lookup should match on the same normalized form `do_create`
stores, so issuance is idempotent regardless of a trailing slash; and/or a
public method to remove an ACME registration should exist.

### Provider handling
`acctest.ACMEDirectory` normalizes `TRUENAS_ACME_DIRECTORY` to a trailing
slash so `TestAccCertificate_acmeIssuance` reuses one account across re-runs.
(The registration row itself is not provider-managed and persists after the
test — reused, not accumulated; a Pebble restart, which wipes accounts,
would require clearing it out of band.)

---

## 7. Concurrent `filesystem.setperm` jobs silently no-op on 25.10

**Severity:** functional — a job reports SUCCESS but does not apply its
change, so permissions are silently wrong (data-exposure / lockout risk).
**Observed on:** TrueNAS 25.10.7.
**Namespace/method:** `filesystem.setperm` (job), run concurrently; the same
class very likely affects `filesystem.setacl`.

### Summary
When several `filesystem.setperm` jobs run at once, some of them complete with
state `SUCCESS` and no error yet leave the target path at its previous
mode/owner. The jobs are not failing — `core.get_jobs` shows SUCCESS — the
mutation is just dropped. Reproduced directly against 25.10.7: 12 concurrent
`filesystem.setperm` jobs (distinct paths, each requesting mode 0750 / uid 1001)
left **6 of 12** paths at 0755 / uid 0, with zero job errors. Serializing the
same 12 calls applied **12 of 12**. Matches a user report of
`truenas_filesystem_permissions` at Terraform's default parallelism leaving
some paths unchanged and failing with an inconsistent-result error.

### Reproduction
1. Create 12 datasets (or dirs), note each mountpoint.
2. Fire 12 `filesystem.setperm` jobs concurrently, each `{path, mode: "750",
   uid: 1001, gid: 1001, options: {recursive: false}}`.
3. Wait for all jobs to reach SUCCESS, then `filesystem.stat` each path.
4. Some paths retain the old mode/owner despite SUCCESS. `-parallelism=1`
   (or any serialization) applies all of them.

### Expected
A `filesystem.setperm` job that reports SUCCESS has applied its change;
concurrent setperm jobs on distinct paths do not interfere.

### Provider handling
The provider serializes `filesystem.setperm` and `filesystem.setacl` through a
single client-side lock (`Client.CallFilesystemMutation`), so these mutations
never run concurrently within a Terraform apply. Removes the dropped-mutation
race at the cost of serial permission application. In `internal/client/jobs.go`
and the `filesystem_permissions` / `filesystem_acl` resources.

---

## 8. `nvmet.subsys` accepts `ieee_oui` values the kernel misreads or ignores

**Severity:** correctness — a configured value silently becomes a different one.
**Observed on:** SCALE 28.0 (MASTER 2026-10-05); the same validation is in the 25.10 and 26.0 source.
**Namespace/method:** `nvmet.subsys.create` / `nvmet.subsys.update` (`ieee_oui`).

### Summary
`ieee_oui` is accepted as a free string. Only the `0x`-prefixed hex form reaches
the kernel's nvmet configfs correctly. An all-digit value such as `005056`
(VMware's OUI) is stored as an integer, returned by `nvmet.subsys.query` as a
JSON number, and written to the kernel as a different OUI (`0x0013c0`). A hex
value without the prefix such as `AABBCC` is silently ignored (the kernel shows
`0x000000`).

### Reproduction
1. `midclt call nvmet.subsys.create '{"name":"ouitest","ieee_oui":"005056"}'`
2. `midclt call nvmet.subsys.query '[["name","=","ouitest"]]'` — `ieee_oui` comes
   back as the number `5056`.
3. Read the subsystem's `attr_ieee_oui` in nvmet configfs — `0x0013c0`.
4. Repeat with `"AABBCC"` — configfs shows `0x000000`.
5. Delete the test subsystem.

### Expected
Validate `ieee_oui` as a 24-bit hex value, store it as a string, and write it to
configfs in the form the kernel parses.

### Provider handling
`truenas_nvmet_subsys.ieee_oui` requires `0x`-prefixed hex at plan time and
decodes a numeric value from the API. In `internal/resources/nvmet_subsys/`.

---

## 9. `interface.create` of a bridge with a member fails on 28.0

**Severity:** functional — a bridge with a member cannot be created.
**Observed on:** TrueNAS 28.0 (MASTER).
**Namespace/method:** `interface.create` (`type: BRIDGE`, `bridge_members`).

### Summary
Creating a BRIDGE interface that lists at least one member fails with
`[EINVAL] 'VMQueryResultItem' object is not subscriptable`. The error comes
from the `nic_attach_users` helper in the network plugin, which indexes a VM
query result as if it were a dict. A bridge without members is created
normally.

### Reproduction
1. `midclt call interface.create '{"type":"BRIDGE","name":"br9","bridge_members":["<spare NIC>"]}'`
   with a NIC that is not in use.
2. The call fails with `'VMQueryResultItem' object is not subscriptable`.
3. Repeat without `bridge_members`; it succeeds (delete the bridge afterwards).

### Expected
The bridge is created with its member.

### Provider handling
None; the provider sends a valid request. The bridge-member acceptance test
fails on 28.0 for this reason and passes on the older releases.

---

## 10. A bare IPv6 certificate SAN is stored as `DNS:db8::7` on 25.10

**Severity:** correctness — a requested SAN silently becomes a different one.
**Observed on:** TrueNAS 25.10.3.1. 26.0 and 28.0 store it correctly.
**Namespace/method:** `certificate.create` (CERTIFICATE_CREATE_CSR, `san`).

### Summary
A SAN entry given as a bare IPv6 address with no `IP:` prefix, for example
`2001:db8::7`, is split on its first colon: `2001` is read as the entry type
and the rest is stored as a DNS name, giving `DNS:db8::7`. The resulting CSR
carries the wrong SAN. The prefixed form `IP:2001:db8::7` is stored correctly.

### Reproduction
1. `midclt call certificate.create` a CSR with `"san": ["2001:db8::7"]`.
2. Read the certificate back; `san` is `["DNS:db8::7"]`.
3. Repeat with `"IP:2001:db8::7"`; the SAN is stored as an IP address.

### Expected
Either detect a bare IPv6 address and store it as an IP SAN, or reject the
entry. Do not store a different name.

### Provider handling
`truenas_certificate.san` sends a bare IP address as `IP:<address>` on every
release. In `internal/resources/certificate/`.

---

## 11. `service.start` / `service.stop` require an `options` argument on 26.0

**Severity:** functional — the documented short call form raises a TypeError.
**Observed on:** TrueNAS 26.0.0-BETA.3. 25.10 and 28.0 accept the service name alone.
**Namespace/method:** `service.start`, `service.stop`.

### Summary
On 26.0, calling `service.start` or `service.stop` with only the service name
fails with `TypeError: ServiceService.start() missing 1 required positional
argument: 'options'`. 25.10 and 28.0 accept the name alone. On 26.0
`core.get_methods` does not list `service.start` or `service.stop` at all; it
lists `service.control`, `service.get_instance`, `service.query`,
`service.started`, `service.started_or_enabled` and `service.update`. A client
cannot discover from the API metadata that the call shape differs on this
release.

### Reproduction
1. `midclt call service.start ftp` on 26.0; it raises the TypeError above.
2. `midclt call service.start <name> '{}'` (with an empty options object) gets past the
   signature check (for a nonexistent name it fails with `MatchNotFound` instead).
3. `midclt call core.get_methods` and look for `service.start`; it is absent.
4. Repeat steps 1 and 3 on 25.10 and 28.0; the short call works.

### Expected
The same call shape on every release, and every callable method listed by
`core.get_methods`.

### Provider handling
`truenas_service` uses `service.control` on every release and reports a failed
start or stop as an error. In `internal/resources/service/`.

---

## 12. 25.10 keeps API-key sessions that `auth.terminate_session` cannot remove

**Severity:** reliability — sessions pile up and the box rate-limits logins.
**Observed on:** TrueNAS 25.10.
**Namespace/method:** `auth.sessions`, `auth.terminate_session`,
`auth.login_with_api_key`.

### Summary
A box that had served a few hundred short-lived API-key connections held 642
authentication sessions, 635 of them from one test API key, while only about
25 TCP connections were open on the web port. `auth.terminate_session` returns
`True` for those sessions, but they stay in `auth.sessions`. Only a middleware
restart clears them. In the same period `auth.login_with_api_key` answered
`false` for a valid key several times, which looks tied to the session load.

### Reproduction
1. Open and close many websocket connections that log in with one API key
   (for example a test run that does not close its connections).
2. Compare `midclt call auth.sessions` (hundreds of entries) with the number
   of established connections on the web port (a few dozen).
3. `midclt call auth.terminate_session <id>` returns `True`; the entry is still
   listed.

### Expected
Sessions disappear when their connection closes, and `auth.terminate_session`
removes the session it reports terminating.

### Provider handling
The provider stops creating sessions at that rate (one client per process, no
keepalive logins on abandoned clients) and retries an API-key login that is
rejected with `false` twice before reporting an invalid key. In
`internal/client/`.

---

## 13. `filesystem.acltemplate.create` fails with `KeyError: 'who'` for `who` entries

**Severity:** functional — an unhandled server-side exception.
**Observed on:** TrueNAS 26.0 and 28.0 (same code path in the source).
**Namespace/method:** `filesystem.acltemplate.create`.

### Summary
An ACL template entry that names its principal with `who` (for example
`"who": "root"` on a USER entry) fails with `KeyError: 'who'`. The plugin pops
`who` while resolving the entry and then reads `ace['who']` again further down.
The same entry given as `id` works.

### Reproduction
1. `midclt call filesystem.acltemplate.create` with an ACL whose USER entry is
   `{"tag":"USER","who":"root",...}`.
2. The call fails with `KeyError: 'who'`.

### Expected
The entry is accepted, or rejected with a validation error.

### Provider handling
`truenas_acl_template` resolves `who` to the numeric id before sending. In
`internal/resources/acl_template/`.

---

## 14. `auth.mechanism_choices` raises an exception when called before login on 25.10

**Severity:** minor — an unhandled exception from a method that is callable
before authentication on 26.0.
**Observed on:** TrueNAS 25.10.3.1.
**Namespace/method:** `auth.mechanism_choices`.

### Summary
Called on an unauthenticated connection, the method raises
`'NoneType' object has no attribute 'may_create_auth_token'` instead of
returning the available login mechanisms. On 26.0 and later it answers.

### Reproduction
1. Open a websocket connection and, before logging in, call
   `auth.mechanism_choices`.
2. The call fails with the AttributeError text above.

### Expected
The list of mechanisms, as on 26.0.

### Provider handling
The provider treats the failure as "this server has no SCRAM login" and uses
the API-key login. In `internal/client/scram.go`.

---

## 15. 25.10 ignores the `refreservation` property when creating a zvol

**Severity:** correctness — a configured value is replaced by another one.
**Observed on:** TrueNAS 25.10.
**Namespace/method:** `pool.dataset.create` (`type: VOLUME`, `refreservation`).

### Summary
Creating a non-sparse zvol with an explicit `refreservation` (including `0`)
stores the value ZFS computes for the volume instead: a requested `0` reads
back as 70254592, and a requested 536870912 reads back as 1092616192. The
property sent in the create call is overridden when the volume is created.

### Reproduction
1. `midclt call pool.dataset.create '{"name":"<pool>/rtest","type":"VOLUME","volsize":1073741824,"refreservation":0}'`
2. `midclt call pool.dataset.query '[["id","=","<pool>/rtest"]]'` shows a
   non-zero `refreservation`.
3. A following `pool.dataset.update` with `refreservation: 0` sets it.
4. Delete the test zvol.

### Expected
The requested `refreservation` is applied at create.

### Provider handling
`truenas_zvol` (and `truenas_dataset` with `type = "VOLUME"`) issues an update
after create when the stored `refreservation` differs from the configured one.
In `internal/resources/zvol/` and `internal/resources/dataset/`.

---

## Filing notes

Findings 1, 2 and 7 have functional impact (a server crash, a broken
re-join, and silently dropped permission changes) and are the strongest
upstream candidates. 3–6 and 8–15 are correctness/reliability gaps (9, 11 and 13 are also hard failures). Reproduction transcripts captured during
development live under `.superpowers/sdd/` (git-ignored) — attach the
relevant task report when filing. This document is summarized in
`TEST-PLAN.md` §10.

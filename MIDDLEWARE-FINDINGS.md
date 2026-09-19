# TrueNAS Middleware Findings

Server-side issues found while building and testing
terraform-provider-truenas. None are provider bugs — each is a behavior in
the TrueNAS middleware itself, recorded here so they can be filed
upstream with iX. Where the provider carries a workaround, it is noted.

Releases referenced: TrueNAS 25.10.3.1, 25.10.4 (Enterprise HA), and
26.0.0-BETA.2. Each finding lists the release(s) where it was observed.

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

## Filing notes

Findings 1 and 2 have functional impact (a server crash and a broken
re-join) and are the strongest upstream candidates. 3–6 are
correctness/reliability gaps. Reproduction transcripts captured during
development live under `.superpowers/sdd/` (git-ignored) — attach the
relevant task report when filing. This document is summarized in
`TEST-PLAN.md` §10.

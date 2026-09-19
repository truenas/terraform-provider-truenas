// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app_registry_test

import (
	"testing"
)

// TestAccAppRegistry_basic is intentionally skipped unconditionally.
//
// DECISIVE PROBE RESULT (TrueNAS 26.0, live, 2026-07-23):
// app.registry.create validates the supplied username/password/uri against
// the real container registry endpoint before persisting anything. Probed
// via a throwaway create with fabricated credentials and an unreachable
// RFC 5737 TEST-NET-1 uri:
//
//	app.registry.create({
//	  "name": "tf-probe-registry-decisive",
//	  "description": "tf probe decisive - safe to delete",
//	  "uri": "https://192.0.2.123:5000",
//	  "username": "tfprobeuser", "password": "tfprobepassword123",
//	})
//	=> truenas API error (code 22): [EINVAL] app_registry_create.uri:
//	   Invalid credentials for registry
//
// Nothing was persisted (the error is returned synchronously, "job": false
// per core.get_methods, before any object is created), so there was
// nothing to clean up.
//
// Per the task brief's decision rule ("if create DOES validate ->
// unconditionally-skipped acceptance file with in-file rationale"), this
// test is a documented, permanent skip rather than a flaky/parameterized
// one (mirrors internal/resources/cloud_backup's TestAccCloudBackup_basic,
// itself skipped because cloud_backup.create validates credentials/bucket
// against a real remote endpoint with no fixture available here).
//
// The identical probe against TrueNAS 25.10.3.1 could not reach this
// validation path: that box has no Docker pool configured, so
// app.registry.create failed earlier with a distinct apps-unconfigured
// error (`truenas API error (code 14): [EFAULT] No pool configured for
// Docker`). See schema.go's doc comment for the full writeup of both
// probes, including confirmation (via core.get_methods on both releases)
// that the app.registry.{create,query,get_instance,update,delete} field
// shape and job:false-ness are identical across 25.10 and 26.0 — only the
// live validation behavior differs by whether apps/Docker happen to be
// configured on the box, not by TrueNAS release.
//
// If this is ever enabled against an environment with a real, reachable
// container registry:
//  1. Point "uri" at a real registry (e.g. a self-hosted registry:2
//     instance, or "https://index.docker.io/v1/" with genuine Docker Hub
//     credentials) that the tester controls and is reachable from the
//     TrueNAS box under test.
//  2. Use genuine username/password (or access token) credentials for that
//     registry — dummy/fabricated credentials are rejected by design.
//  3. Add ImportStateVerifyIgnore for "password": it is Required +
//     WriteOnly (never stored in Terraform state — see schema.go/model.go),
//     so ImportStateVerify would otherwise fail comparing an empty
//     imported value against the write-only config value.
//  4. Scope CheckDestroy to the test's own RandName-generated "name" (via
//     app.registry.query filtered on that name), matching every other Tier
//     1 acceptance test in this provider.
func TestAccAppRegistry_basic(t *testing.T) {
	t.Skip("app.registry.create validates username/password/uri against the real container registry endpoint " +
		"(live-probe-confirmed on TrueNAS 26.0: a throwaway create with dummy credentials and an unreachable " +
		"TEST-NET-1 uri was rejected with \"Invalid credentials for registry\"); no live, reachable container " +
		"registry fixture is available in this environment, so this test is permanently skipped. See the doc " +
		"comment on TestAccAppRegistry_basic for the decisive probe evidence and how to enable this against an " +
		"environment with a real container registry.")
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vmware_test

import (
	"testing"
)

// TestAccVMware_basic is intentionally skipped unconditionally.
//
// DECISIVE PROBE RESULT (BOTH TrueNAS 25.10.4 HA and 26.0, live,
// 2026-07-23): vmware.create validates the supplied hostname/username/
// password against the real vCenter/ESXi endpoint before persisting
// anything. Probed via a throwaway create with fabricated credentials and
// an unreachable RFC 5737 TEST-NET-1 hostname:
//
//	vmware.create({
//	  "datastore": "tf-probe-datastore", "filesystem": "tank",
//	  "hostname": "192.0.2.123",
//	  "username": "tfprobeuser", "password": "tf-probe-fake-password-1234",
//	})
//	=> TrueNAS 25.10.4 HA (wss://10.220.16.188): truenas API error (code 22):
//	   [EINVAL] vmware_create.datastore: Failed to connect: [ENETUNREACH]
//	   [Errno 101] Network is unreachable
//	=> TrueNAS 26.0 (wss://192.168.1.68): truenas API error (code 22):
//	   [EINVAL] vmware_create.datastore: Failed to connect: [ETIMEDOUT]
//	   [Errno 110] Connection timed out
//
// Nothing was persisted on either box (the error is returned synchronously,
// "job": false per core.get_methods, before any record is created;
// vmware.query returned `[]` on both boxes both before and after each
// probe), so there was nothing to clean up.
//
// Per the task brief's decision rule ("if create DOES validate ->
// documented-skip acceptance + full units"), this test is a documented,
// permanent skip rather than a flaky/parameterized one — mirrors
// internal/resources/app_registry's TestAccAppRegistry_basic (identical
// situation: create validates a remote endpoint with fabricated
// credentials, rejected before persisting) and internal/resources/
// cloud_backup's TestAccCloudBackup_basic.
//
// If this is ever enabled against an environment with a real, reachable
// vCenter/ESXi host:
//  1. Point "hostname" at a real vCenter server or ESXi host the tester
//     controls and is reachable from the TrueNAS box under test, with a
//     real "datastore" name that exists on it.
//  2. Use genuine "username"/"password" credentials for that host — dummy/
//     fabricated credentials are rejected by design (see the decisive
//     probe above).
//  3. Point "filesystem" at a disposable dataset path fixture (e.g. via a
//     throwaway truenas_dataset resource).
//  4. Add ImportStateVerifyIgnore for "password": it is Required +
//     WriteOnly (never stored in Terraform state — see schema.go/model.go),
//     so ImportStateVerify would otherwise fail comparing an empty imported
//     value against the write-only config value.
//  5. Scope CheckDestroy to the test's own resource id (via vmware.query
//     filtered on that id, or simply checking get_instance returns
//     not-found), matching every other Tier 1 acceptance test in this
//     provider.
func TestAccVMware_basic(t *testing.T) {
	t.Skip("vmware.create validates hostname/username/password against the real vCenter/ESXi endpoint " +
		"(live-probe-confirmed on BOTH TrueNAS 25.10.4 HA and 26.0: a throwaway create with dummy " +
		"credentials and an unreachable RFC 5737 TEST-NET-1 hostname was rejected with a connection error on " +
		"each release); no live, reachable vCenter/ESXi fixture is available in this environment, so this test " +
		"is permanently skipped. See the doc comment on TestAccVMware_basic for the decisive probe evidence and " +
		"how to enable this against an environment with a real VMware host.")
}

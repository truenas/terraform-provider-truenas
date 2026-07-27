// Copyright (c) iXsystems, Inc.
// SPDX-License-Identifier: MPL-2.0

package cloud_backup_test

import (
	"testing"
)

// TestAccCloudBackup_basic is intentionally skipped unconditionally.
//
// DECISIVE PROBE RESULT (TrueNAS 25.10.3.1, live, 2026-07-22):
// unlike truenas_cloudsync_credentials/truenas_cloudsync_task (whose create
// calls never touch the remote endpoint), cloud_backup.create DOES validate
// the credential against the real remote bucket at apply time. Probed via a
// throwaway S3-type cloudsync credential (fabricated access key) plus a
// nonexistent bucket name:
//
//	cloud_backup.create({
//	  "path": "/mnt/tank", "credentials": <throwaway S3 creds id>,
//	  "attributes": {"bucket": "tf-probe-nonexistent-bucket-xyz123", "folder": "tf-probe-folder"},
//	  "password": "tf-probe-password-1234", "keep_last": 1, "enabled": false,
//	})
//	=> truenas API error (code 22): An error occurred (InvalidAccessKeyId)
//	   when calling the GetBucketLocation operation: The AWS Access Key Id
//	   you provided does not exist in our records.
//
// Source-confirmed (middlewared/plugins/cloud_backup/crud.py, _validate):
// do_create/do_update call `cloud_backup.ensure_initialized(data)`, which
// opens the actual restic repository against the remote endpoint before the
// task is ever persisted — so ANY working Tier 1 create/update/import cycle
// for this resource needs credentials that authenticate against a REAL,
// reachable cloud storage bucket. No such fixture is available in this
// environment (dummy credentials are rejected by design, and this repo has
// no live S3-compatible bucket to point at), so a green apply cannot be
// produced here. Per the task brief's decision rule ("if create DOES
// validate -> unconditionally-skipped acceptance file with in-file
// rationale"), this test is a documented, permanent skip rather than a
// flaky/parameterized one (mirrors truenas_network_config's
// TestAccNetworkConfig_basic, skipped because driving it live risks cutting
// off the box's own network connectivity; here the risk is simply "no such
// fixture exists to run against").
//
// If this is ever enabled against an environment with real cloud storage
// credentials:
//  1. Create a truenas_cloudsync_credentials fixture with a real,
//     working provider (e.g. S3 with genuine access_key_id/secret_access_key
//     pointed at a bucket the tester controls).
//  2. Point truenas_cloud_backup.path at a disposable dataset path fixture
//     (e.g. via a throwaway truenas_dataset resource), set enabled=false,
//     and keep keep_last small (e.g. 1) to bound blast radius.
//  3. NEVER call cloud_backup.sync (nor exercise any Terraform behavior
//     that would trigger it) — that performs a real backup upload/restic
//     init against the live bucket, which is out of scope for an
//     apply/destroy CRUD acceptance test.
//  4. Use ImportStateVerifyIgnore for "password": cloud_backup.get_instance
//     returns it unmasked only to a FULL_ADMIN/CLOUD_BACKUP_WRITE-scoped
//     credential (source-verified against middlewared's
//     dump_result()/remove_secrets() logic) — safer to exclude from the
//     import-verify diff than to assume that role scope holds in every
//     test environment.
func TestAccCloudBackup_basic(t *testing.T) {
	t.Skip("cloud_backup.create validates the credential/bucket against the real remote endpoint (source- and live-probe-confirmed via cloud_backup.ensure_initialized); no live S3-compatible bucket fixture is available in this environment, so this test is permanently skipped. See the doc comment on TestAccCloudBackup_basic for the decisive probe evidence and how to enable this against an environment with real cloud storage credentials.")
}

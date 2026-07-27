// Copyright (c) iXsystems, Inc.
// SPDX-License-Identifier: MPL-2.0

package system_dataset_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccSystemDatasetDataSource_basic reads the current TrueNAS system
// dataset configuration through the truenas_system_dataset datasource only.
// It never writes: migrating the system dataset between pools is a
// disruptive, long-running operation, and the box's real configuration must
// not be touched by an acceptance test run.
//
// Requires:
//   - TF_ACC=1
//   - TRUENAS_API_KEY (or TRUENAS_USERNAME+TRUENAS_PASSWORD) set for
//     acctest.PreCheck
//   - A running TrueNAS instance
func TestAccSystemDatasetDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_system_dataset" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_system_dataset.test", "id", "system_dataset"),
					resource.TestCheckResourceAttrSet("data.truenas_system_dataset.test", "pool"),
					resource.TestCheckResourceAttrSet("data.truenas_system_dataset.test", "basename"),
					resource.TestCheckResourceAttrSet("data.truenas_system_dataset.test", "path"),
					resource.TestCheckResourceAttrSet("data.truenas_system_dataset.test", "uuid"),
				),
			},
		},
	})
}

// TestAccSystemDataset_basic is intentionally skipped by default.
// truenas_system_dataset is a SINGLETON resource whose "pool" attribute, when
// changed, triggers a long-running migration of core system state (logs,
// reporting, syslog, samba4 data) to a different pool on the live target
// system. A naive create/update/destroy acceptance test would move the
// system dataset off its current pool, which is disruptive and can be slow
// or risky to reverse against a shared/production TrueNAS instance.
//
// If this test is ever enabled against a disposable/non-production TrueNAS
// instance with at least two pools available, it should:
//  1. Read the current config via the datasource first, noting the current
//     pool.
//  2. Drive "pool" through a value and back to the original pool, so the net
//     effect on the box is a no-op (accepting that this still triggers two
//     real migrations).
//  3. Use ImportState with ImportStateId "system_dataset" to verify import
//     normalizes any ID to the fixed singleton ID.
func TestAccSystemDataset_basic(t *testing.T) {
	t.Skip("truenas_system_dataset migrates core system state between pools; skipped to avoid a disruptive, long-running migration on the target box. See comment on TestAccSystemDataset_basic for how to safely enable this against a disposable instance.")
}

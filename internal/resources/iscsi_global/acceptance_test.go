package iscsi_global_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccISCSIGlobalDataSource_basic reads the current TrueNAS iSCSI global
// configuration through the truenas_iscsi_global datasource only. It never
// writes: the iSCSI global configuration serves live storage, and the box's
// real configuration must not be overwritten by an acceptance test run.
//
// Requires:
//   - TF_ACC=1
//   - TRUENAS_API_KEY (or TRUENAS_USERNAME+TRUENAS_PASSWORD) set for
//     acctest.PreCheck
//   - A running TrueNAS SCALE instance
func TestAccISCSIGlobalDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_iscsi_global" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_iscsi_global.test", "id", "iscsi_global"),
					resource.TestCheckResourceAttrSet("data.truenas_iscsi_global.test", "basename"),
					resource.TestCheckResourceAttrSet("data.truenas_iscsi_global.test", "listen_port"),
				),
			},
		},
	})
}

// TestAccISCSIGlobal_basic is intentionally skipped by default.
// truenas_iscsi_global is a SINGLETON resource that serves live storage:
// bringing it under Terraform management mutates the box's actual iSCSI
// global configuration (iscsi.global.update), and a naive
// create/update/destroy acceptance test risks disrupting active iSCSI
// sessions or discovery on whatever system runs it.
//
// If this test is ever enabled against a disposable/non-production TrueNAS
// instance, it should:
//  1. Read the current config via the datasource first.
//  2. Only touch "alua" (a low-risk boolean toggle) in the resource config,
//     driving it through a value and back to the value the datasource
//     observed in step 1, so the net effect on the box is a no-op. Never
//     touch basename, listen_port, isns_servers, or pool_avail_threshold in
//     an automated test: changing those on a live system can disrupt active
//     iSCSI sessions or discovery.
//  3. Use ImportState with ImportStateId "iscsi_global" to verify import
//     normalizes any ID to the fixed singleton ID.
func TestAccISCSIGlobal_basic(t *testing.T) {
	t.Skip("truenas_iscsi_global serves live storage; skipped to avoid mutating the target box's iSCSI configuration. See comment on TestAccISCSIGlobal_basic for how to safely enable this against a disposable instance.")
}

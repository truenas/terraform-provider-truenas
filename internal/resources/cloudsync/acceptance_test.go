// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccCloudSync_basic tests create, update, and import of a cloud sync
// task. It requires:
//   - TrueNAS_API_KEY set in the environment (checked by acctest.PreCheck)
//   - A running TrueNAS instance
//   - TF_ACC_CLOUDSYNC_CREDENTIALS_ID set to the ID of pre-existing cloud
//     sync credentials to use
//   - TF_ACC_CLOUDSYNC_PATH set to a local path to sync (e.g. /mnt/tank/data)
func TestAccCloudSync_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	credsID := os.Getenv("TF_ACC_CLOUDSYNC_CREDENTIALS_ID")
	if credsID == "" {
		t.Skip("TF_ACC_CLOUDSYNC_CREDENTIALS_ID not set: skipping truenas_cloudsync_task acceptance test")
	}
	path := os.Getenv("TF_ACC_CLOUDSYNC_PATH")
	if path == "" {
		t.Skip("TF_ACC_CLOUDSYNC_PATH not set: skipping truenas_cloudsync_task acceptance test")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccCloudSyncConfig(credsID, path, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_cloudsync_task.test", "id"),
					resource.TestCheckResourceAttr("truenas_cloudsync_task.test", "description", "tf-acc-cloudsync"),
					resource.TestCheckResourceAttr("truenas_cloudsync_task.test", "enabled", "true"),
					resource.TestCheckResourceAttr("truenas_cloudsync_task.test", "direction", "PUSH"),
					resource.TestCheckResourceAttr("truenas_cloudsync_task.test", "transfer_mode", "SYNC"),
					resource.TestCheckResourceAttr("truenas_cloudsync_task.test", "schedule.minute", "0"),
					resource.TestCheckResourceAttr("truenas_cloudsync_task.test", "schedule.hour", "0"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccCloudSyncConfig(credsID, path, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_cloudsync_task.test", "enabled", "false"),
				),
			},
			{
				ResourceName:      "truenas_cloudsync_task.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCloudSyncConfig(credsID, path string, enabled bool) string {
	return fmt.Sprintf(`
resource "truenas_cloudsync_task" "test" {
  description   = "tf-acc-cloudsync"
  path          = %q
  credentials   = %s
  direction     = "PUSH"
  transfer_mode = "SYNC"
  attributes    = "{\"bucket\": \"tf-acc-bucket\", \"folder\": \"backups\"}"
  enabled       = %v
  schedule = {
    minute = "0"
    hour   = "0"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}
`, path, credsID, enabled)
}

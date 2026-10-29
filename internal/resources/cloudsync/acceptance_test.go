// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
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

// TestAccCloudSync_encryptionPasswordRotation: encryption_password/salt are
// write-only, so a changed value was never sent (B-026). Bumping
// encryption_wo_version sends them.
func TestAccCloudSync_encryptionPasswordRotation(t *testing.T) {
	name := acctest.RandName("tffix3shtasks-cs")
	check := func(wantPW, wantSalt string) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			id, _ := strconv.ParseInt(s.RootModule().Resources["truenas_cloudsync_task.test"].Primary.ID, 10, 64)
			raw, err := acctest.Client().Call(context.Background(), "cloudsync.get_instance", id)
			if err != nil {
				return err
			}
			var got struct {
				Password string `json:"encryption_password"`
				Salt     string `json:"encryption_salt"`
			}
			if err := json.Unmarshal(raw, &got); err != nil {
				return err
			}
			if got.Password != wantPW || got.Salt != wantSalt {
				return fmt.Errorf("server encryption_password/salt = %q/%q, want %q/%q", got.Password, got.Salt, wantPW, wantSalt)
			}
			return nil
		}
	}
	body := func(pw, salt string, version int) string {
		return fmt.Sprintf(`
  encryption            = true
  encryption_password   = %q
  encryption_salt       = %q
  encryption_wo_version = %d
`, pw, salt, version)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		Steps: []resource.TestStep{
			{Config: acctest.ProviderConfig() + cloudSyncS3Config(name, body("tffix3pw1", "tffix3salt1", 1)), Check: check("tffix3pw1", "tffix3salt1")},
			{Config: acctest.ProviderConfig() + cloudSyncS3Config(name, body("tffix3pw2", "tffix3salt2", 2)), Check: check("tffix3pw2", "tffix3salt2")},
		},
	})
}

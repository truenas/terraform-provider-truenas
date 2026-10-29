// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloud_backup_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func checkCloudBackupPassword(want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, _ := strconv.ParseInt(s.RootModule().Resources["truenas_cloud_backup.test"].Primary.ID, 10, 64)
		raw, err := acctest.Client().CallRead(context.Background(), "cloud_backup.get_instance", id)
		if err != nil {
			return err
		}
		var got struct {
			Password string `json:"password"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			return err
		}
		if got.Password != want {
			return fmt.Errorf("stored password = %q, want %q", got.Password, want)
		}
		return nil
	}
}

// TestAccCloudBackup_passwordSentOnlyOnVersionChange: an unrelated update
// (keep_last) with a changed password_wo and the same password_wo_version
// leaves the stored restic password alone (before, it was sent, and TrueNAS
// rejected it because the repository was initialized with the old one);
// bumping the version sends it (with a new folder, so a fresh repository is
// initialized with the new password). (B-141)
func TestAccCloudBackup_passwordSentOnlyOnVersionChange(t *testing.T) {
	endpoint, access, secret, bucket := cloudBackupS3Env(t)
	acctest.PreCheck(t)
	name := acctest.RandName("tf-acc-cbwo")
	cfg := func(folder, pw string, version, keep int) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "cb" {
  name = "%[1]s/%[2]s"
}

resource "truenas_cloudsync_credentials" "cb" {
  name = %[2]q
  provider_config = jsonencode({
    type              = "S3"
    access_key_id     = %[3]q
    secret_access_key = %[4]q
    endpoint          = %[5]q
    skip_region       = true
    signatures_v2     = false
  })
}

resource "truenas_cloud_backup" "test" {
  description         = %[2]q
  path                = truenas_dataset.cb.mountpoint
  credentials         = truenas_cloudsync_credentials.cb.id
  attributes          = jsonencode({ bucket = %[6]q, folder = %[7]q })
  password_wo         = %[8]q
  password_wo_version = %[9]d
  keep_last           = %[10]d
  enabled             = false
}
`, acctest.TestPool(), name, access, secret, endpoint, bucket, name+folder, pw, version, keep)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		Steps: []resource.TestStep{
			{Config: cfg("-a", "tfwo-restic-a", 1, 1), Check: checkCloudBackupPassword("tfwo-restic-a")},
			{
				Config: cfg("-a", "tfwo-restic-b", 1, 2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_cloud_backup.test", "keep_last", "2"),
					checkCloudBackupPassword("tfwo-restic-a"),
				),
			},
			{Config: cfg("-b", "tfwo-restic-b", 2, 2), Check: checkCloudBackupPassword("tfwo-restic-b")},
		},
	})
}

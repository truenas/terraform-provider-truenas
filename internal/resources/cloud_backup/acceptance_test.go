// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloud_backup_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccCloudBackup_basic exercises full create/import/destroy of a
// truenas_cloud_backup. Unlike most resources, cloud_backup.create validates
// the credential against the real remote bucket at apply time (it opens the
// restic repository via cloud_backup.ensure_initialized), so it needs a
// reachable, authenticating S3 endpoint. It is gated behind the TF_ACC_S3_*
// env vars, which point at an S3-compatible bucket the tester controls (e.g. a
// local MinIO/SeaweedFS fixture):
//
//	TF_ACC_S3_ENDPOINT   e.g. http://192.168.1.154:9000
//	TF_ACC_S3_ACCESS     access key id
//	TF_ACC_S3_SECRET     secret access key
//	TF_ACC_S3_BUCKET     an existing bucket
//
// It sets enabled=false and keep_last=1 to bound blast radius, and never
// triggers cloud_backup.sync (no real data upload). password is excluded from
// import-verify because get_instance only returns it unmasked to a
// sufficiently-scoped credential.
func TestAccCloudBackup_basic(t *testing.T) {
	endpoint := os.Getenv("TF_ACC_S3_ENDPOINT")
	access := os.Getenv("TF_ACC_S3_ACCESS")
	secret := os.Getenv("TF_ACC_S3_SECRET")
	bucket := os.Getenv("TF_ACC_S3_BUCKET")
	if endpoint == "" || access == "" || secret == "" || bucket == "" {
		t.Skip("set TF_ACC_S3_ENDPOINT/ACCESS/SECRET/BUCKET (an S3-compatible bucket) to run the cloud_backup test")
	}
	acctest.PreCheck(t)

	dsName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-cb"))
	folder := acctest.RandName("tf-cb")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCloudBackupDestroyed("/mnt/" + dsName),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccCloudBackupConfig(dsName, endpoint, access, secret, bucket, folder),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_cloud_backup.test", "id"),
					resource.TestCheckResourceAttr("truenas_cloud_backup.test", "enabled", "false"),
					resource.TestCheckResourceAttr("truenas_cloud_backup.test", "keep_last", "1"),
					resource.TestCheckResourceAttrPair(
						"truenas_cloud_backup.test", "credentials",
						"truenas_cloudsync_credentials.cb", "id"),
				),
			},
			{
				ResourceName:      "truenas_cloud_backup.test",
				ImportState:       true,
				ImportStateVerify: true,
				// password: get_instance only returns it unmasked to a
				// sufficiently-scoped credential. attributes: the API expands
				// the JSON blob with server defaults (encryption, region,
				// fast_list, storage_class), so an import can't reproduce the
				// config's minimal form — same normalization the apply path
				// handles via write-what-you-said.
				ImportStateVerifyIgnore: []string{"password", "attributes"},
			},
		},
	})
}

func testAccCloudBackupConfig(dsName, endpoint, access, secret, bucket, folder string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "cb" {
  name = %[1]q
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
  path        = truenas_dataset.cb.mountpoint
  credentials = truenas_cloudsync_credentials.cb.id
  attributes  = jsonencode({ bucket = %[6]q, folder = %[7]q })
  password    = "tf-acc-cloudbackup-encpw"
  keep_last   = 1
  enabled     = false
  schedule = {
    minute = "0"
    hour   = "0"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}
`, dsName, acctest.RandName("tf-cb-cred"), access, secret, endpoint, bucket, folder)
}

// testAccCheckCloudBackupDestroyed verifies no cloud_backup task remains for
// the fixture dataset path.
func testAccCheckCloudBackupDestroyed(path string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		raw, err := acctest.Client().Call(context.Background(), "cloud_backup.query", [][]any{{"path", "=", path}})
		if err != nil {
			return fmt.Errorf("querying cloud_backup for %s: %w", path, err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("parsing cloud_backup.query: %w", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("cloud_backup for %s still exists", path)
		}
		return nil
	}
}

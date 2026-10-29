// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloud_backup_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// cloudBackupS3Env returns the TF_ACC_S3_* bucket settings, or skips (see
// TestAccCloudBackup_basic).
func cloudBackupS3Env(t *testing.T) (endpoint, access, secret, bucket string) {
	t.Helper()
	endpoint = os.Getenv("TF_ACC_S3_ENDPOINT")
	access = os.Getenv("TF_ACC_S3_ACCESS")
	secret = os.Getenv("TF_ACC_S3_SECRET")
	bucket = os.Getenv("TF_ACC_S3_BUCKET")
	if endpoint == "" || access == "" || secret == "" || bucket == "" {
		t.Skip("set TF_ACC_S3_ENDPOINT/ACCESS/SECRET/BUCKET (an S3-compatible bucket) to run the cloud_backup test")
	}
	return endpoint, access, secret, bucket
}

// cloudBackupMinimalConfig is a cloud backup task with schedule and every
// other Optional attribute left unset; extra is appended after the resources.
func cloudBackupMinimalConfig(name, endpoint, access, secret, bucket, extra string) string {
	return fmt.Sprintf(`
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
  description = %[2]q
  path        = truenas_dataset.cb.mountpoint
  credentials = truenas_cloudsync_credentials.cb.id
  attributes  = jsonencode({ bucket = %[6]q, folder = %[2]q })
  password    = "tf-acc-cloudbackup-encpw"
  keep_last   = 1
  enabled     = false
}
%[7]s`, acctest.TestPool(), name, access, secret, endpoint, bucket, extra)
}

// TestAccCloudBackup_scheduleOmitted: schedule is documented Optional, but
// omitting it failed every create with a Value Conversion Error before any
// API call (a plain-struct model field cannot hold the unknown plan value)
// (B-016). The server default schedule is read back. The same config also
// reads the task through the truenas_cloud_backup data source, which failed
// every read the same way (B-017).
func TestAccCloudBackup_scheduleOmitted(t *testing.T) {
	endpoint, access, secret, bucket := cloudBackupS3Env(t)
	acctest.PreCheck(t)
	name := acctest.RandName("tf-acc-cb")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCloudBackupDestroyed("/mnt/" + acctest.TestPool() + "/" + name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + cloudBackupMinimalConfig(name, endpoint, access, secret, bucket, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_cloud_backup.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_cloud_backup.test", "schedule.minute"),
					resource.TestCheckResourceAttrSet("truenas_cloud_backup.test", "schedule.hour"),
				),
			},
			{
				Config: acctest.ProviderConfig() + cloudBackupMinimalConfig(name, endpoint, access, secret, bucket, `
data "truenas_cloud_backup" "d" {
  description = truenas_cloud_backup.test.description
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.truenas_cloud_backup.d", "id", "truenas_cloud_backup.test", "id"),
					resource.TestCheckResourceAttrPair("data.truenas_cloud_backup.d", "path", "truenas_cloud_backup.test", "path"),
					resource.TestCheckResourceAttrPair("data.truenas_cloud_backup.d", "schedule.minute", "truenas_cloud_backup.test", "schedule.minute"),
					resource.TestCheckResourceAttrPair("data.truenas_cloud_backup.d", "schedule.hour", "truenas_cloud_backup.test", "schedule.hour"),
					resource.TestCheckResourceAttr("data.truenas_cloud_backup.d", "keep_last", "1"),
					resource.TestCheckResourceAttr("data.truenas_cloud_backup.d", "enabled", "false"),
				),
			},
		},
	})
}

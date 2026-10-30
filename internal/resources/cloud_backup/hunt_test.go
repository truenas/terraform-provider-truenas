// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloud_backup_test

// Adversarial bug-hunt test: set every API-accepted attribute of
// truenas_cloud_backup to a non-default value, assert the read-back re-plans
// empty, then import. cloud_backup.create validates the credential/bucket
// against the real remote endpoint at apply time, so this test is gated
// behind the TF_ACC_S3_* env vars (cloudBackupS3Env skips when they are
// unset). enabled=false and no sync is triggered, so no data is uploaded.

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestHuntCloudBackup_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)

	endpoint, access, secret, bucket := cloudBackupS3Env(t)
	name := acctest.RandName("tfhunt-cb")

	cfg := acctest.ProviderConfig() + huntCloudBackupConfig(name, endpoint, access, secret, bucket)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCloudBackupDestroyed("/mnt/" + acctest.TestPool() + "/" + name),
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				Config:           cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				ResourceName:      "truenas_cloud_backup.test",
				ImportState:       true,
				ImportStateVerify: true,
				// password: get_instance only returns it unmasked to a
				// sufficiently-scoped credential. attributes: the API expands
				// the JSON blob with server defaults, so an import can't
				// reproduce the config's minimal form.
				ImportStateVerifyIgnore: []string{"password", "attributes"},
			},
		},
	})
}

// huntCloudBackupConfig is a cloud backup task with every Optional/Computed
// attribute set to a non-default value. absolute_paths is left at its default
// (false) because it is mutually exclusive with snapshot=true, and password_wo
// is left unset because exactly one of password/password_wo may be set.
func huntCloudBackupConfig(name, endpoint, access, secret, bucket string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "cb" {
  name = "%[1]s/%[2]s"
}

# cache_path must live on a pool (the API rejects /tmp), and keeping it out
# of the backup source avoids restic caching inside what it backs up.
resource "truenas_dataset" "cbcache" {
  name = "%[1]s/%[2]s-cache"
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
  description      = %[2]q
  path             = truenas_dataset.cb.mountpoint
  credentials      = truenas_cloudsync_credentials.cb.id
  attributes       = jsonencode({ bucket = %[6]q, folder = %[2]q })
  password         = "tfhunt-cloudbackup-encpw"
  keep_last        = 3
  enabled          = false
  snapshot         = true
  pre_script       = "echo pre"
  post_script      = "echo post"
  include          = ["*.log"]
  exclude          = ["*.tmp"]
  transfer_setting = "PERFORMANCE"
  cache_path       = truenas_dataset.cbcache.mountpoint
  rate_limit       = 1024
  schedule = {
    minute = "15"
    hour   = "3"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}
`, acctest.TestPool(), name, access, secret, endpoint, bucket)
}

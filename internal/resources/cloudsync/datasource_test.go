// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// cloudSyncS3Config is a self-contained cloud sync task. An S3 credential with
// region set is never contacted by cloudsync.create (the region lookup is
// skipped), so the task is creatable on any box with a fake endpoint.
func cloudSyncS3Config(name, body string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "src" {
  name = "%[1]s/%[2]s"
}

resource "truenas_cloudsync_credentials" "c" {
  name = %[2]q
  provider_config = jsonencode({
    type              = "S3"
    access_key_id     = "AKIATFACC"
    secret_access_key = "tfacc"
    endpoint          = "https://s3.example.com/"
    region            = "us-east-1"
  })
}

resource "truenas_cloudsync_task" "test" {
  description   = %[2]q
  path          = truenas_dataset.src.mountpoint
  credentials   = truenas_cloudsync_credentials.c.id
  direction     = "PUSH"
  transfer_mode = "COPY"
  enabled       = false
  attributes    = jsonencode({ bucket = "b", folder = "x" })
  schedule = { minute = "15", hour = "3", dom = "*", month = "*", dow = "*" }
%[3]s
}
`, acctest.TestPool(), name, body)
}

// TestAccCloudSync_dataSource reads a cloud sync task through the
// truenas_cloudsync_task data source. Every read used to fail with a Value
// Conversion Error on schedule (a plain struct cannot hold the null that a
// Computed nested attribute has in config) (B-017).
func TestAccCloudSync_dataSource(t *testing.T) {
	name := acctest.RandName("tf-acc-cs")
	cfg := acctest.ProviderConfig() + cloudSyncS3Config(name, "") + `
data "truenas_cloudsync_task" "d" {
  description = truenas_cloudsync_task.test.description
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: cfg,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrPair("data.truenas_cloudsync_task.d", "id", "truenas_cloudsync_task.test", "id"),
				resource.TestCheckResourceAttrPair("data.truenas_cloudsync_task.d", "path", "truenas_cloudsync_task.test", "path"),
				resource.TestCheckResourceAttrPair("data.truenas_cloudsync_task.d", "credentials", "truenas_cloudsync_task.test", "credentials"),
				resource.TestCheckResourceAttr("data.truenas_cloudsync_task.d", "schedule.minute", "15"),
				resource.TestCheckResourceAttr("data.truenas_cloudsync_task.d", "schedule.hour", "3"),
				resource.TestCheckResourceAttr("data.truenas_cloudsync_task.d", "schedule.dow", "*"),
				resource.TestCheckResourceAttr("data.truenas_cloudsync_task.d", "enabled", "false"),
				resource.TestCheckResourceAttr("data.truenas_cloudsync_task.d", "transfer_mode", "COPY"),
			),
		}},
	})
}

// TestAccCloudSync_bwlimitTimeNeedsLeadingZero: the server zero-pads a
// bwlimit time ("8:00" -> "08:00"), which used to fail the apply with an
// inconsistent result. The schema now rejects the unpadded form at plan time,
// before anything is created.
func TestAccCloudSync_bwlimitTimeNeedsLeadingZero(t *testing.T) {
	name := acctest.RandName("tf-acc-cs")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Rejected at plan time. First, not last: post-test destroy uses
			// the last step's config.
			{
				Config: acctest.ProviderConfig() + cloudSyncS3Config(name, `
  bwlimit = [{ time = "8:00", bandwidth = 1048576 }]
`),
				ExpectError: regexp.MustCompile(`leading\s+zero`),
			},
			{
				Config: acctest.ProviderConfig() + cloudSyncS3Config(name, `
  bwlimit = [{ time = "08:00", bandwidth = 1048576 }]
`),
				Check: resource.TestCheckResourceAttr("truenas_cloudsync_task.test", "bwlimit.0.time", "08:00"),
			},
		},
	})
}

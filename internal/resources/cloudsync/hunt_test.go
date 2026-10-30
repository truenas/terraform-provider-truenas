// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync_test

// Adversarial bug-hunt test: set every API-accepted attribute of
// truenas_cloudsync_task to a non-default value, assert the read-back
// re-plans empty, then import. It reuses cloudSyncS3Config, whose S3
// credential has region set so cloudsync.create never contacts the (fake)
// endpoint, and enabled=false so the task is never run — lab-safe.

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestHuntCloudSync_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)

	name := acctest.RandName("tfhunt-cs")

	// Every Optional/Computed knob the base config leaves at its default,
	// flipped to a non-default value. encryption_* are write-only (TF >= 1.11).
	body := `
  snapshot              = true
  transfers             = 8
  follow_symlinks       = true
  create_empty_src_dirs = true
  encryption            = true
  filename_encryption   = true
  encryption_password   = "tfhuntpw"
  encryption_salt       = "tfhuntsalt"
  encryption_wo_version = 1
  bwlimit               = [{ time = "08:00", bandwidth = 1048576 }]
  include               = ["*.log"]
  exclude               = ["*.tmp"]
  pre_script            = "echo pre"
  post_script           = "echo post"
`
	cfg := acctest.ProviderConfig() + cloudSyncS3Config(name, body)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				Config:           cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				ResourceName:      "truenas_cloudsync_task.test",
				ImportState:       true,
				ImportStateVerify: true,
				// The server expands the provider attributes JSON with all
				// rclone keys (region/storage_class/fast_list/encryption) on
				// read; the plan treats it as an equivalent subset (empty
				// re-plan passes), but raw ImportStateVerify string-compares.
				ImportStateVerifyIgnore: []string{"encryption_password", "encryption_salt", "encryption_wo_version", "attributes"},
			},
		},
	})
}

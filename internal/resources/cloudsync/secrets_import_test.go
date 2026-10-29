// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccCloudSync_importPlan: import stored the full server attributes
// document, so the plan showed an update (B-118).
func TestAccCloudSync_importPlan(t *testing.T) {
	name := acctest.RandName("tffix3shtasks-cs")
	cfg := acctest.ProviderConfig() + cloudSyncS3Config(name, "")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ResourceName: "truenas_cloudsync_task.test", ImportState: true, ImportStateKind: resource.ImportBlockWithID},
		},
	})
}

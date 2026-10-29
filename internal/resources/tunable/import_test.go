// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package tunable_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccTunable_importUpdateInitramfs: ZFS tunable with update_initramfs set,
// then import + re-plan with the same config must be empty: the plan used to
// show an update that re-ran the tunable job (B-116).
func TestAccTunable_importUpdateInitramfs(t *testing.T) {
	acctest.DisruptiveCheck(t)
	cfg := acctest.ProviderConfig() + `
resource "truenas_tunable" "test" {
  var              = "zfs_txg_timeout"
  value            = "5"
  type             = "ZFS"
  comment          = "tfhunt"
  update_initramfs = false
}
`
	var id string
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, Check: func(s *terraform.State) error {
				id = s.RootModule().Resources["truenas_tunable.test"].Primary.ID
				return nil
			}},
			{Config: acctest.ProviderConfig() + `
removed {
  from = truenas_tunable.test
  lifecycle {
    destroy = false
  }
}
`},
			{Config: cfg, ResourceName: "truenas_tunable.test", ImportState: true, ImportStatePersist: true,
				ImportStateIdFunc: func(*terraform.State) (string, error) { return id, nil }},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
			}},
		},
	})
}

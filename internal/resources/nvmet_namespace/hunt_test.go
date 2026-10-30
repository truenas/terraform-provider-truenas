// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_namespace_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntNVMetNamespace_devZvolFormTyped: /dev/zvol/... form with device_type.
func TestHuntNVMetNamespace_devZvolFormTyped(t *testing.T) {
	base := acctest.RandName("tfhuntblknsdt")
	cfg := acctest.ProviderConfig() + nsFixtures(base) + `
resource "truenas_nvmet_namespace" "test" {
  subsys_id   = truenas_nvmet_subsys.s.id
  device_type = "ZVOL"
  device_path = "/dev/zvol/${truenas_zvol.a.name}"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

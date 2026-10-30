// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_subsys_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntNVMetSubsys_lifecycle: all attrs, empty plan, import, unset.
func TestHuntNVMetSubsys_lifecycle(t *testing.T) {
	name := acctest.RandName("tfhuntblklc")
	full := `
  allow_any_host = true
  ieee_oui       = "AABBCC"
  pi_enable      = true
  qid_max        = 64
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: subsysCfg(name, full)},
			{Config: subsysCfg(name, full), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{ResourceName: "truenas_nvmet_subsys.test", ImportState: true, ImportStateVerify: true},
			{Config: subsysCfg(name, full), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{Config: subsysCfg(name, `  ieee_oui = ""`), Check: resource.TestCheckResourceAttr("truenas_nvmet_subsys.test", "ieee_oui", "")},
			{Config: subsysCfg(name, ``), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

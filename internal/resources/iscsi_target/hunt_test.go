// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_target_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntISCSITarget_lifecycle: create with groups + auth_networks, plan
// empty, change mode-independent attr, remove attrs, import.
func TestHuntISCSITarget_lifecycle(t *testing.T) {
	pc := acctest.RandName("tfhunt-tgt-portal")
	name := acctest.RandName("tfhunttgtlc")
	full := `
  alias = "a1"
  groups = [
    {
      portal     = truenas_iscsi_portal.fixture.id
      authmethod = "NONE"
    }
  ]
  auth_networks = ["10.0.0.0/8", "2001:db8::/32"]
  iscsi_parameters = {
    queued_commands = 32
  }
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: targetCfg(pc, name, full)},
			{
				Config:           targetCfg(pc, name, full),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				ResourceName:      "truenas_iscsi_target.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{Config: targetCfg(pc, name, full), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			// Change only the name (in-place) with groups intact.
			{Config: targetCfg(pc, name+"x", full)},
			// Empty groups list explicitly.
			{
				Config: targetCfg(pc, name+"x", `
  groups = []
  auth_networks = []
`),
				Check: resource.TestCheckResourceAttr("truenas_iscsi_target.test", "groups.#", "0"),
			},
		},
	})
}

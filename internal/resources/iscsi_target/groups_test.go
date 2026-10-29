// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_target_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func targetCfg(portalComment, name, targetBody string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_iscsi_portal" "fixture" {
  comment = %q
  listen = [
    {
      ip = "0.0.0.0"
    }
  ]
}

resource "truenas_iscsi_target" "test" {
  name = %q
%s
}
`, portalComment, name, targetBody)
}

// TestAccISCSITarget_groupWithoutAuthmethod: authmethod is Optional with a
// server default of NONE. A group with only `portal` set used to send
// authmethod "" and fail create/update; it must apply, read back NONE,
// re-plan empty and import cleanly, on create and on an update that adds
// the group.
func TestAccISCSITarget_groupWithoutAuthmethod(t *testing.T) {
	pc := acctest.RandName("tf-acc-tgt-portal")
	name := acctest.RandName("tfacctgtnoauth")
	group := `
  groups = [
    {
      portal = truenas_iscsi_portal.fixture.id
    }
  ]
`
	empty := resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckISCSITargetDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: targetCfg(pc, name, group),
				Check:  resource.TestCheckResourceAttr("truenas_iscsi_target.test", "groups.0.authmethod", "NONE"),
			},
			{Config: targetCfg(pc, name, group), ConfigPlanChecks: empty},
			{ResourceName: "truenas_iscsi_target.test", ImportState: true, ImportStateVerify: true},
			// Update path: drop the group, then add it back without authmethod.
			{
				Config: targetCfg(pc, name, "  groups = []"),
				Check:  resource.TestCheckResourceAttr("truenas_iscsi_target.test", "groups.#", "0"),
			},
			{
				Config: targetCfg(pc, name, group),
				Check:  resource.TestCheckResourceAttr("truenas_iscsi_target.test", "groups.0.authmethod", "NONE"),
			},
			{Config: targetCfg(pc, name, group), ConfigPlanChecks: empty},
		},
	})
}

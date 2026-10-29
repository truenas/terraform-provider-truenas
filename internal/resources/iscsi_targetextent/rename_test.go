// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_targetextent_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// renameCfg maps one extent into a target.
func renameCfg(pool, base, extentName string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "z" {
  name    = "%[1]s/%[2]s"
  volsize = 67108864
}
resource "truenas_iscsi_extent" "e" {
  name = %[3]q
  type = "DISK"
  disk = "zvol/${truenas_zvol.z.name}"
}
resource "truenas_iscsi_target" "t" {
  name = %[2]q
}
resource "truenas_iscsi_targetextent" "m" {
  target = truenas_iscsi_target.t.id
  extent = truenas_iscsi_extent.e.id
  lunid  = 3
}
`, pool, base, extentName)
}

// TestAccISCSITargetExtent_extentRename renames a mapped extent. The API
// renames in place, so the extent must keep its id, serial and NAA (the
// disk identity initiators see) and the LUN mapping must stay untouched.
// The extent name used to be RequiresReplace, which gave initiators a new
// disk and recreated the mapping (B-041).
func TestAccISCSITargetExtent_extentRename(t *testing.T) {
	base := acctest.RandName("tfacctgtren")
	pool := acctest.TestPool()
	var serial, naa, id string
	keep := func(s *terraform.State) error {
		a := s.RootModule().Resources["truenas_iscsi_extent.e"].Primary.Attributes
		if a["serial"] != serial || a["naa"] != naa || a["id"] != id {
			return fmt.Errorf("extent identity changed: id %s->%s serial %s->%s naa %s->%s", id, a["id"], serial, a["serial"], naa, a["naa"])
		}
		return nil
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: renameCfg(pool, base, base+"-e1"),
				Check: func(s *terraform.State) error {
					a := s.RootModule().Resources["truenas_iscsi_extent.e"].Primary.Attributes
					serial, naa, id = a["serial"], a["naa"], a["id"]
					return nil
				},
			},
			{
				Config: renameCfg(pool, base, base+"-e2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_iscsi_extent.e", plancheck.ResourceActionUpdate),
					plancheck.ExpectResourceAction("truenas_iscsi_targetextent.m", plancheck.ResourceActionNoop),
				}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_extent.e", "name", base+"-e2"),
					resource.TestCheckResourceAttr("truenas_iscsi_targetextent.m", "lunid", "3"),
					keep,
				),
			},
			{Config: renameCfg(pool, base, base+"-e2"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_targetextent_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func huntTEFixtures(base string) string {
	return fmt.Sprintf(`
resource "truenas_iscsi_target" "t" {
  name = "%[2]s"
}
resource "truenas_zvol" "z" {
  count   = 2
  name    = "%[1]s/%[2]s-${count.index}"
  volsize = 67108864
}
resource "truenas_iscsi_extent" "e" {
  count = 2
  name  = "%[2]s-${count.index}"
  type  = "DISK"
  disk  = "zvol/${truenas_zvol.z[count.index].name}"
}
`, acctest.TestPool(), base)
}

// TestHuntISCSITargetExtent_autoLun: two LUN mappings on one target with lunid
// omitted (auto-assign), empty plan, import, then pin one lunid explicitly,
// then unpin it.
func TestHuntISCSITargetExtent_autoLun(t *testing.T) {
	base := acctest.RandName("tfhuntblkte")
	auto := acctest.ProviderConfig() + huntTEFixtures(base) + `
resource "truenas_iscsi_targetextent" "m0" {
  target = truenas_iscsi_target.t.id
  extent = truenas_iscsi_extent.e[0].id
}
resource "truenas_iscsi_targetextent" "m1" {
  target = truenas_iscsi_target.t.id
  extent = truenas_iscsi_extent.e[1].id
}
`
	pinned := acctest.ProviderConfig() + huntTEFixtures(base) + `
resource "truenas_iscsi_targetextent" "m0" {
  target = truenas_iscsi_target.t.id
  extent = truenas_iscsi_extent.e[0].id
}
resource "truenas_iscsi_targetextent" "m1" {
  target = truenas_iscsi_target.t.id
  extent = truenas_iscsi_extent.e[1].id
  lunid  = 7
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: auto},
			{Config: auto, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{ResourceName: "truenas_iscsi_targetextent.m1", ImportState: true, ImportStateVerify: true},
			{Config: auto, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{Config: pinned, Check: resource.TestCheckResourceAttr("truenas_iscsi_targetextent.m1", "lunid", "7")},
			{Config: pinned, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

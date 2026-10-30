// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_extent_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntISCSIExtent_allAttrsThenUnset: set every optional attribute to a
// non-default value, plan must be empty, import must verify, then remove them
// from config (plan should be empty / no error).
func TestHuntISCSIExtent_allAttrsThenUnset(t *testing.T) {
	base := acctest.RandName("tfhuntextall")
	pool := acctest.TestPool()
	zv := fmt.Sprintf(`
resource "truenas_zvol" "a" {
  name    = "%s/%s"
  volsize = 67108864
}
`, pool, base)
	full := acctest.ProviderConfig() + zv + fmt.Sprintf(`
resource "truenas_iscsi_extent" "test" {
  name            = %q
  type            = "DISK"
  disk            = "zvol/${truenas_zvol.a.name}"
  comment         = "c1"
  blocksize       = 4096
  pblocksize      = true
  avail_threshold = 50
  insecure_tpc    = false
  xen             = true
  rpm             = "7200"
  enabled         = false
}
`, base)
	bare := acctest.ProviderConfig() + zv + fmt.Sprintf(`
resource "truenas_iscsi_extent" "test" {
  name = %q
  type = "DISK"
  disk = "zvol/${truenas_zvol.a.name}"
}
`, base)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: full},
			{Config: full, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{ResourceName: "truenas_iscsi_extent.test", ImportState: true, ImportStateVerify: true},
			{Config: full, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{Config: bare},
			{Config: bare, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

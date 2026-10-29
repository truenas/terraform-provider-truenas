// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_portal_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func portalCfg(comment, listen string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_iscsi_portal" "test" {
  comment = %q
  listen  = %s
}
`, comment, listen)
}

func portalEmptyPlan() resource.ConfigPlanChecks {
	return resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
}

// TestAccISCSIPortal_portReadOnly: no supported TrueNAS release accepts a
// per-listen port; the provider never sent it, so any value other than the
// global listen_port failed with "inconsistent result" (B-101). port is now
// Computed-only and a config value is rejected at plan time.
func TestAccISCSIPortal_portReadOnly(t *testing.T) {
	c := acctest.RandName("tf-acc-portal")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      portalCfg(c, `[{ ip = "0.0.0.0", port = 3261 }]`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`(?s)Invalid Configuration for Read-Only Attribute.*port`),
			},
		},
	})
}

// TestAccISCSIPortal_reorderListen: wildcard v4 + v6 in one portal, import,
// reorder, shrink to one IP, then add one back. TrueNAS keeps its stored
// order on update, so a reordered config used to fail with "inconsistent
// result" (B-102); the provider now reports the list in config order when
// the IPs match.
func TestAccISCSIPortal_reorderListen(t *testing.T) {
	c := acctest.RandName("tf-acc-portal")
	two := `[{ ip = "::" }, { ip = "0.0.0.0" }]`
	swapped := `[{ ip = "0.0.0.0" }, { ip = "::" }]`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: portalCfg(c, two)},
			{Config: portalCfg(c, two), ConfigPlanChecks: portalEmptyPlan()},
			{ResourceName: "truenas_iscsi_portal.test", ImportState: true, ImportStateVerify: true},
			{
				Config: portalCfg(c, swapped),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_portal.test", "listen.0.ip", "0.0.0.0"),
					resource.TestCheckResourceAttr("truenas_iscsi_portal.test", "listen.1.ip", "::"),
				),
			},
			{Config: portalCfg(c, swapped), ConfigPlanChecks: portalEmptyPlan()},
			{Config: portalCfg(c, `[{ ip = "::" }]`)},
			{Config: portalCfg(c, swapped)},
			{Config: portalCfg(c, swapped), ConfigPlanChecks: portalEmptyPlan()},
		},
	})
}

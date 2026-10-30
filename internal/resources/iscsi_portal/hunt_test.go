// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_portal_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntISCSIPortal_allFields is an adversarial value round-trip for the iSCSI
// portal: it sets every writable attribute to a non-default value (comment, and
// a listen entry bound to the box IP) and proves each round-trips by (1)
// applying, (2) re-planning the identical config and asserting an empty plan, and
// (3) importing with ImportStateVerify.
//
// Creating a portal is a listen-IP/port config only; it does not touch the global
// iSCSI basename, so it is lab-safe. The listen port is read-only on all
// supported releases (set via truenas_iscsi_global instead), and tag/id are
// server-assigned — so comment and listen.ip are the only writable attributes,
// and both round-trip, leaving nothing for ImportStateVerify to ignore.
func TestHuntISCSIPortal_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)
	comment := acctest.RandName("tfhunt-portal")
	cfg := acctest.ProviderConfig() + testAccISCSIPortalConfig(comment)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckISCSIPortalDestroyed(comment),
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_iscsi_portal.test", "id"),
					resource.TestCheckResourceAttr("truenas_iscsi_portal.test", "comment", comment),
					resource.TestCheckResourceAttr("truenas_iscsi_portal.test", "listen.0.ip", acctest.EndpointHost()),
					resource.TestCheckResourceAttrSet("truenas_iscsi_portal.test", "tag"),
				),
			},
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:      "truenas_iscsi_portal.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

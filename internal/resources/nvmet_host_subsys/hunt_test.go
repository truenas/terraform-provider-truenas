// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_host_subsys_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntNVMetHostSubsys_allFields is an adversarial value round-trip for the
// NVMe-oF host/subsystem association: it sets both writable attributes (host_id,
// subsys_id) from freshly created host and subsystem fixtures and proves they
// round-trip by (1) applying, (2) re-planning the identical config and asserting
// an empty plan, and (3) importing with ImportStateVerify.
//
// host_id and subsys_id are the only writable attributes and both are Required
// (and RequiresReplace), so there is nothing for ImportStateVerify to ignore. It
// creates and destroys its own RandName/RandNQN host and subsystem and never
// touches the box's live association.
func TestHuntNVMetHostSubsys_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)
	hostNQN := acctest.RandNQN()
	subsysName := acctest.RandName("tfhunt-host-subsys")
	cfg := acctest.ProviderConfig() + testAccNVMetHostSubsysConfig(hostNQN, subsysName)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNVMetHostSubsysDestroyed,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_nvmet_host_subsys.test", "id"),
					resource.TestCheckResourceAttrPair("truenas_nvmet_host_subsys.test", "host_id", "truenas_nvmet_host.test", "id"),
					resource.TestCheckResourceAttrPair("truenas_nvmet_host_subsys.test", "subsys_id", "truenas_nvmet_subsys.test", "id"),
				),
			},
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:      "truenas_nvmet_host_subsys.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

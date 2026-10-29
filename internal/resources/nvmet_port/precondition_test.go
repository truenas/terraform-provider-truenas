// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_port_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// attachedPortCfg is an enabled TCP port with a subsystem attached through
// truenas_nvmet_port_subsys.
func attachedPortCfg(name string, trsvcid, maxQueue int) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_nvmet_subsys" "s" {
  name = %[1]q
}
resource "truenas_nvmet_port" "test" {
  addr_trtype    = "TCP"
  addr_traddr    = %[2]q
  addr_trsvcid   = %[3]d
  max_queue_size = %[4]d
  enabled        = true
}
resource "truenas_nvmet_port_subsys" "ps" {
  port_id   = truenas_nvmet_port.test.id
  subsys_id = truenas_nvmet_subsys.s.id
}
`, name, acctest.EndpointHost(), trsvcid, maxQueue)
}

// checkPortServerEnabled asserts the port is enabled on the server, so a
// change that disabled it to get past the server precondition re-enabled it.
func checkPortServerEnabled(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := strconv.ParseInt(s.RootModule().Resources["truenas_nvmet_port.test"].Primary.ID, 10, 64)
		if err != nil {
			return err
		}
		if inst := acctest.GetInstance(t, "nvmet.port", id); inst["enabled"] != true {
			return fmt.Errorf("server enabled = %v, want true", inst["enabled"])
		}
		return nil
	}
}

// TestAccNVMetPort_changeWithSubsysAttached changes addr_trsvcid and
// max_queue_size on an enabled port that has a subsystem attached. While the
// NVMe-oF target runs, TrueNAS rejects any change to such a port except
// enabled ("Cannot change addr_trsvcid on an active port. Disable first to
// allow change."), so the provider disables the port, applies the change and
// re-enables it. The change must be an in-place update, the port must be
// enabled again afterwards, and the re-plan must be empty. With the NVMe-oF
// service stopped the server has no precondition and the test still checks
// the update path.
func TestAccNVMetPort_changeWithSubsysAttached(t *testing.T) {
	name := acctest.RandName("tfaccportpre")
	const r = "truenas_nvmet_port.test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNVMetPortDestroyed,
		Steps: []resource.TestStep{
			{Config: attachedPortCfg(name, 14431, 64), Check: checkPortServerEnabled(t)},
			{
				Config: attachedPortCfg(name, 14432, 128),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(r, plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(r, "addr_trsvcid", "14432"),
					resource.TestCheckResourceAttr(r, "max_queue_size", "128"),
					resource.TestCheckResourceAttr(r, "enabled", "true"),
					checkPortServerEnabled(t),
				),
			},
			{Config: attachedPortCfg(name, 14432, 128), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

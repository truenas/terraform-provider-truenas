// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_subsys_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

const subsysAddr = "truenas_nvmet_subsys.test"

// TestAccNVMetSubsys_subnqnEmpty: subnqn = "" used to be documented as the
// way to clear a custom NQN, but the server answers it by deriving
// <basenqn>:<name>, so the apply always failed with "inconsistent result"
// (B-103). "" is now rejected at plan time, on create and on update.
func TestAccNVMetSubsys_subnqnEmpty(t *testing.T) {
	name := acctest.RandName("tfaccsubnqn")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: subsysCfg(name, `  subnqn = ""`), PlanOnly: true, ExpectError: regexp.MustCompile(`(?s)subnqn.*at least 1`)},
			{Config: subsysCfg(name, `  subnqn = "nqn.2014-08.org.example:tfaccsubnqn"`)},
			{Config: subsysCfg(name, `  subnqn = ""`), ExpectError: regexp.MustCompile(`(?s)subnqn.*at least 1`)},
			// Leaving subnqn out keeps the custom NQN.
			{
				Config:           subsysCfg(name, ``),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				Check:            resource.TestCheckResourceAttr(subsysAddr, "subnqn", "nqn.2014-08.org.example:tfaccsubnqn"),
			},
		},
	})
}

// renameCfg is a subsystem attached to a disabled port and a host, so the
// rename test can check that the mappings survive.
func renameCfg(name string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_nvmet_subsys" "test" {
  name = %[1]q
}
resource "truenas_nvmet_port" "p" {
  addr_trtype  = "TCP"
  addr_traddr  = %[2]q
  addr_trsvcid = 14433
  enabled      = false
}
resource "truenas_nvmet_port_subsys" "ps" {
  port_id   = truenas_nvmet_port.p.id
  subsys_id = truenas_nvmet_subsys.test.id
}
resource "truenas_nvmet_host" "h" {
  hostnqn = "nqn.2014-08.org.example:%[1]s"
}
resource "truenas_nvmet_host_subsys" "hs" {
  host_id   = truenas_nvmet_host.h.id
  subsys_id = truenas_nvmet_subsys.test.id
}
`, name, acctest.EndpointHost())
}

// TestAccNVMetSubsys_rename renames a subsystem that has port and host
// mappings. nvmet.subsys.update renames in place, so the subsystem must keep
// its id, serial and subnqn, and the mappings must not be replaced. name
// used to be RequiresReplace, which dropped the namespaces and mappings and
// gave hosts a new serial (B-041).
func TestAccNVMetSubsys_rename(t *testing.T) {
	name := acctest.RandName("tfaccsubren")
	var id, serial, subnqn string
	keep := func(s *terraform.State) error {
		a := s.RootModule().Resources[subsysAddr].Primary.Attributes
		if a["id"] != id || a["serial"] != serial || a["subnqn"] != subnqn {
			return fmt.Errorf("subsystem identity changed: id %s->%s serial %s->%s subnqn %s->%s", id, a["id"], serial, a["serial"], subnqn, a["subnqn"])
		}
		return nil
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: renameCfg(name),
				Check: func(s *terraform.State) error {
					a := s.RootModule().Resources[subsysAddr].Primary.Attributes
					id, serial, subnqn = a["id"], a["serial"], a["subnqn"]
					return nil
				},
			},
			{
				Config: renameCfg(name + "r"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(subsysAddr, plancheck.ResourceActionUpdate),
					plancheck.ExpectResourceAction("truenas_nvmet_port_subsys.ps", plancheck.ResourceActionNoop),
					plancheck.ExpectResourceAction("truenas_nvmet_host_subsys.hs", plancheck.ResourceActionNoop),
				}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(subsysAddr, "name", name+"r"),
					keep,
				),
			},
			{Config: renameCfg(name + "r"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

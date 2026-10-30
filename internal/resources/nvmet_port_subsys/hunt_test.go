// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_port_subsys_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntNVMetPortSubsys_subsysReplace: port/subsys/host + both association
// types; empty plan; import both; then rename the subsystem (RequiresReplace)
// which must cascade cleanly through the associations.
func TestHuntNVMetPortSubsys_subsysReplace(t *testing.T) {
	base := acctest.RandName("tfhuntblkps")
	cfg := func(sfx string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_nvmet_subsys" "s" {
  name = "%[1]s%[2]s"
}
resource "truenas_nvmet_port" "p" {
  addr_trtype  = "TCP"
  addr_traddr  = "0.0.0.0"
  addr_trsvcid = 4494
  enabled      = false
}
resource "truenas_nvmet_host" "h" {
  hostnqn = "nqn.2014-08.org.example:%[1]s"
}
resource "truenas_nvmet_port_subsys" "test" {
  port_id   = truenas_nvmet_port.p.id
  subsys_id = truenas_nvmet_subsys.s.id
}
resource "truenas_nvmet_host_subsys" "hs" {
  host_id   = truenas_nvmet_host.h.id
  subsys_id = truenas_nvmet_subsys.s.id
}
`, base, sfx)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg("")},
			{Config: cfg(""), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{ResourceName: "truenas_nvmet_port_subsys.test", ImportState: true, ImportStateVerify: true},
			{ResourceName: "truenas_nvmet_host_subsys.hs", ImportState: true, ImportStateVerify: true},
			{Config: cfg(""), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{Config: cfg("r")},
			{Config: cfg("r"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

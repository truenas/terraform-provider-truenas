// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_port_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func huntPortCfg(body string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_nvmet_port" "test" {
  addr_trtype = "TCP"
%s
}
`, body)
}

// TestHuntNVMetPort_lifecycle: enabled (default) port on wildcard v4, empty
// plan, import, change svcid + queue size on the enabled port, move to "::",
// then unset optional attrs.
func TestHuntNVMetPort_lifecycle(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: huntPortCfg(`  addr_traddr = "0.0.0.0"
  addr_trsvcid = 4496`)},
			{Config: huntPortCfg(`  addr_traddr = "0.0.0.0"
  addr_trsvcid = 4496`), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{ResourceName: "truenas_nvmet_port.test", ImportState: true, ImportStateVerify: true},
			{Config: huntPortCfg(`  addr_traddr = "0.0.0.0"
  addr_trsvcid = 4496`), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{Config: huntPortCfg(`  addr_traddr = "0.0.0.0"
  addr_trsvcid = 4495
  max_queue_size = 64
  inline_data_size = 16384`)},
			{Config: huntPortCfg(`  addr_traddr = "::"
  addr_trsvcid = 4495
  max_queue_size = 64
  inline_data_size = 16384`), Check: resource.TestCheckResourceAttr("truenas_nvmet_port.test", "addr_adrfam", "IPV6")},
			{Config: huntPortCfg(`  addr_traddr = "::"
  addr_trsvcid = 4495`), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

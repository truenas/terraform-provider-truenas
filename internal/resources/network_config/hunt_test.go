// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package network_config_test

// Adversarial bug-hunt tests: apply the low-risk network fields and assert an
// empty re-plan, then import. Only fields that cannot drop connectivity are
// touched (service_announcement, domains, hosts, httpproxy); gateways and
// nameservers are left alone. The original config is captured and restored by
// readNetConfigSnapshot/restore (defined in acceptance_test.go), which knows
// which fields are writable.

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// service_announcement all on, a domains list, a hosts entry, empty proxy.
// Assert the read-back re-plans empty, then import.
func TestHuntNetworkConfig_lowRiskFields(t *testing.T) {
	acctest.DisruptiveCheck(t)
	orig := readNetConfigSnapshot(t)
	t.Cleanup(func() { orig.restore(t) })
	c := acctest.ProviderConfig() + `
resource "truenas_network_config" "test" {
  domains   = ["hunt.example", "hunt2.example"]
  hosts     = ["192.0.2.50 hunthost"]
  httpproxy = ""

  service_announcement = {
    mdns    = true
    netbios = true
    wsd     = true
  }
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c},
			{Config: c, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:      "truenas_network_config.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

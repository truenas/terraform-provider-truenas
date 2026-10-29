// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package static_route_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func routeConfig(dest, gw string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_static_route" "test" {
  destination = %q
  gateway     = %q
  description = "tf-acc normalize"
}
`, dest, gw)
}

// routeSpellingSteps applies a route written in a spelling TrueNAS rewrites,
// re-plans (empty), looks it up through the data source with the same
// spelling, then removes it from state, imports it, and plans the same
// configuration again (empty).
func routeSpellingSteps(dest, gw, storedDest string) []resource.TestStep {
	cfg := routeConfig(dest, gw)
	var id string
	return []resource.TestStep{
		{
			Config: cfg,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("truenas_static_route.test", "destination", dest),
				resource.TestCheckResourceAttr("truenas_static_route.test", "gateway", gw),
				func(s *terraform.State) error {
					id = s.RootModule().Resources["truenas_static_route.test"].Primary.ID
					return nil
				},
			),
		},
		{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{
			Config: cfg + fmt.Sprintf(`
data "truenas_static_route" "lookup" {
  destination = %q
  depends_on  = [truenas_static_route.test]
}
`, dest),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttrPair("data.truenas_static_route.lookup", "id", "truenas_static_route.test", "id"),
				resource.TestCheckResourceAttr("data.truenas_static_route.lookup", "destination", dest),
			),
		},
		{Config: acctest.ProviderConfig() + `
removed {
  from = truenas_static_route.test
  lifecycle {
    destroy = false
  }
}
`},
		{
			Config:             cfg,
			ResourceName:       "truenas_static_route.test",
			ImportState:        true,
			ImportStatePersist: true,
			ImportStateIdFunc:  func(*terraform.State) (string, error) { return id, nil },
			ImportStateCheck: func(is []*terraform.InstanceState) error {
				if len(is) != 1 || is[0].Attributes["destination"] != storedDest {
					return fmt.Errorf("imported destination = %v, want the server form %q", is, storedDest)
				}
				return nil
			},
		},
		{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}
}

// TestAccStaticRoute_ipv6Compressed guards B-076/B-131: TrueNAS stores IPv6
// destinations and gateways exploded; a compressed spelling must apply,
// re-plan, look up, and import cleanly.
func TestAccStaticRoute_ipv6Compressed(t *testing.T) {
	acctest.DisruptiveCheck(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: routeSpellingSteps("2001:db8:77::/64", "fe80::e21f:2bff:fe46:7882",
			"2001:0db8:0077:0000:0000:0000:0000:0000/64"),
	})
}

// TestAccStaticRoute_ipv4HostNoPrefix guards B-076: a bare IPv4 destination
// is stored with /32.
func TestAccStaticRoute_ipv4HostNoPrefix(t *testing.T) {
	acctest.DisruptiveCheck(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps:                    routeSpellingSteps("198.51.100.7", "192.168.0.254", "198.51.100.7/32"),
	})
}

// TestAccStaticRoute_ipv4Netmask guards B-076: a dotted netmask is stored as
// a prefix length.
func TestAccStaticRoute_ipv4Netmask(t *testing.T) {
	acctest.DisruptiveCheck(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps:                    routeSpellingSteps("198.51.101.0/255.255.255.0", "192.168.0.254", "198.51.101.0/24"),
	})
}

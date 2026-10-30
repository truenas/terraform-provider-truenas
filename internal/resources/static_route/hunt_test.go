// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package static_route_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntStaticRoute_lifecycle: change description, unset description,
// change gateway, import.
func TestHuntStaticRoute_lifecycle(t *testing.T) {
	acctest.DisruptiveCheck(t)
	base := acctest.ProviderConfig()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: base + `
resource "truenas_static_route" "test" {
  destination = "198.51.100.0/24"
  gateway     = "192.168.0.254"
  description = "tfhunt"
}
`},
			{Config: base + `
resource "truenas_static_route" "test" {
  destination = "198.51.100.0/24"
  gateway     = "192.168.0.254"
}
`},
			{Config: base + `
resource "truenas_static_route" "test" {
  destination = "198.51.100.0/24"
  gateway     = "192.168.0.254"
}
`, PlanOnly: true},
			{ResourceName: "truenas_static_route.test", ImportState: true, ImportStateVerify: true},
		},
	})
}

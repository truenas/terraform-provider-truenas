// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package static_route_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccStaticRoute_basic tests create, update, and import of a static
// route. The destination uses the TEST-NET-2 documentation range
// (RFC 5737), which is never routable on the test network.
func TestAccStaticRoute_basic(t *testing.T) {
	const destination = "198.51.100.0/24"
	const gateway = "192.168.1.254"
	description := acctest.RandName("tf-acc-route")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckStaticRouteDestroyed(destination),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccStaticRouteConfig(destination, gateway, description),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_static_route.test", "destination", destination),
					resource.TestCheckResourceAttr("truenas_static_route.test", "gateway", gateway),
					resource.TestCheckResourceAttr("truenas_static_route.test", "description", description),
					resource.TestCheckResourceAttrSet("truenas_static_route.test", "id"),
				),
			},
			// Update the description in place.
			{
				Config: acctest.ProviderConfig() + testAccStaticRouteConfig(destination, gateway, description+"-updated"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_static_route.test", "description", description+"-updated"),
				),
			},
			{
				ResourceName:      "truenas_static_route.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccStaticRouteConfig(destination, gateway, description string) string {
	return fmt.Sprintf(`
resource "truenas_static_route" "test" {
  destination = %q
  gateway     = %q
  description = %q
}
`, destination, gateway, description)
}

func testAccCheckStaticRouteDestroyed(destination string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "staticroute.query", [][]any{{"destination", "=", destination}})
		if err != nil {
			return fmt.Errorf("error checking static route %s: %v", destination, err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing staticroute.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("static route %s still exists", destination)
		}
		return nil
	}
}

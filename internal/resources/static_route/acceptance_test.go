package static_route_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccStaticRoute_basic tests create, update, and import of a static route.
// It requires:
//   - TrueNAS_API_KEY set in the environment (checked by acctest.PreCheck)
//   - A running TrueNAS SCALE instance
func TestAccStaticRoute_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccStaticRouteConfig("10.20.0.0/16", "10.20.0.1", "tf-acc-route"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_static_route.test", "destination", "10.20.0.0/16"),
					resource.TestCheckResourceAttr("truenas_static_route.test", "gateway", "10.20.0.1"),
					resource.TestCheckResourceAttr("truenas_static_route.test", "description", "tf-acc-route"),
					resource.TestCheckResourceAttrSet("truenas_static_route.test", "id"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccStaticRouteConfig("10.20.0.0/16", "10.20.0.2", "tf-acc-route-updated"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_static_route.test", "gateway", "10.20.0.2"),
					resource.TestCheckResourceAttr("truenas_static_route.test", "description", "tf-acc-route-updated"),
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

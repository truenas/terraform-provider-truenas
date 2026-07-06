package network_interface_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNetworkInterface_bridge creates a BRIDGE interface (br999) with no
// members and no aliases, verifies it, then destroys it. It deliberately
// avoids touching enp7s0 (the management NIC) or any other physical
// interface.
//
// It requires TF_ACC=1 and credentials set via TRUENAS_API_KEY or
// TRUENAS_USERNAME+TRUENAS_PASSWORD.
func TestAccNetworkInterface_bridge(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccNetworkInterfaceBridgeConfig(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_network_interface.test", "name", "br999"),
					resource.TestCheckResourceAttr("truenas_network_interface.test", "type", "BRIDGE"),
					resource.TestCheckResourceAttrSet("truenas_network_interface.test", "id"),
				),
			},
			{
				ResourceName:      "truenas_network_interface.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccNetworkInterfaceBridgeConfig() string {
	return `
resource "truenas_network_interface" "test" {
  name = "br999"
  type = "BRIDGE"
}
`
}

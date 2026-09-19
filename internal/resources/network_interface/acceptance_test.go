// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package network_interface_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNetworkInterface_bridge would create a BRIDGE interface (br999)
// with no members and no aliases, verify it, then destroy it.
//
// truenas_network_interface changes are staged and then committed with an
// auto-rollback safety window against the box's live networking stack
// (interface.commit / interface.checkin). A memberless, IP-less BRIDGE is
// inert and never touches the management interface, but the commit/checkin
// still exercises the box's global network apply. So this is gated behind
// TRUENAS_TEST_NET_INTERFACE (not just TF_ACC) — run it only against a
// disposable box you are prepared to lose network access to. The 60s
// auto-rollback is the backstop if a commit ever severs connectivity.
func TestAccNetworkInterface_bridge(t *testing.T) {
	if os.Getenv("TRUENAS_TEST_NET_INTERFACE") == "" {
		t.Skip("set TRUENAS_TEST_NET_INTERFACE=1 (on a disposable box) to run the network_interface create/destroy test")
	}
	acctest.PreCheck(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckInterfaceDestroyed("br999"),
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

// testAccCheckInterfaceDestroyed verifies the named interface is gone.
func testAccCheckInterfaceDestroyed(name string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		raw, err := acctest.Client().Call(context.Background(), "interface.query", [][]any{{"id", "=", name}})
		if err != nil {
			return fmt.Errorf("querying interface %s: %w", name, err)
		}
		var results []struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("parsing interface.query: %w", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("interface %s still exists", name)
		}
		return nil
	}
}

func testAccNetworkInterfaceBridgeConfig() string {
	return `
resource "truenas_network_interface" "test" {
  name = "br999"
  type = "BRIDGE"
}
`
}

// TestAccNetworkInterface_datasourceEnp7s0 is a strictly read-only
// datasource lookup of the box's physical management NIC (enp7s0). It never
// creates, modifies, or imports a resource — only a data source Read — so
// it carries none of the commit/checkin risk of TestAccNetworkInterface_bridge.
// It skips itself if enp7s0 is not present on the target host, so it is
// safe to run against any TF_ACC target.
func TestAccNetworkInterface_datasourceEnp7s0(t *testing.T) {
	// PreCheck validates credentials too — acctest.Client() panics without
	// them, so gate on the full precheck rather than TF_ACC alone.
	acctest.PreCheck(t)

	const ifaceName = "enp7s0"

	raw, err := acctest.Client().Call(context.Background(), "interface.query", [][]any{{"id", "=", ifaceName}})
	if err != nil {
		t.Fatalf("querying interface.query for %s: %v", ifaceName, err)
	}
	var results []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &results); err != nil {
		t.Fatalf("parsing interface.query response: %v", err)
	}
	if len(results) == 0 {
		t.Skipf("%s not present on target host: skipping read-only datasource check", ifaceName)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_network_interface" "enp7s0" {
  name = "enp7s0"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_network_interface.enp7s0", "name", "enp7s0"),
					resource.TestCheckResourceAttr("data.truenas_network_interface.enp7s0", "type", "PHYSICAL"),
					resource.TestCheckResourceAttrSet("data.truenas_network_interface.enp7s0", "id"),
				),
			},
		},
	})
}

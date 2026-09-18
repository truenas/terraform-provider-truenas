// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package network_interface_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNetworkInterface_bridge would create a BRIDGE interface (br999)
// with no members and no aliases, verify it, then destroy it.
//
// It is deliberately kept skipped unconditionally (independent of TF_ACC):
// truenas_network_interface changes are staged and then committed with an
// auto-rollback safety window against the box's live networking stack
// (interface.commit / interface.checkin). Even a self-contained BRIDGE
// create/destroy cycle carries a global commit/checkin risk — a failed or
// interrupted commit can leave the box's network configuration in a
// pending, uncommitted, or rolled-back state affecting all interfaces, not
// just the one under test. Enable this manually only against a disposable
// test host you are prepared to lose network access to.
func TestAccNetworkInterface_bridge(t *testing.T) {
	t.Skip("Skipped unconditionally: truenas_network_interface create/destroy carries a " +
		"global commit/checkin risk to the box's live networking stack (interface.commit " +
		"auto-rollback affects all interfaces, not just the one under test). Enable manually " +
		"only against a disposable test host.")

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

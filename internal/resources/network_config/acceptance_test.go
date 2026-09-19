// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package network_config_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNetworkConfigDataSource_basic reads the current TrueNAS global
// network configuration through the truenas_network_config datasource only.
// It never writes: this configuration controls hostname, DNS, and default
// gateways, and the box's real configuration must not be overwritten by an
// acceptance test run.
//
// Requires:
//   - TF_ACC=1
//   - TRUENAS_API_KEY (or TRUENAS_USERNAME+TRUENAS_PASSWORD) set for
//     acctest.PreCheck
//   - A running TrueNAS instance
func TestAccNetworkConfigDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_network_config" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_network_config.test", "id", "network_config"),
					resource.TestCheckResourceAttrSet("data.truenas_network_config.test", "hostname"),
				),
			},
		},
	})
}

// TestAccNetworkConfig_basic is intentionally skipped by default.
// truenas_network_config is a SINGLETON resource that controls hostname,
// DNS, and default gateways: the box this suite runs against is reached
// live over that network configuration, and bringing it under Terraform
// management mutates the box's actual global network configuration
// (network.configuration.update). A naive create/update/destroy acceptance
// test risks changing hostname, ipv4gateway, ipv6gateway, or nameserver1-3
// on a live system and cutting off connectivity entirely.
//
// If this test is ever enabled against a disposable/non-production TrueNAS
// instance, it should:
//  1. Read the current config via the datasource first.
//  2. Only touch a low-risk field (e.g. "httpproxy" or "domains") in the
//     resource config, driving it through a value and back to the value the
//     datasource observed in step 1, so the net effect on the box is a
//     no-op. Never touch hostname, ipv4gateway, ipv6gateway, or
//     nameserver1-3 in an automated test: changing those on a live system
//     can cut off network connectivity.
//  3. Use ImportState with ImportStateId "network_config" to verify import
//     normalizes any ID to the fixed singleton ID.
//
// It is gated behind TRUENAS_TEST_NETWORK_CONFIG (not just TF_ACC). It touches
// ONLY `hostname`: updatePayload sends only the fields set in config, so DNS,
// gateways, and domains are never rewritten and connectivity is preserved (a
// hostname change does not drop the box's IP). A t.Cleanup restores the box's
// original hostname regardless of outcome. Run only against a disposable box.
func TestAccNetworkConfig_basic(t *testing.T) {
	if os.Getenv("TRUENAS_TEST_NETWORK_CONFIG") == "" {
		t.Skip("set TRUENAS_TEST_NETWORK_CONFIG=1 (on a disposable box) to run the network_config set/restore test")
	}
	acctest.PreCheck(t)

	orig := currentHostname(t)
	t.Cleanup(func() {
		if _, err := acctest.Client().Call(context.Background(), "network.configuration.update",
			map[string]any{"hostname": orig}); err != nil {
			t.Logf("WARNING: failed to restore hostname=%q: %v", orig, err)
		}
	})

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccNetworkConfigHostname("tftest-nc-a"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_network_config.test", "id", "network_config"),
					resource.TestCheckResourceAttr("truenas_network_config.test", "hostname", "tftest-nc-a"),
				),
			},
			// Update path: change the hostname in place.
			{
				Config: acctest.ProviderConfig() + testAccNetworkConfigHostname("tftest-nc-b"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_network_config.test", "hostname", "tftest-nc-b"),
				),
			},
			{
				ResourceName:      "truenas_network_config.test",
				ImportState:       true,
				ImportStateId:     "network_config",
				ImportStateVerify: true,
			},
		},
	})
}

func testAccNetworkConfigHostname(name string) string {
	return fmt.Sprintf(`
resource "truenas_network_config" "test" {
  hostname = %q
}
`, name)
}

// currentHostname reads the box's current network.configuration hostname.
func currentHostname(t *testing.T) string {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "network.configuration.config")
	if err != nil {
		t.Fatalf("reading network.configuration.config: %v", err)
	}
	var cfg struct {
		Hostname string `json:"hostname"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parsing network.configuration.config: %v", err)
	}
	return cfg.Hostname
}

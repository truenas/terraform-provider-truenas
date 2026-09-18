// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package network_config_test

import (
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
func TestAccNetworkConfig_basic(t *testing.T) {
	t.Skip("truenas_network_config controls the LIVE network configuration (hostname/DNS/gateways); skipped to avoid cutting off connectivity to the target box. See comment on TestAccNetworkConfig_basic for how to safely enable this against a disposable instance.")
}

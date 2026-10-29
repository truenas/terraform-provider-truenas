// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package network_config_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNetworkConfig_hostsWhitespace guards B-145: a hosts entry with
// trailing whitespace (TrueNAS trims it) or a domains entry with a space
// (TrueNAS splits it) is a plan-time error instead of an apply that fails
// with "inconsistent result". Validation stops the plan before anything is
// written, so this runs without TRUENAS_DISRUPTIVE.
func TestAccNetworkConfig_hostsWhitespace(t *testing.T) {
	acctest.PreCheck(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
resource "truenas_network_config" "test" {
  hosts = ["192.0.2.1 tfacca", "192.0.2.2 tfaccb "]
}
`,
				ExpectError: regexp.MustCompile(`no leading or trailing\s+whitespace`),
			},
			{
				Config: acctest.ProviderConfig() + `
resource "truenas_network_config" "test" {
  domains = ["a.example b.example"]
}
`,
				ExpectError: regexp.MustCompile(`single domain with\s+no\s+whitespace`),
			},
		},
	})
}

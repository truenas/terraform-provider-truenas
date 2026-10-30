// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package network_interface_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// Spare NIC on .70 only. Never the management NIC enp1s0f1.
const huntNIC = "enp1s0f0"

// TestHuntNetworkInterface_vlanLifecycle: VLAN with IPv4 + IPv6 aliases,
// then change tag, drop description, clear aliases, import.
func TestHuntNetworkInterface_vlanLifecycle(t *testing.T) {
	acctest.DisruptiveCheck(t)
	base := acctest.ProviderConfig()
	step1 := base + `
resource "truenas_network_interface" "test" {
  name                  = "vlan3901"
  type                  = "VLAN"
  vlan_parent_interface = "` + huntNIC + `"
  vlan_tag              = 3901
  description           = "tfhunt"
  mtu                   = 1400
  aliases = [
    { address = "198.51.100.20", netmask = 24 },
    { address = "198.51.100.10", netmask = 24 },
    { address = "2001:db8::10", netmask = 64 },
  ]
}
`
	step2 := base + `
resource "truenas_network_interface" "test" {
  name                  = "vlan3901"
  type                  = "VLAN"
  vlan_parent_interface = "` + huntNIC + `"
  vlan_tag              = 3902
  vlan_pcp              = 3
  aliases = [
    { address = "198.51.100.20", netmask = 24 },
  ]
}
`
	step3 := base + `
resource "truenas_network_interface" "test" {
  name                  = "vlan3901"
  type                  = "VLAN"
  vlan_parent_interface = "` + huntNIC + `"
  vlan_tag              = 3902
  aliases               = []
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: step1},
			{Config: step1, PlanOnly: true},
			{Config: step2},
			{Config: step2, PlanOnly: true},
			{Config: step3},
			{Config: step3, PlanOnly: true},
			{ResourceName: "truenas_network_interface.test", ImportState: true, ImportStateVerify: true},
		},
	})
}

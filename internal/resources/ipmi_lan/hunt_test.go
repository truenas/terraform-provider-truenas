// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ipmi_lan_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntIPMILan_dhcpReapply: on a DHCP BMC channel (.70 channel 1), an
// update that re-sends dhcp=true must not fail with an inconsistent ipaddress/
// netmask/gateway (those are Optional+Computed+UseStateForUnknown, but the BMC
// re-acquires its lease / flaps after every ipmi.lan.update).
// Only run against .70 (disposable BMC).
func TestHuntIPMILan_dhcpReapply(t *testing.T) {
	acctest.DisruptiveCheck(t)
	base := acctest.ProviderConfig()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: base + `
resource "truenas_ipmi_lan" "test" {
  channel = 1
  dhcp    = true
}
`},
			{Config: base + `
resource "truenas_ipmi_lan" "test" {
  channel      = 1
  dhcp         = true
  apply_remote = false
}
`},
			{Config: base + `
resource "truenas_ipmi_lan" "test" {
  channel      = 1
  dhcp         = true
  apply_remote = false
}
`, PlanOnly: true},
		},
	})
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package tunable_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntTunable_lifecycle: create, change value, drop comment from config,
// toggle enabled, import.
func TestHuntTunable_lifecycle(t *testing.T) {
	acctest.DisruptiveCheck(t)
	base := acctest.ProviderConfig()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: base + `
resource "truenas_tunable" "test" {
  var     = "net.ipv4.tcp_keepalive_probes"
  value   = "9"
  comment = "tfhunt"
}
`},
			{Config: base + `
resource "truenas_tunable" "test" {
  var     = "net.ipv4.tcp_keepalive_probes"
  value   = "8"
  comment = "tfhunt"
}
`},
			{Config: base + `
resource "truenas_tunable" "test" {
  var     = "net.ipv4.tcp_keepalive_probes"
  value   = "8"
  enabled = false
}
`},
			{Config: base + `
resource "truenas_tunable" "test" {
  var     = "net.ipv4.tcp_keepalive_probes"
  value   = "8"
  enabled = false
}
`, PlanOnly: true},
			{ResourceName: "truenas_tunable.test", ImportState: true, ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{"update_initramfs"}},
		},
	})
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package lxc_config_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// Values are the .70 originals (bridge null, fast, 10.236.1.0/24,
// fd42:1538:707f:15a2::/64) spelled differently; the last test restores them.
// TestHuntLXCConfig_bridgeAuto was promoted to bridge_auto_acc_test.go
// (TestAccLXCConfig_bridgeAuto).

func TestHuntLXCConfig_v6Case(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: acctest.ProviderConfig() + `
resource "truenas_lxc_config" "test" {
  v6_network = "FD42:1538:707F:15A2::/64"
}
`},
		},
	})
}

func TestHuntLXCConfig_restore(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: acctest.ProviderConfig() + `
resource "truenas_lxc_config" "test" {
  preferred_pool = "fast"
  v4_network     = "10.236.1.0/24"
  v6_network     = "fd42:1538:707f:15a2::/64"
}
`},
		},
	})
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package catalog_config_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// Restores .70's original order ["stable","test","enterprise","community","dev"] in the last step.
func TestHuntCatalogConfig_lifecycle(t *testing.T) {
	orig := `
resource "truenas_catalog_config" "test" {
  preferred_trains = ["stable", "test", "enterprise", "community", "dev"]
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: acctest.ProviderConfig() + `
resource "truenas_catalog_config" "test" {}
`},
			{Config: acctest.ProviderConfig() + `
resource "truenas_catalog_config" "test" {
  preferred_trains = ["community", "stable", "stable"]
}
`},
			{Config: acctest.ProviderConfig() + `
resource "truenas_catalog_config" "test" {}
`},
			{Config: acctest.ProviderConfig() + orig},
			{Config: acctest.ProviderConfig() + orig, ResourceName: "truenas_catalog_config.test", ImportState: true, ImportStateVerify: true},
			{Config: acctest.ProviderConfig() + orig + `
data "truenas_catalog_config" "d" {
  depends_on = [truenas_catalog_config.test]
}
`, Check: resource.TestCheckResourceAttrPair("data.truenas_catalog_config.d", "preferred_trains.#", "truenas_catalog_config.test", "preferred_trains.#")},
		},
	})
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package resilver_config_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntResilver_weekdayOrder: unsorted / duplicate weekday list.
func TestHuntResilver_weekdayOrder(t *testing.T) {
	acctest.PreCheck(t)
	acctest.DisruptiveCheck(t)
	resilverRestore(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: acctest.ProviderConfig() + `
resource "truenas_resilver_config" "test" {
  weekday = [7, 1, 3]
}
`},
			{
				ResourceName:                         "truenas_resilver_config.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        "resilver_config",
				ImportStateVerifyIdentifierAttribute: "id",
			},
		},
	})
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package webshare_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntWebshare_allFields is an adversarial value round-trip for
// truenas_webshare: it sets every settable attribute to a non-default value
// at once — path (to a dataset mountpoint fixture), name, enabled = false
// (non-default; the API default is true), and is_home_base = true
// (non-default; the API default is false) — then asserts the apply, that a
// re-plan is empty, and that import round-trips.
//
// Requires TrueNAS 26.0+: the sharing.webshare namespace does not exist on
// earlier releases, so this self-skips on an older box exactly as the
// acceptance test does.
//
// The computed read-back fields (dataset, relative_path, locked) have a
// documented create-time null quirk — they can read back null immediately
// after create and settle on a later read. The empty re-plan step refreshes
// before import, so they are not added to ImportStateVerifyIgnore here
// (matching the acceptance test, which imports without ignoring them).
func TestHuntWebshare_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("truenas_webshare requires TrueNAS 26.0 or later (sharing.webshare namespace absent on 25.10, confirmed live)")
	}

	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfhunt-ds-webshare"))
	shareName := acctest.RandName("tfhunt-webshare")

	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name = %q
}

resource "truenas_webshare" "test" {
  path         = truenas_dataset.test.mountpoint
  name         = %q
  enabled      = false
  is_home_base = true
}
`, datasetName, shareName)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckWebshareDestroyed(shareName),
			testAccCheckWebshareDatasetDestroyed(datasetName),
		),
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_webshare.test", "id"),
					resource.TestCheckResourceAttr("truenas_webshare.test", "name", shareName),
					resource.TestCheckResourceAttr("truenas_webshare.test", "enabled", "false"),
					resource.TestCheckResourceAttr("truenas_webshare.test", "is_home_base", "true"),
					resource.TestCheckResourceAttrSet("truenas_webshare.test", "path"),
				),
			},
			{
				Config:           cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				ResourceName:      "truenas_webshare.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

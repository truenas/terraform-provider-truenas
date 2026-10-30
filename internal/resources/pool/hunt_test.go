// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package pool_test

// Adversarial bug-hunt test: build a pool that sets every API-accepted,
// user-writable attribute to a non-default value at once (full topology plus
// autotrim and the root-dataset properties), assert the read-back re-plans
// empty, then import. The provider never plans a pool destroy/recreate, so
// the resource is created, verified, and torn down only by the framework's
// own CheckDestroy at the end.

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntPool_allFields creates a pool exercising data(MIRROR) + log + cache
// + spare topology, autotrim, deduplication, and checksum together — every
// field pool.create/pool.update and the root-dataset property path accept.
// Gated on TRUENAS_TEST_DISKS (needs 5 dedicated blank disks), reusing the
// acceptance suite's testDisks picker and testAccCheckPoolDestroyed teardown.
func TestHuntPool_allFields(t *testing.T) {
	disks := testDisks(t, 5) // t.Skip unless TRUENAS_TEST_DISKS lists >= 5 blank disks
	acctest.PreCheck(t)
	name := acctest.RandName("tfhuntpool")

	c := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_pool" "test" {
  name = %q
  topology = {
    data  = [{ type = "MIRROR", disks = [%q, %q] }]
    log   = [{ type = "STRIPE", disks = [%q] }]
    cache = [%q]
    spare = [%q]
  }
  autotrim      = true
  deduplication = "VERIFY"
  checksum      = "SHA256"
}
`, name, disks[0], disks[1], disks[2], disks[3], disks[4])

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPoolDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: c,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_pool.test", "name", name),
					resource.TestCheckResourceAttr("truenas_pool.test", "topology.data.0.type", "MIRROR"),
					resource.TestCheckResourceAttr("truenas_pool.test", "autotrim", "true"),
					resource.TestCheckResourceAttr("truenas_pool.test", "deduplication", "VERIFY"),
					resource.TestCheckResourceAttr("truenas_pool.test", "checksum", "SHA256"),
					resource.TestCheckResourceAttr("truenas_pool.test", "healthy", "true"),
				),
			},
			{
				Config:           c,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				ResourceName:            "truenas_pool.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"allocated", "free"},
			},
		},
	})
}

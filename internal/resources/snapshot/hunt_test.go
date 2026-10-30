// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snapshot_test

// Adversarial bug-hunt tests: take a recursive snapshot of a fresh dataset
// tree, assert an empty re-plan, then import. Terraform destroys the snapshot
// and fixture datasets on cleanup.

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// Recursive snapshot over a parent + child dataset. Assert the read-back
// re-plans empty, then import by dataset@name.
func TestHuntSnapshot_recursive(t *testing.T) {
	acctest.DisruptiveCheck(t)
	parent := acctest.TestPool() + "/tfhuntsnap"
	child := parent + "/child"
	c := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "parent" {
  name = %q
}

resource "truenas_dataset" "child" {
  name       = %q
  depends_on = [truenas_dataset.parent]
}

resource "truenas_snapshot" "test" {
  dataset   = truenas_dataset.parent.name
  name      = "tfhuntsnap1"
  recursive = true
  depends_on = [truenas_dataset.child]
}
`, parent, child)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c},
			{Config: c, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:            "truenas_snapshot.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"recursive"},
			},
		},
	})
}

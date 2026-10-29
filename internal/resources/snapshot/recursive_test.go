// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snapshot_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func snapshotRecursiveConfig(ds, snap string, recursive bool) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "parent" {
  name = %q
}
resource "truenas_dataset" "child" {
  name = "${truenas_dataset.parent.name}/child"
}
resource "truenas_snapshot" "test" {
  dataset    = truenas_dataset.parent.name
  name       = %q
  recursive  = %t
  depends_on = [truenas_dataset.child]
}
`, ds, snap, recursive)
}

// S1: import a snapshot whose config sets recursive (write-only, RequiresReplace):
// null after import -> plan destroys and re-creates the snapshot (its point-in-
// time content is lost).
func TestAccSnapshot_importRecursive(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfhunt-snap"))
	snap := "tfhunt-s1"
	cfg := snapshotRecursiveConfig(ds, snap, true)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "parent" {
  name = %q
}
resource "truenas_dataset" "child" {
  name = "${truenas_dataset.parent.name}/child"
}
removed {
  from = truenas_snapshot.test
  lifecycle {
    destroy = false
  }
}
`, ds)},
			{
				Config: cfg + fmt.Sprintf("\nimport {\n  to = truenas_snapshot.test\n  id = %q\n}\n", ds+"@"+snap),
				// recursive is never returned, so it reads back null and is recorded
				// in place (fix C1); the snapshot must not be replaced.
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					acctest.ExpectNoReplace("truenas_snapshot.test"),
					acctest.ExpectOnlyAttrChanges("truenas_snapshot.test", "recursive"),
				}},
			},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

// S1b: same with recursive = false spelled out explicitly.
func TestAccSnapshot_importRecursiveFalse(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfhunt-snapf"))
	snap := "tfhunt-s1f"
	cfg := snapshotRecursiveConfig(ds, snap, false)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "parent" {
  name = %q
}
resource "truenas_dataset" "child" {
  name = "${truenas_dataset.parent.name}/child"
}
removed {
  from = truenas_snapshot.test
  lifecycle {
    destroy = false
  }
}
`, ds)},
			{
				Config: cfg + fmt.Sprintf("\nimport {\n  to = truenas_snapshot.test\n  id = %q\n}\n", ds+"@"+snap),
				// recursive is never returned, so it reads back null and is recorded
				// in place (fix C1); the snapshot must not be replaced.
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					acctest.ExpectNoReplace("truenas_snapshot.test"),
					acctest.ExpectOnlyAttrChanges("truenas_snapshot.test", "recursive"),
				}},
			},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

// S2: destroying a recursive snapshot deletes only parent@snap; child@snap
// created by the same recursive create is left behind.
func TestAccSnapshot_recursiveDestroyLeavesChildren(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfhunt-snapr"))
	snap := "tfhunt-s2"
	childSnap := ds + "/child@" + snap
	cfgBoth := snapshotRecursiveConfig(ds, snap, true)
	// Remove only the snapshot resource, keep the datasets.
	cfgNoSnap := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "parent" {
  name = %q
}
resource "truenas_dataset" "child" {
  name = "${truenas_dataset.parent.name}/child"
}
`, ds)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfgBoth},
			{
				Config: cfgNoSnap,
				Check: func(_ *terraform.State) error {
					raw, err := acctest.Client().Call(context.Background(), "pool.snapshot.query",
						[][]any{{"id", "=", childSnap}})
					if err != nil {
						return err
					}
					var res []map[string]any
					_ = json.Unmarshal(raw, &res)
					if len(res) > 0 {
						return fmt.Errorf("recursive snapshot destroyed but child snapshot %s still exists", childSnap)
					}
					return nil
				},
			},
		},
	})
}

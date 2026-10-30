// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol_test

// Hunt finding F8: renaming a zvol in config is a replacement (the resource
// identity is the name and may not change on an update), and the destroy
// half used to wipe the volume. A zvol that holds data is now refused unless
// force_destroy is set (ZvolHoldsData, logicalreferenced above the empty
// baseline). Blocks cannot be written through the API from a test, so the
// data path is proven by scratchpad/upgrade/repro-rename-zvol.sh (dd a
// signature, rename, expect refusal and the signature intact); this test
// covers the snapshot-protected rename and the force_destroy escape.

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestAccZvol_renameRefusedWithSnapshot(t *testing.T) {
	acctest.PreCheck(t)
	base := acctest.TestPool() + "/" + acctest.RandName("tfaccrenz")
	renamed := base + "-renamed"
	cfg := func(name string, force bool) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "z" {
  name          = %q
  volsize       = 16777216
  sparse        = true
  force_destroy = %t
}
`, name, force)
	}
	c := acctest.Client()
	ctx := context.Background()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg(base, false), Check: func(*terraform.State) error {
				_, err := c.Call(ctx, "pool.snapshot.create", map[string]any{"dataset": base, "name": "keep"})
				return err
			}},
			// rename = replace; the destroy half is refused (snapshot), nothing lost
			{
				Config: cfg(renamed, false),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_zvol.z", plancheck.ResourceActionReplace),
				}},
				ExpectError: regexp.MustCompile(`snapshot|holds data`),
			},
			{Config: cfg(base, false), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				Check: func(*terraform.State) error {
					raw, err := c.Call(ctx, "pool.snapshot.query", []any{[]any{"name", "=", base + "@keep"}})
					if err != nil {
						return err
					}
					if string(raw) == "[]" {
						return fmt.Errorf("snapshot %s@keep is gone: the refused rename destroyed the zvol", base)
					}
					return nil
				}},
			{Config: cfg(base, true)},
		},
	})
}

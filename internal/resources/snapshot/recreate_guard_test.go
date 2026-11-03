// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snapshot_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccSnapshot_recreateRefused: renaming a snapshot in config would destroy
// the point-in-time copy and take a new one. Refused at plan time by default;
// allowed with allow_recreate = true.
func TestAccSnapshot_recreateRefused(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-snapguard"))
	cfg := func(snap string, allow bool) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name          = %q
  force_destroy = true # the snapshots below
}

resource "truenas_snapshot" "test" {
  dataset        = truenas_dataset.fixture.name
  name           = %q
  allow_recreate = %v
}
`, ds, snap, allow)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg("first", false), Check: resource.TestCheckResourceAttr("truenas_snapshot.test", "id", ds+"@first")},
			// rename -> replace -> refused
			{Config: cfg("second", false), ExpectError: regexp.MustCompile(`Snapshot replacement refused`)},
			// the original is untouched
			{Config: cfg("first", false), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			// opt in: the flag alone is an in-place update
			{Config: cfg("first", true), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
				plancheck.ExpectResourceAction("truenas_snapshot.test", plancheck.ResourceActionUpdate)}}},
			// now the rename replaces
			{Config: cfg("second", true), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
				plancheck.ExpectResourceAction("truenas_snapshot.test", plancheck.ResourceActionReplace)}},
				Check: resource.TestCheckResourceAttr("truenas_snapshot.test", "id", ds+"@second")},
		},
	})
}

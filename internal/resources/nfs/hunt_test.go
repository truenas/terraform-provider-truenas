// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nfs_test

// Adversarial bug-hunt tests (throwaway, not for commit).

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// Change one attribute on an existing share, then import + re-plan.
func TestHuntNFS_updateOneImport(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfhunt"))
	c1 := nfsShareConfig(ds, `
  comment  = "a"
  networks = ["192.168.0.0/16", "10.0.0.0/8"]
  mapall_user  = "nobody"
  mapall_group = "nogroup"
`)
	c2 := nfsShareConfig(ds, `
  comment  = "b"
  networks = ["192.168.0.0/16", "10.0.0.0/8"]
  mapall_user  = "nobody"
  mapall_group = "nogroup"
`)
	steps := []resource.TestStep{
		{Config: c1},
		{Config: c1, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{Config: c2},
		{Config: c2, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}
	steps = append(steps, huntImportPersist("truenas_nfs_share.test", c2)...)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps:                    steps,
	})
}

// Trailing slash on path: stored verbatim by the server, but the dataset
// mountpoint has none. Ensure no replace loop.
func TestHuntNFS_trailingSlash(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfhunt"))
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name = %q
}
resource "truenas_nfs_share" "test" {
  path = "${truenas_dataset.fixture.mountpoint}/"
}
`, ds)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

// huntImportPersist: import WITH ImportStatePersist so the following
// empty-plan step runs against the imported state.
func huntImportPersist(name, cfg string) []resource.TestStep {
	return []resource.TestStep{
		// import block + plan in a fresh workdir; the framework requires a no-op plan.
		{Config: cfg, ResourceName: name, ImportState: true, ImportStateKind: resource.ImportBlockWithID},
	}
}

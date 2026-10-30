// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_test

// Adversarial bug-hunt tests (throwaway, not for commit).

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// Typed options partial on TIME_LOCKED_SHARE, update one option, import.
func TestHuntSMB_timeLockedOptions(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfhunt"))
	name := acctest.RandName("tfhunt")
	c1 := smbShareConfig(ds, name, `
  purpose = "TIME_LOCKED_SHARE"
  options = { grace_period = 600 }
`)
	c2 := smbShareConfig(ds, name, `
  purpose = "TIME_LOCKED_SHARE"
  options = { grace_period = 1200, hostsallow = ["10.0.0.0/8"] }
`)
	steps := []resource.TestStep{
		{Config: c1},
		{Config: c1, ConfigPlanChecks: expectEmptyPlan()},
		{Config: c2},
		{Config: c2, ConfigPlanChecks: expectEmptyPlan()},
	}
	steps = append(steps, huntImportPersist("truenas_smb_share.test", c2)...)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps:                    steps,
	})
}

// Change only `name` on an existing default (LEGACY) share, then import.
func TestHuntSMB_renameImport(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfhunt"))
	c1 := smbShareConfig(ds, acctest.RandName("tfhunt"), `  comment = "x"`)
	c2 := smbShareConfig(ds, acctest.RandName("tfhunt"), `  comment = "x"`)
	steps := []resource.TestStep{
		{Config: c1},
		{Config: c2},
		{Config: c2, ConfigPlanChecks: expectEmptyPlan()},
	}
	steps = append(steps, huntImportPersist("truenas_smb_share.test", c2)...)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps:                    steps,
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

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_dataset_test

// Adversarial bug-hunt test: set every API-accepted system_dataset attribute
// (pool and the write-only pool_exclude) to a non-default value, assert the
// read-back re-plans empty, then import. Moving the system dataset is a
// disruptive, long-running migration, so this mirrors the acceptance suite's
// double-gate (TRUENAS_TEST_SYSTEM_DATASET + TRUENAS_TEST_DISKS), builds a
// throwaway pool, moves the system dataset onto it, and a t.Cleanup returns
// the system dataset to its original pool and destroys the throwaway pool.

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntSystemDataset_allFields moves the system dataset to a throwaway pool
// while also exercising the write-only pool_exclude field, then asserts an
// empty re-plan and imports.
func TestHuntSystemDataset_allFields(t *testing.T) {
	if os.Getenv("TRUENAS_TEST_SYSTEM_DATASET") == "" {
		t.Skip("set TRUENAS_TEST_SYSTEM_DATASET=1 (on a disposable box) to run the system_dataset all-fields hunt test")
	}
	disksEnv := os.Getenv("TRUENAS_TEST_DISKS")
	if disksEnv == "" {
		t.Skip("set TRUENAS_TEST_DISKS (a spare disk) for the throwaway pool")
	}
	disk := strings.TrimSpace(strings.Split(disksEnv, ",")[0])
	acctest.PreCheck(t)

	c := acctest.Client()
	ctx := context.Background()
	orig := systemDatasetPool(t)
	const testPool = "sysdshunt"

	if _, err := c.CallJob(ctx, "pool.create", map[string]any{
		"name": testPool,
		"topology": map[string]any{
			"data": []map[string]any{{"type": "STRIPE", "disks": []string{disk}}},
		},
	}); err != nil {
		t.Fatalf("create throwaway pool %q: %v", testPool, err)
	}

	t.Cleanup(func() {
		// The system dataset must be off the throwaway pool before it can be destroyed.
		if p := systemDatasetPoolNoFatal(c); p != "" && p != orig {
			if _, err := c.CallJob(ctx, "systemdataset.update", map[string]any{"pool": orig}); err != nil {
				t.Logf("WARNING: failed to move system dataset back to %q: %v", orig, err)
			}
		}
		id, err := poolIDByName(c, testPool)
		if err != nil {
			t.Logf("WARNING: could not find throwaway pool %q to destroy: %v", testPool, err)
			return
		}
		if _, err := c.CallJob(ctx, "pool.export", id, map[string]any{"cascade": true, "destroy": true}); err != nil {
			t.Logf("WARNING: failed to destroy throwaway pool %q: %v", testPool, err)
		}
	})

	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_system_dataset" "test" {
  pool         = %q
  pool_exclude = %q
}
`, testPool, orig)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_system_dataset.test", "id", "system_dataset"),
					resource.TestCheckResourceAttr("truenas_system_dataset.test", "pool", testPool),
				),
			},
			{
				Config:           cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				ResourceName:            "truenas_system_dataset.test",
				ImportState:             true,
				ImportStateId:           "system_dataset",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"pool_exclude"},
			},
		},
	})
}

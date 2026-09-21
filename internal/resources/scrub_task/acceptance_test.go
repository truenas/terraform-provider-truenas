// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package scrub_task_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// testPoolID resolves acctest.TestPool()'s name to its numeric pool id via
// pool.query, since pool.scrub.create/update/query key on the pool id, not
// its name.
func testPoolID(t *testing.T) int64 {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "pool.query", [][]any{{"name", "=", acctest.TestPool()}})
	if err != nil {
		t.Fatalf("error querying pool %s: %v", acctest.TestPool(), err)
	}
	var results []struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(raw, &results); err != nil {
		t.Fatalf("error parsing pool.query response: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("pool %s not found", acctest.TestPool())
	}
	return results[0].ID
}

// existingScrubTaskID returns the id of a pre-existing pool.scrub schedule
// for poolID, if any, and 0 otherwise.
func existingScrubTaskID(t *testing.T, poolID int64) int64 {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "pool.scrub.query", [][]any{{"pool", "=", poolID}})
	if err != nil {
		t.Fatalf("error querying scrub tasks for pool %d: %v", poolID, err)
	}
	var results []struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(raw, &results); err != nil {
		t.Fatalf("error parsing pool.scrub.query response: %v", err)
	}
	if len(results) == 0 {
		return 0
	}
	return results[0].ID
}

// TestAccScrubTask_basic creates a scrub schedule on the test pool, checks
// its attributes, updates threshold in place, imports it by id, and
// verifies destruction via a live pool.scrub.query.
//
// TrueNAS allows at most one scrub schedule per pool (verified by probe:
// pool.scrub.create on a pool that already has one returns "[EINVAL]
// pool_scrub_create.pool: A scrub with this pool already exists"). The test
// pool typically ships with a default scrub schedule already present, in
// which case this test cannot create its own without deleting that default
// — so it SKIPs rather than mutating a schedule it doesn't own.
func TestAccScrubTask_basic(t *testing.T) {
	acctest.PreCheck(t)

	poolID := testPoolID(t)
	if existing := existingScrubTaskID(t, poolID); existing != 0 {
		t.Skipf("pool %s (id %d) already has a scrub schedule (id %d); TrueNAS allows only one scrub "+
			"schedule per pool, so this test cannot create its own without deleting the existing one", acctest.TestPool(), poolID, existing)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckScrubTaskDestroyed(poolID),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccScrubTaskConfig(poolID, 30, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_scrub_task.test", "pool", fmt.Sprintf("%d", poolID)),
					resource.TestCheckResourceAttr("truenas_scrub_task.test", "pool_name", acctest.TestPool()),
					resource.TestCheckResourceAttr("truenas_scrub_task.test", "threshold", "30"),
					resource.TestCheckResourceAttr("truenas_scrub_task.test", "enabled", "true"),
					resource.TestCheckResourceAttr("truenas_scrub_task.test", "schedule.minute", "0"),
					resource.TestCheckResourceAttr("truenas_scrub_task.test", "schedule.hour", "2"),
					resource.TestCheckResourceAttrSet("truenas_scrub_task.test", "id"),
				),
			},
			// Update in place: change threshold.
			{
				Config: acctest.ProviderConfig() + testAccScrubTaskConfig(poolID, 45, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_scrub_task.test", "threshold", "45"),
				),
			},
			// Import by the task's numeric id.
			{
				ResourceName:      "truenas_scrub_task.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccScrubTaskConfig(poolID int64, threshold int, enabled bool) string {
	return fmt.Sprintf(`
resource "truenas_scrub_task" "test" {
  pool        = %d
  threshold   = %d
  description = "tf-acc scrub task"
  enabled     = %v
  schedule = {
    minute = "0"
    hour   = "2"
    dom    = "*"
    month  = "*"
    dow    = "7"
  }
}
`, poolID, threshold, enabled)
}

func testAccCheckScrubTaskDestroyed(poolID int64) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "pool.scrub.query", [][]any{{"pool", "=", poolID}})
		if err != nil {
			return fmt.Errorf("error checking scrub task for pool %d: %v", poolID, err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing pool.scrub.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("scrub task for pool %d still exists", poolID)
		}
		return nil
	}
}

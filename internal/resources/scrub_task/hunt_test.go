// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package scrub_task_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntScrub_lifecycle deletes the test pool's built-in scrub schedule so
// the resource's create path can actually run (the acceptance test always
// skips because every pool ships with one), then exercises create with
// defaults, update, unset, import. Cleanup recreates the original schedule.
func TestHuntScrub_lifecycle(t *testing.T) {
	acctest.PreCheck(t)
	acctest.DisruptiveCheck(t)
	ctx := context.Background()
	poolID := testPoolID(t)
	var orig map[string]any
	if id := existingScrubTaskID(t, poolID); id != 0 {
		raw, err := acctest.Client().Call(ctx, "pool.scrub.get_instance", id)
		if err != nil {
			t.Fatal(err)
		}
		_ = json.Unmarshal(raw, &orig)
		if _, err := acctest.Client().Call(ctx, "pool.scrub.delete", id); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		if orig == nil || existingScrubTaskID(t, poolID) != 0 {
			return
		}
		p := map[string]any{"pool": poolID, "threshold": orig["threshold"], "schedule": orig["schedule"], "enabled": orig["enabled"], "description": orig["description"]}
		if _, err := acctest.RestoreCall(context.Background(), "pool.scrub.create", p); err != nil {
			t.Errorf("restore scrub: %v", err)
		}
	})

	cfg := func(body string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_scrub_task" "test" {
  pool = %d
%s
}
`, poolID, body)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg(""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_scrub_task.test", "threshold", "35"),
					resource.TestCheckResourceAttr("truenas_scrub_task.test", "enabled", "true"),
					resource.TestCheckResourceAttr("truenas_scrub_task.test", "schedule.dow", "7"),
					resource.TestCheckResourceAttr("truenas_scrub_task.test", "pool_name", acctest.TestPool()),
				),
			},
			{
				Config: cfg(`  threshold = 0
  description = "tfhunt"
  enabled = false
  schedule = { minute = "*/30", hour = "1-3", dom = "1,15", month = "jan-mar", dow = "sat,sun" }`),
			},
			{
				Config: cfg(`  threshold = 0`),
			},
			{
				ResourceName:      "truenas_scrub_task.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

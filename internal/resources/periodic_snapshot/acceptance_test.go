// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package periodic_snapshot_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccPeriodicSnapshot_basic creates a dataset fixture and a periodic
// snapshot task on it, checks its attributes, updates the schedule minute
// and lifetime_value in place, imports the task by its numeric id, and
// verifies destruction of both the task and the dataset fixture.
func TestAccPeriodicSnapshot_basic(t *testing.T) {
	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-pst"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckPeriodicSnapshotDestroyed(datasetName),
			testAccCheckPSTDatasetDestroyed(datasetName),
		),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccPeriodicSnapshotConfig(datasetName, "0", 2, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "dataset", datasetName),
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "enabled", "true"),
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "lifetime_value", "2"),
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "lifetime_unit", "WEEK"),
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "naming_schema", "tf-acc-%Y%m%d-%H%M"),
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "schedule.minute", "0"),
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "schedule.hour", "0"),
					resource.TestCheckResourceAttrSet("truenas_periodic_snapshot_task.test", "id"),
				),
			},
			// Update in place: change schedule minute and lifetime_value.
			{
				Config: acctest.ProviderConfig() + testAccPeriodicSnapshotConfig(datasetName, "15", 3, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "schedule.minute", "15"),
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "lifetime_value", "3"),
				),
			},
			// Import by the task's numeric id.
			{
				ResourceName:      "truenas_periodic_snapshot_task.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccPeriodicSnapshotConfig(datasetName, minute string, lifetimeValue int, enabled bool) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name = %q
}

resource "truenas_periodic_snapshot_task" "test" {
  dataset        = truenas_dataset.test.name
  recursive      = false
  lifetime_value = %d
  lifetime_unit  = "WEEK"
  naming_schema  = "tf-acc-%%Y%%m%%d-%%H%%M"
  enabled        = %v
  schedule = {
    minute = %q
    hour   = "0"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}
`, datasetName, lifetimeValue, enabled, minute)
}

func testAccCheckPeriodicSnapshotDestroyed(datasetName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "pool.snapshottask.query", [][]any{{"dataset", "=", datasetName}})
		if err != nil {
			return fmt.Errorf("error checking periodic snapshot task for dataset %s: %v", datasetName, err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing pool.snapshottask.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("periodic snapshot task for dataset %s still exists", datasetName)
		}
		return nil
	}
}

func testAccCheckPSTDatasetDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "pool.dataset.query", [][]any{{"id", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking dataset %s: %v", name, err)
		}
		var results []struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing pool.dataset.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("dataset %s still exists", name)
		}
		return nil
	}
}

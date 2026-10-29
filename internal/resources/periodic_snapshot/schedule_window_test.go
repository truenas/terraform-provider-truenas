// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package periodic_snapshot_test

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// pstMinimalConfig is a periodic snapshot task with every Optional attribute
// left unset; window is spliced into the schedule block.
func pstMinimalConfig(ds string, lifetime int, window string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name = %q
}

resource "truenas_periodic_snapshot_task" "test" {
  dataset        = truenas_dataset.fixture.name
  lifetime_value = %d
  lifetime_unit  = "WEEK"
  naming_schema  = "tf-acc-%%Y%%m%%d-%%H%%M"
  schedule = {
    minute = "0"
    hour   = "0"
    dom    = "*"
    month  = "*"
    dow    = "*"
%s
  }
}
`, ds, lifetime, window)
}

// pstServerSchedule reads the task's schedule straight from the server.
func pstServerSchedule(s *terraform.State) (map[string]string, error) {
	id, err := strconv.ParseInt(s.RootModule().Resources["truenas_periodic_snapshot_task.test"].Primary.ID, 10, 64)
	if err != nil {
		return nil, err
	}
	raw, err := acctest.Client().Call(context.Background(), "pool.snapshottask.get_instance", id)
	if err != nil {
		return nil, err
	}
	var got struct {
		Schedule map[string]string `json:"schedule"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		return nil, err
	}
	return got.Schedule, nil
}

func pstCheckServerWindow(begin, end string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		sched, err := pstServerSchedule(s)
		if err != nil {
			return err
		}
		if sched["begin"] != begin || sched["end"] != end {
			return fmt.Errorf("server schedule window = %q-%q, want %q-%q", sched["begin"], sched["end"], begin, end)
		}
		return nil
	}
}

// TestAccPeriodicSnapshot_unsetEnabledAllowEmpty: enabled and allow_empty
// left unset must take the server defaults (true). The provider used to send
// ValueBool() of the unknown plan value, false, so the task was created
// disabled and never took a snapshot, with a clean plan (B-010).
func TestAccPeriodicSnapshot_unsetEnabledAllowEmpty(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-pst"))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + pstMinimalConfig(ds, 2, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "enabled", "true"),
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "allow_empty", "true"),
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "recursive", "false"),
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "exclude.#", "0"),
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "schedule.begin", "00:00"),
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "schedule.end", "23:59"),
				),
			},
			// An unrelated update keeps the server values.
			{
				Config: acctest.ProviderConfig() + pstMinimalConfig(ds, 3, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "lifetime_value", "3"),
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "enabled", "true"),
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "allow_empty", "true"),
				),
			},
		},
	})
}

// TestAccPeriodicSnapshot_beginEndKept: the schedule window (begin/end) set
// out of band (e.g. in the UI) must survive an unrelated Terraform update.
// begin/end were not modeled and the server replaces the whole schedule, so
// every update reset the window to 00:00-23:59 (B-036). The window is now
// managed too: set it, then remove it from config (kept).
func TestAccPeriodicSnapshot_beginEndKept(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-pst"))
	var id string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + pstMinimalConfig(ds, 2, ""),
				Check: func(s *terraform.State) error {
					id = s.RootModule().Resources["truenas_periodic_snapshot_task.test"].Primary.ID
					return nil
				},
			},
			// An unpadded time is zero-padded by the server; rejected at plan
			// time. (Not the last step: post-test destroy uses the last config.)
			{
				Config:      acctest.ProviderConfig() + pstMinimalConfig(ds, 2, `    begin = "9:30"`),
				ExpectError: regexp.MustCompile(`leading\s+zero`),
			},
			{
				PreConfig: func() {
					n, _ := strconv.ParseInt(id, 10, 64)
					_, err := acctest.Client().Call(context.Background(), "pool.snapshottask.update", n,
						map[string]any{"schedule": map[string]any{"minute": "0", "hour": "0", "dom": "*", "month": "*", "dow": "*", "begin": "08:00", "end": "17:00"}})
					if err != nil {
						t.Fatalf("out-of-band update: %v", err)
					}
				},
				Config: acctest.ProviderConfig() + pstMinimalConfig(ds, 3, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					pstCheckServerWindow("08:00", "17:00"),
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "lifetime_value", "3"),
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "schedule.begin", "08:00"),
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "schedule.end", "17:00"),
				),
			},
			{
				Config: acctest.ProviderConfig() + pstMinimalConfig(ds, 3, `    begin = "09:30"
    end   = "18:00"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					pstCheckServerWindow("09:30", "18:00"),
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "schedule.begin", "09:30"),
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "schedule.end", "18:00"),
				),
			},
			{
				Config: acctest.ProviderConfig() + pstMinimalConfig(ds, 4, ""),
				Check:  pstCheckServerWindow("09:30", "18:00"),
			},
			{
				ResourceName:      "truenas_periodic_snapshot_task.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccPeriodicSnapshot_dataSource reads a task through the
// truenas_periodic_snapshot_task data source. Every read used to fail with a
// Value Conversion Error on schedule (B-017).
func TestAccPeriodicSnapshot_dataSource(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-pst"))
	cfg := acctest.ProviderConfig() + pstMinimalConfig(ds, 2, `    begin = "08:00"
    end   = "17:00"`) + `
data "truenas_periodic_snapshot_task" "d" {
  dataset = truenas_periodic_snapshot_task.test.dataset
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: cfg,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrPair("data.truenas_periodic_snapshot_task.d", "id", "truenas_periodic_snapshot_task.test", "id"),
				resource.TestCheckResourceAttr("data.truenas_periodic_snapshot_task.d", "lifetime_value", "2"),
				resource.TestCheckResourceAttr("data.truenas_periodic_snapshot_task.d", "lifetime_unit", "WEEK"),
				resource.TestCheckResourceAttr("data.truenas_periodic_snapshot_task.d", "schedule.minute", "0"),
				resource.TestCheckResourceAttr("data.truenas_periodic_snapshot_task.d", "schedule.begin", "08:00"),
				resource.TestCheckResourceAttr("data.truenas_periodic_snapshot_task.d", "schedule.end", "17:00"),
				resource.TestCheckResourceAttr("data.truenas_periodic_snapshot_task.d", "enabled", "true"),
			),
		}},
	})
}

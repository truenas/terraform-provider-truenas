// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package replication_test

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

const replAddr = "truenas_replication_task.test"

// replConfig is a LOCAL PUSH task; body is spliced into the resource block.
// Two disabled periodic snapshot tasks are available as feed_a / feed_b.
func replConfig(name, body string) string {
	pool := acctest.TestPool()
	return fmt.Sprintf(`
resource "truenas_dataset" "src" {
  name = "%[1]s/%[2]s-src"
}

resource "truenas_dataset" "dst" {
  name = "%[1]s/%[2]s-dst"
}

resource "truenas_periodic_snapshot_task" "feed_a" {
  dataset        = truenas_dataset.src.name
  lifetime_value = 1
  lifetime_unit  = "DAY"
  naming_schema  = "tfacca-%%Y%%m%%d-%%H%%M"
  enabled        = false
  allow_empty    = true
  schedule = { minute = "0", hour = "0", dom = "*", month = "*", dow = "*" }
}

resource "truenas_periodic_snapshot_task" "feed_b" {
  dataset        = truenas_dataset.src.name
  lifetime_value = 1
  lifetime_unit  = "DAY"
  naming_schema  = "tfaccb-%%Y%%m%%d-%%H%%M"
  enabled        = false
  allow_empty    = true
  schedule = { minute = "0", hour = "0", dom = "*", month = "*", dow = "*" }
}

resource "truenas_replication_task" "test" {
  name            = %[2]q
  direction       = "PUSH"
  transport       = "LOCAL"
  source_datasets = [truenas_dataset.src.name]
  target_dataset  = truenas_dataset.dst.name
  recursive       = false
  enabled         = false
%[3]s
}
`, pool, name, body)
}

// replServer reads the task straight from the server.
func replServer(s *terraform.State) (map[string]any, error) {
	id, err := strconv.ParseInt(s.RootModule().Resources[replAddr].Primary.ID, 10, 64)
	if err != nil {
		return nil, err
	}
	raw, err := acctest.Client().Call(context.Background(), "replication.get_instance", id)
	if err != nil {
		return nil, err
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		return nil, err
	}
	return got, nil
}

// replCheckServer asserts on the server's copy of the task.
func replCheckServer(check func(map[string]any) error) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		got, err := replServer(s)
		if err != nil {
			return err
		}
		return check(got)
	}
}

func replCheckServerWindow(key, begin, end string) resource.TestCheckFunc {
	return replCheckServer(func(got map[string]any) error {
		sched, _ := got[key].(map[string]any)
		if sched["begin"] != begin || sched["end"] != end {
			return fmt.Errorf("server %s window = %v-%v, want %s-%s", key, sched["begin"], sched["end"], begin, end)
		}
		return nil
	})
}

const replScheduleBody = `
  auto             = true
  retention_policy = "SOURCE"
  also_include_naming_schema = ["tfacc-%Y%m%d-%H%M"]
  schedule = { minute = "0", hour = "1", dom = "*", month = "*", dow = "*" }
`

// TestAccReplication_unsetSchedule: removing schedule (to turn auto-run off)
// must clear it on the server. The provider omitted the key, the server kept
// the old schedule and rejected it for auto = false with EINVAL (B-014).
func TestAccReplication_unsetSchedule(t *testing.T) {
	name := acctest.RandName("tf-acc-repl")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckReplicationDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + replConfig(name, replScheduleBody),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(replAddr, "schedule.hour", "1"),
					resource.TestCheckResourceAttr(replAddr, "schedule.begin", "00:00"),
					resource.TestCheckResourceAttr(replAddr, "schedule.end", "23:59"),
				),
			},
			{
				Config: acctest.ProviderConfig() + replConfig(name, `
  auto             = false
  retention_policy = "SOURCE"
  also_include_naming_schema = ["tfacc-%Y%m%d-%H%M"]
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(replAddr, "auto", "false"),
					resource.TestCheckNoResourceAttr(replAddr, "schedule.hour"),
					replCheckServer(func(got map[string]any) error {
						if got["schedule"] != nil {
							return fmt.Errorf("server schedule = %v, want null", got["schedule"])
						}
						return nil
					}),
				),
			},
		},
	})
}

// replHourlyBody runs hourly: the server asserts that the cron hours fall
// inside the begin/end window, so the window tests need hour = "*".
const replHourlyBody = `
  auto             = true
  retention_policy = "SOURCE"
  also_include_naming_schema = ["tfacc-%Y%m%d-%H%M"]
  schedule = { minute = "0", hour = "*", dom = "*", month = "*", dow = "*" }
`

// TestAccReplication_scheduleWindowKept: a schedule window (begin/end) set
// out of band must survive an unrelated update. begin/end were not modeled
// and the server replaces the whole schedule object, so every update reset
// the window to 00:00-23:59 (the B-036 class). The window is now managed too.
func TestAccReplication_scheduleWindowKept(t *testing.T) {
	name := acctest.RandName("tf-acc-repl")
	var id string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckReplicationDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + replConfig(name, replHourlyBody),
				Check: func(s *terraform.State) error {
					id = s.RootModule().Resources[replAddr].Primary.ID
					return nil
				},
			},
			{
				PreConfig: func() {
					n, _ := strconv.ParseInt(id, 10, 64)
					_, err := acctest.Client().Call(context.Background(), "replication.update", n,
						map[string]any{"schedule": map[string]any{"minute": "0", "hour": "*", "dom": "*", "month": "*", "dow": "*", "begin": "08:00", "end": "17:00"}})
					if err != nil {
						t.Fatalf("out-of-band update: %v", err)
					}
				},
				Config: acctest.ProviderConfig() + replConfig(name, replHourlyBody+`  retries = 3
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					replCheckServerWindow("schedule", "08:00", "17:00"),
					resource.TestCheckResourceAttr(replAddr, "retries", "3"),
					resource.TestCheckResourceAttr(replAddr, "schedule.begin", "08:00"),
				),
			},
			{
				Config: acctest.ProviderConfig() + replConfig(name, `
  auto             = true
  retention_policy = "SOURCE"
  also_include_naming_schema = ["tfacc-%Y%m%d-%H%M"]
  schedule = { minute = "0", hour = "*", dom = "*", month = "*", dow = "*", begin = "09:30", end = "18:00" }
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					replCheckServerWindow("schedule", "09:30", "18:00"),
					resource.TestCheckResourceAttr(replAddr, "schedule.end", "18:00"),
				),
			},
			{
				ResourceName:      replAddr,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccReplication_customToSource: switching retention CUSTOM -> SOURCE by
// removing lifetime_value/lifetime_unit/lifetimes. They were pinned by
// UseStateForUnknown or omitted from the update, so the server kept them and
// rejected them under SOURCE (B-015). Then back to CUSTOM.
func TestAccReplication_customToSource(t *testing.T) {
	custom := `
  auto             = false
  retention_policy = "CUSTOM"
  lifetime_value   = 2
  lifetime_unit    = "WEEK"
  also_include_naming_schema = ["tfacc-%Y%m%d-%H%M"]
  lifetimes = [{
    schedule       = { minute = "0", hour = "0", dom = "*", month = "*", dow = "*" }
    lifetime_value = 1
    lifetime_unit  = "DAY"
  }]
`
	name := acctest.RandName("tf-acc-repl")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckReplicationDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + replConfig(name, custom),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(replAddr, "lifetime_value", "2"),
					resource.TestCheckResourceAttr(replAddr, "lifetimes.#", "1"),
				),
			},
			{
				Config: acctest.ProviderConfig() + replConfig(name, `
  auto             = false
  retention_policy = "SOURCE"
  also_include_naming_schema = ["tfacc-%Y%m%d-%H%M"]
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(replAddr, "retention_policy", "SOURCE"),
					resource.TestCheckNoResourceAttr(replAddr, "lifetime_value"),
					resource.TestCheckNoResourceAttr(replAddr, "lifetime_unit"),
					resource.TestCheckNoResourceAttr(replAddr, "lifetimes.#"),
					replCheckServer(func(got map[string]any) error {
						if got["lifetime_value"] != nil || got["lifetime_unit"] != nil {
							return fmt.Errorf("server lifetime = %v %v, want null", got["lifetime_value"], got["lifetime_unit"])
						}
						if l, _ := got["lifetimes"].([]any); len(l) != 0 {
							return fmt.Errorf("server lifetimes = %v, want []", got["lifetimes"])
						}
						return nil
					}),
				),
			},
			{
				Config: acctest.ProviderConfig() + replConfig(name, custom),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(replAddr, "retention_policy", "CUSTOM"),
					resource.TestCheckResourceAttr(replAddr, "lifetime_unit", "WEEK"),
					resource.TestCheckResourceAttr(replAddr, "lifetimes.0.lifetime_unit", "DAY"),
				),
			},
		},
	})
}

// TestAccReplication_emptyCollectionsRejected: lifetimes = [] and
// properties_override = {} are stored by the server as "none" and read back
// as null, so the apply failed with an inconsistent result (B-015, B-125).
// They are now rejected at plan time; omit them instead.
func TestAccReplication_emptyCollectionsRejected(t *testing.T) {
	name := acctest.RandName("tf-acc-repl")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + replConfig(name, `
  auto             = false
  retention_policy = "CUSTOM"
  lifetime_value   = 2
  lifetime_unit    = "WEEK"
  also_include_naming_schema = ["tfacc-%Y%m%d-%H%M"]
  lifetimes = []
`),
				ExpectError: regexp.MustCompile(`(?s)lifetimes.*at\s+least\s+1`),
			},
			{
				Config: acctest.ProviderConfig() + replConfig(name, `
  auto             = false
  retention_policy = "SOURCE"
  also_include_naming_schema = ["tfacc-%Y%m%d-%H%M"]
  properties_override = {}
`),
				ExpectError: regexp.MustCompile(`(?s)properties_override.*at\s+least\s+1`),
			},
			// A valid last step: post-test destroy uses the last step's config.
			{
				Config: acctest.ProviderConfig() + replConfig(name, `
  auto             = false
  retention_policy = "SOURCE"
  also_include_naming_schema = ["tfacc-%Y%m%d-%H%M"]
`),
			},
		},
	})
}

// TestAccReplication_unsetRestrictScheduleAndOverride: removing
// restrict_schedule and properties_override from config must clear them. The
// provider omitted the keys, the server kept the old values, and the apply
// failed with an inconsistent result (B-089).
func TestAccReplication_unsetRestrictScheduleAndOverride(t *testing.T) {
	name := acctest.RandName("tf-acc-repl")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckReplicationDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + replConfig(name, replScheduleBody+`
  restrict_schedule = { minute = "0", hour = "*", dom = "*", month = "*", dow = "*", begin = "01:00", end = "22:00" }
  properties_override = { compression = "lz4" }
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(replAddr, "restrict_schedule.begin", "01:00"),
					resource.TestCheckResourceAttr(replAddr, "properties_override.compression", "lz4"),
				),
			},
			{
				Config: acctest.ProviderConfig() + replConfig(name, replScheduleBody),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(replAddr, "restrict_schedule.hour"),
					resource.TestCheckNoResourceAttr(replAddr, "properties_override.%"),
					replCheckServer(func(got map[string]any) error {
						if got["restrict_schedule"] != nil {
							return fmt.Errorf("server restrict_schedule = %v, want null", got["restrict_schedule"])
						}
						if po, _ := got["properties_override"].(map[string]any); len(po) != 0 {
							return fmt.Errorf("server properties_override = %v, want {}", got["properties_override"])
						}
						return nil
					}),
				),
			},
		},
	})
}

// TestAccReplication_restrictScheduleWindowDefaults: restrict_schedule without
// begin/end. The server fills 00:00/23:59, which failed the apply with an
// inconsistent result (B-087). An unpadded time ("8:00") is zero-padded by
// the server and is now rejected at plan time (B-088).
func TestAccReplication_restrictScheduleWindowDefaults(t *testing.T) {
	name := acctest.RandName("tf-acc-repl")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckReplicationDestroyed(name),
		Steps: []resource.TestStep{
			// First, not last: post-test destroy uses the last step's config.
			{
				Config: acctest.ProviderConfig() + replConfig(name, replScheduleBody+`
  restrict_schedule = { minute = "0", hour = "*", dom = "*", month = "*", dow = "*", begin = "8:00", end = "17:00" }
`),
				ExpectError: regexp.MustCompile(`leading\s+zero`),
			},
			{
				Config: acctest.ProviderConfig() + replConfig(name, replScheduleBody+`
  restrict_schedule = { minute = "0", hour = "*", dom = "*", month = "*", dow = "*" }
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(replAddr, "restrict_schedule.begin", "00:00"),
					resource.TestCheckResourceAttr(replAddr, "restrict_schedule.end", "23:59"),
				),
			},
		},
	})
}

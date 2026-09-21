// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cronjob_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccCronjob_basic creates a cron job running /usr/bin/true as root,
// disabled so nothing is ever actually scheduled to run, checks its
// attributes, updates its description in place, imports it by numeric id,
// and verifies destruction via a live cronjob.query.
func TestAccCronjob_basic(t *testing.T) {
	desc := acctest.RandName("tf-acc-cronjob")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCronjobDestroyed(desc),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccCronjobConfig(desc),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_cronjob.test", "id"),
					resource.TestCheckResourceAttr("truenas_cronjob.test", "command", "/usr/bin/true"),
					resource.TestCheckResourceAttr("truenas_cronjob.test", "user", "root"),
					resource.TestCheckResourceAttr("truenas_cronjob.test", "description", desc),
					resource.TestCheckResourceAttr("truenas_cronjob.test", "enabled", "false"),
					resource.TestCheckResourceAttr("truenas_cronjob.test", "schedule.minute", "0"),
					resource.TestCheckResourceAttr("truenas_cronjob.test", "schedule.hour", "3"),
				),
			},
			// Update description in place.
			{
				Config: acctest.ProviderConfig() + testAccCronjobConfig(desc+"-updated"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_cronjob.test", "description", desc+"-updated"),
				),
			},
			// Import by the job's numeric id. Every field the API tracks is
			// echoed back on read (no write-only flags like rsync_task), so
			// ImportStateVerify needs no ignore list.
			{
				ResourceName:      "truenas_cronjob.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCronjobConfig(desc string) string {
	return fmt.Sprintf(`
resource "truenas_cronjob" "test" {
  command     = "/usr/bin/true"
  user        = "root"
  enabled     = false
  description = %q
  schedule = {
    minute = "0"
    hour   = "3"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}
`, desc)
}

// cronjobSummary is the subset of cronjob.query fields this test package
// needs directly (outside of the provider's own resource code).
type cronjobSummary struct {
	ID int64 `json:"id"`
}

// testAccCheckCronjobDestroyed queries cronjob by the fixture's description
// since the job's ID is not known outside of Terraform state at
// CheckDestroy time.
func testAccCheckCronjobDestroyed(desc string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		for _, d := range []string{desc, desc + "-updated"} {
			raw, err := c.Call(context.Background(), "cronjob.query", [][]any{{"description", "=", d}})
			if err != nil {
				return fmt.Errorf("error checking cron job for description %q: %v", d, err)
			}
			var results []cronjobSummary
			if err := json.Unmarshal(raw, &results); err != nil {
				return fmt.Errorf("error parsing cronjob.query response: %v", err)
			}
			if len(results) > 0 {
				return fmt.Errorf("cron job with description %q still exists", d)
			}
		}
		return nil
	}
}

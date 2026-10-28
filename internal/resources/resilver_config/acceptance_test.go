// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package resilver_config_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccResilverConfigDataSource_basic reads the current TrueNAS resilver
// configuration through the truenas_resilver_config datasource only. It
// never writes: the resilver schedule is system-wide pool maintenance
// configuration, and the box's real schedule must not be overwritten by an
// acceptance test run.
func TestAccResilverConfigDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_resilver_config" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_resilver_config.test", "id", "resilver_config"),
					resource.TestCheckResourceAttrSet("data.truenas_resilver_config.test", "begin"),
					resource.TestCheckResourceAttrSet("data.truenas_resilver_config.test", "end"),
				),
			},
		},
	})
}

// resilverConfigOriginal captures the fields TestAccResilverConfig_setAndRestore touches.
type resilverConfigOriginal struct {
	Enabled bool    `json:"enabled"`
	Weekday []int64 `json:"weekday"`
	Begin   string  `json:"begin"`
	End     string  `json:"end"`
}

// readResilverConfigOriginal reads the box's current enabled/weekday via
// pool.resilver.config, so the test can restore them exactly afterward.
func readResilverConfigOriginal(t *testing.T) resilverConfigOriginal {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "pool.resilver.config")
	if err != nil {
		t.Fatalf("error reading current resilver config: %v", err)
	}
	var orig resilverConfigOriginal
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing pool.resilver.config response: %v", err)
	}
	return orig
}

// restoreResilverConfig sends enabled/weekday back to their original values
// via pool.resilver.update. It runs from t.Cleanup, so it restores the box
// even if the Terraform steps themselves fail partway through.
func restoreResilverConfig(t *testing.T, orig resilverConfigOriginal) {
	t.Helper()
	if _, err := acctest.Client().Call(context.Background(), "pool.resilver.update", map[string]any{
		"enabled": orig.Enabled,
		"weekday": orig.Weekday,
		"begin":   orig.Begin,
		"end":     orig.End,
	}); err != nil {
		t.Fatalf("error restoring resilver config: %v", err)
	}
}

// TestAccResilverConfig_setAndRestore drives the singleton
// truenas_resilver_config resource's "enabled" field through a toggled
// value and back to the value read from the box before the test ran, then
// imports it. It requires TF_ACC=1 and TRUENAS_DISRUPTIVE=1
// (acctest.DisruptiveCheck), since it mutates the box's live resilver
// schedule; a t.Cleanup-registered API restore is the safety net if the
// Terraform steps fail.
func TestAccResilverConfig_setAndRestore(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readResilverConfigOriginal(t)
	t.Cleanup(func() { restoreResilverConfig(t, orig) })

	toggled := !orig.Enabled

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccResilverConfigConfig(toggled),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_resilver_config.test", "id", "resilver_config"),
					resource.TestCheckResourceAttr("truenas_resilver_config.test", "enabled", fmt.Sprintf("%v", toggled)),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccResilverConfigConfig(orig.Enabled),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_resilver_config.test", "enabled", fmt.Sprintf("%v", orig.Enabled)),
				),
			},
			{
				ResourceName:      "truenas_resilver_config.test",
				ImportState:       true,
				ImportStateId:     "resilver_config",
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccResilverConfig_coverage sets the schedule window attributes the
// toggle test leaves alone (begin, end, weekday) to values that differ from
// the box's current ones, asserts they read back, and relies on the
// framework's post-apply empty-plan check to catch any value that does not
// round-trip. "enabled" is held at its original value so the box's live
// resilver behavior is not switched on or off. The original schedule is
// restored (and re-read to confirm) from t.Cleanup.
func TestAccResilverConfig_coverage(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readResilverConfigOriginal(t)
	t.Cleanup(func() {
		restoreResilverConfig(t, orig)
		got := readResilverConfigOriginal(t)
		if got.Begin != orig.Begin || got.End != orig.End || got.Enabled != orig.Enabled ||
			fmt.Sprint(got.Weekday) != fmt.Sprint(orig.Weekday) {
			t.Errorf("resilver config not restored: want %+v, got %+v", orig, got)
		}
	})

	begin, end := "01:00", "05:00"
	if orig.Begin == begin && orig.End == end {
		begin, end = "02:00", "06:00"
	}
	weekday := []int64{1, 2, 3}
	if fmt.Sprint(orig.Weekday) == fmt.Sprint(weekday) {
		weekday = []int64{4, 5, 6}
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_resilver_config" "test" {
  enabled = %v
  begin   = %q
  end     = %q
  weekday = [%d, %d, %d]
}
`, orig.Enabled, begin, end, weekday[0], weekday[1], weekday[2]),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_resilver_config.test", "begin", begin),
					resource.TestCheckResourceAttr("truenas_resilver_config.test", "end", end),
					resource.TestCheckResourceAttr("truenas_resilver_config.test", "weekday.#", "3"),
					resource.TestCheckResourceAttr("truenas_resilver_config.test", "weekday.0", fmt.Sprint(weekday[0])),
					resource.TestCheckResourceAttr("truenas_resilver_config.test", "weekday.1", fmt.Sprint(weekday[1])),
					resource.TestCheckResourceAttr("truenas_resilver_config.test", "weekday.2", fmt.Sprint(weekday[2])),
				),
			},
		},
	})
}

func testAccResilverConfigConfig(enabled bool) string {
	return fmt.Sprintf(`
resource "truenas_resilver_config" "test" {
  enabled = %v
}
`, enabled)
}

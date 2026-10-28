// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_general_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccSystemGeneralDataSource_basic reads the current TrueNAS system
// general configuration through the truenas_system_general datasource only.
// It never writes: this configuration controls the management UI (bind
// addresses, ports, certificate), and the box's real configuration must not
// be overwritten by an acceptance test run.
//
// Requires:
//   - TF_ACC=1
//   - TRUENAS_API_KEY (or TRUENAS_USERNAME+TRUENAS_PASSWORD) set for
//     acctest.PreCheck
//   - A running TrueNAS instance
func TestAccSystemGeneralDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_system_general" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_system_general.test", "id", "system_general"),
					resource.TestCheckResourceAttrSet("data.truenas_system_general.test", "timezone"),
				),
			},
		},
	})
}

// TestAccSystemGeneral_basic is intentionally skipped by default.
// truenas_system_general is a SINGLETON resource that controls the
// management UI: the box this suite runs against is managed live through
// that UI, and bringing this configuration under Terraform management
// mutates the box's actual system general configuration
// (system.general.update). A naive create/update/destroy acceptance test
// risks changing ui_address, ui_port, ui_httpsport, ui_allowlist, or
// ui_certificate on a live system and cutting off management access
// entirely.
//
// If this test is ever enabled against a disposable/non-production TrueNAS
// instance, it should:
//  1. Read the current config via the datasource first.
//  2. Only touch a low-risk field (e.g. "ui_consolemsg") in the resource
//     config, driving it through a value and back to the value the
//     datasource observed in step 1, so the net effect on the box is a
//     no-op. Never touch ui_address, ui_allowlist, ui_port, ui_httpsport,
//     ui_v6address, or ui_certificate in an automated test: changing those
//     on a live system can cut off management access.
//  3. Use ImportState with ImportStateId "system_general" to verify import
//     normalizes any ID to the fixed singleton ID.
//
// It is gated behind TRUENAS_TEST_SYSTEM_GENERAL (not just TF_ACC). It touches
// ONLY `timezone`: updatePayload sends only the fields set in config, so the
// UI ports (ui_port/ui_httpsport) and addresses are never rewritten and
// management access can't be cut. A t.Cleanup restores the box's original
// timezone regardless of outcome. Run only against a disposable box.
func TestAccSystemGeneral_basic(t *testing.T) {
	if os.Getenv("TRUENAS_TEST_SYSTEM_GENERAL") == "" {
		t.Skip("set TRUENAS_TEST_SYSTEM_GENERAL=1 (on a disposable box) to run the system_general set/restore test")
	}
	acctest.PreCheck(t)

	orig := currentTimezone(t)
	t.Cleanup(func() {
		if _, err := acctest.Client().Call(context.Background(), "system.general.update",
			map[string]any{"timezone": orig}); err != nil {
			t.Logf("WARNING: failed to restore timezone=%q: %v", orig, err)
		}
	})

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccSystemGeneralTimezone("UTC"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_system_general.test", "id", "system_general"),
					resource.TestCheckResourceAttr("truenas_system_general.test", "timezone", "UTC"),
				),
			},
			// Update path: change to another valid timezone in place.
			{
				Config: acctest.ProviderConfig() + testAccSystemGeneralTimezone("America/New_York"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_system_general.test", "timezone", "America/New_York"),
				),
			},
			{
				ResourceName:      "truenas_system_general.test",
				ImportState:       true,
				ImportStateId:     "system_general",
				ImportStateVerify: true,
			},
		},
	})
}

func testAccSystemGeneralTimezone(tz string) string {
	return fmt.Sprintf(`
resource "truenas_system_general" "test" {
  timezone = %q
}
`, tz)
}

// currentTimezone reads the box's current system.general timezone.
func currentTimezone(t *testing.T) string {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "system.general.config")
	if err != nil {
		t.Fatalf("reading system.general.config: %v", err)
	}
	var cfg struct {
		Timezone string `json:"timezone"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parsing system.general.config: %v", err)
	}
	return cfg.Timezone
}

// safeGeneralFields are the system.general fields TestAccSystemGeneral_coverage
// sets and restores. None of them affect how the box is reached over the
// network (no addresses, ports, certificate, allowlist, or HTTPS settings).
var safeGeneralFields = []string{"kbdmap", "ui_consolemsg", "ui_x_frame_options", "usage_collection"}

// readGeneralSafeFields reads the current values of safeGeneralFields.
func readGeneralSafeFields(t *testing.T) map[string]any {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "system.general.config")
	if err != nil {
		t.Fatalf("reading system.general.config: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parsing system.general.config: %v", err)
	}
	out := map[string]any{}
	for _, k := range safeGeneralFields {
		out[k] = cfg[k]
	}
	return out
}

// TestAccSystemGeneral_coverage sets kbdmap, ui_consolemsg,
// ui_x_frame_options and usage_collection to values that differ from the
// box's current ones, then restores the captured originals and re-reads to
// confirm. The framework's post-apply empty-plan check verifies each value
// round-trips.
//
// Deliberately NOT set (risk the box's network/UI reachability): ui_address,
// ui_v6address, ui_allowlist, ui_certificate, ui_port, ui_httpsport,
// ui_httpsredirect, ui_httpsprotocols.
func TestAccSystemGeneral_coverage(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readGeneralSafeFields(t)
	t.Cleanup(func() {
		if _, err := acctest.Client().Call(context.Background(), "system.general.update", orig); err != nil {
			t.Errorf("FAILED to restore system.general fields %v: %v", orig, err)
			return
		}
		after := readGeneralSafeFields(t)
		for _, k := range safeGeneralFields {
			if fmt.Sprint(after[k]) != fmt.Sprint(orig[k]) {
				t.Errorf("restore of %s not confirmed: got %v, want %v", k, after[k], orig[k])
			}
		}
	})

	flip := func(k string) bool {
		b, _ := orig[k].(bool)
		return !b
	}
	kbdmap := "de"
	if orig["kbdmap"] == "de" {
		kbdmap = "us"
	}
	xfo := "DENY"
	if orig["ui_x_frame_options"] == "DENY" {
		xfo = "SAMEORIGIN"
	}

	cfg := fmt.Sprintf(`
resource "truenas_system_general" "test" {
  kbdmap             = %q
  ui_consolemsg      = %t
  ui_x_frame_options = %q
  usage_collection   = %t
}
`, kbdmap, flip("ui_consolemsg"), xfo, flip("usage_collection"))

	r := "truenas_system_general.test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(r, "kbdmap", kbdmap),
					resource.TestCheckResourceAttr(r, "ui_consolemsg", fmt.Sprint(flip("ui_consolemsg"))),
					resource.TestCheckResourceAttr(r, "ui_x_frame_options", xfo),
					resource.TestCheckResourceAttr(r, "usage_collection", fmt.Sprint(flip("usage_collection"))),
				),
			},
		},
	})
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_general_test

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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
var safeGeneralFields = []string{"kbdmap", "ui_consolemsg", "ui_x_frame_options", "usage_collection", "usage_collection_is_set"}

// restoreGeneralPayload turns captured safeGeneralFields into a
// system.general.update payload. usage_collection goes back to null when the
// administrator had never set it, so usage_collection_is_set stays false
// (sending the value would record an explicit opt-in). With restartWeb the
// web server is restarted so a restored ui_x_frame_options takes effect.
func restoreGeneralPayload(orig map[string]any, restartWeb bool) map[string]any {
	p := map[string]any{}
	if restartWeb {
		p["ui_restart_delay"] = 1
	}
	for _, k := range safeGeneralFields {
		if k != "usage_collection_is_set" {
			p[k] = orig[k]
		}
	}
	if set, _ := orig["usage_collection_is_set"].(bool); !set {
		p["usage_collection"] = nil
	}
	return p
}

// readGeneralSafeFields reads the current values of safeGeneralFields.
func readGeneralSafeFields(t *testing.T) map[string]any {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "system.general.config")
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
		if _, err := acctest.RestoreCall(context.Background(), "system.general.update", restoreGeneralPayload(orig, true)); err != nil {
			t.Errorf("FAILED to restore system.general fields %v: %v", orig, err)
			return
		}
		time.Sleep(12 * time.Second) // let the web server restart before reading back
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

// liveXFrameOptions fetches the web UI root over HTTPS and returns the
// X-Frame-Options header nginx is actually serving.
func liveXFrameOptions() (string, error) {
	u, err := url.Parse(acctest.Endpoint())
	if err != nil {
		return "", err
	}
	u.Scheme, u.Path = "https", "/"
	hc := &http.Client{
		Timeout:   15 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, //nolint:gosec // lab boxes use self-signed certificates
	}
	resp, err := hc.Get(u.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return resp.Header.Get("X-Frame-Options"), nil
}

func checkLiveXFrameOptions(want string) resource.TestCheckFunc {
	if want == "ALLOW_ALL" {
		want = "" // nginx sends no X-Frame-Options header for ALLOW_ALL
	}
	return func(*terraform.State) error {
		got, err := liveXFrameOptions()
		if err != nil {
			return fmt.Errorf("fetch web UI headers: %w", err)
		}
		if got != want {
			return fmt.Errorf("web server serves X-Frame-Options %q, want %q: the saved setting was not applied", got, want)
		}
		return nil
	}
}

// TestAccSystemGeneral_webServerSettingsApplied guards B-027: a web server
// setting changed through the resource must be served by nginx after the
// apply, not only saved. It flips ui_x_frame_options, checks the live
// response header, then sets the original value back the same way.
func TestAccSystemGeneral_webServerSettingsApplied(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readGeneralSafeFields(t)
	t.Cleanup(func() {
		if _, err := acctest.RestoreCall(context.Background(), "system.general.update", restoreGeneralPayload(orig, true)); err != nil {
			t.Errorf("FAILED to restore system.general fields %v: %v", orig, err)
		}
		time.Sleep(12 * time.Second)
	})
	origXFO, _ := orig["ui_x_frame_options"].(string)
	xfo := "DENY"
	if origXFO == "DENY" {
		xfo = "SAMEORIGIN"
	}
	cfg := func(v string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_system_general" "test" {
  ui_x_frame_options = %q
}
`, v)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg(xfo),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_system_general.test", "ui_x_frame_options", xfo),
					checkLiveXFrameOptions(xfo),
				),
			},
			{
				Config: cfg(origXFO),
				Check:  checkLiveXFrameOptions(origXFO),
			},
		},
	})
}

// TestAccSystemGeneral_usageCollectionNotRecorded guards B-143: with
// usage_collection never set by an administrator (usage_collection_is_set =
// false), an update of an unrelated attribute must not send usage_collection,
// which records an explicit opt-in. ui_consolemsg is the unrelated attribute
// (cheap to change, no network effect). Original values are restored.
func TestAccSystemGeneral_usageCollectionNotRecorded(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readGeneralSafeFields(t)
	t.Cleanup(func() {
		if _, err := acctest.RestoreCall(context.Background(), "system.general.update", restoreGeneralPayload(orig, false)); err != nil {
			t.Errorf("FAILED to restore system.general fields %v: %v", orig, err)
		}
	})
	// Start from "never set": a null usage_collection clears the flag.
	if _, err := acctest.RestoreCall(context.Background(), "system.general.update", map[string]any{"usage_collection": nil}); err != nil {
		t.Fatalf("clear usage_collection: %v", err)
	}
	b, _ := orig["ui_consolemsg"].(bool)
	cfg := func(v bool) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_system_general" "test" {
  ui_consolemsg = %t
}
`, v)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg(!b), Check: resource.TestCheckResourceAttr("truenas_system_general.test", "usage_collection_is_set", "false")},
			{Config: cfg(b), Check: resource.TestCheckResourceAttr("truenas_system_general.test", "usage_collection_is_set", "false")},
		},
	})
}

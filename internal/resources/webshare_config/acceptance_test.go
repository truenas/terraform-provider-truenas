// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package webshare_config_test

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccWebshareConfigDataSource_basic reads the current TrueNAS Webshare
// configuration through the truenas_webshare_config datasource only. It
// never writes. Requires TrueNAS 26.0+: the webshare namespace does
// not exist on earlier releases (probed live — TrueNAS 25.10 exposes 0
// webshare.* methods via core.get_methods), so this test self-skips cleanly
// via acctest.ServerVersionAtLeast on any older box, matching this
// resource's own version-gate diagnostic (see resource.go's checkVersion /
// model.go's versionGateDiagnostics).
func TestAccWebshareConfigDataSource_basic(t *testing.T) {
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("truenas_webshare_config requires TrueNAS 26.0 or later (webshare namespace absent on 25.10, confirmed live)")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_webshare_config" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_webshare_config.test", "id", "webshare_config"),
					resource.TestCheckResourceAttrSet("data.truenas_webshare_config.test", "passkey"),
				),
			},
		},
	})
}

// webshareConfigOriginal captures the fields TestAccWebshareConfig_setAndRestore
// touches, as read from webshare.config before the test runs.
type webshareConfigOriginal struct {
	Search bool `json:"search"`
}

// readWebshareConfigOriginal reads the box's current webshare.config via
// acctest.RestoreCall (job:false, probed live on TrueNAS 26.0), so the test
// can restore the exact original "search" value afterward. Returns
// ok=false (rather than failing the test outright) when the read itself
// errors, so the caller can self-skip — mirroring the mail/ups/lxc_config
// precedent.
func readWebshareConfigOriginal(t *testing.T) (webshareConfigOriginal, bool) {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "webshare.config")
	if err != nil {
		return webshareConfigOriginal{}, false
	}
	var orig webshareConfigOriginal
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing webshare.config response: %v", err)
	}
	return orig, true
}

// restoreWebshareConfigSearch sends "search" back to its original value via
// webshare.update. webshare.update is job:false (probed live on TrueNAS
// 26.0) and accepts a genuine partial update (probed live: sending only
// "search" leaves bindip/passkey/groups untouched), so the plain
// acctest.RestoreCall (which wraps a job-unaware CallRead) is sufficient
// here — no CallJob polling needed, and no other field needs to be resent.
func restoreWebshareConfigSearch(t *testing.T, search bool) {
	t.Helper()
	if _, err := acctest.RestoreCall(context.Background(), "webshare.update", map[string]any{
		"search": search,
	}); err != nil {
		t.Fatalf("error restoring webshare search: %v", err)
	}
}

// TestAccWebshareConfig_setAndRestore drives the singleton
// truenas_webshare_config resource's "search" field (a cosmetic,
// low-risk toggle — probed live: unlike passkey/groups/bindip it carries
// no authentication or network-exposure risk) through its opposite value,
// then back to the value read from the box before the test ran, then
// imports it. It requires TF_ACC=1, TRUENAS_DISRUPTIVE=1
// (acctest.DisruptiveCheck), and TrueNAS 26.0+ (self-skip via
// acctest.ServerVersionAtLeast — the webshare namespace does not exist on
// 25.10, confirmed live), since it mutates the box's live Webshare
// configuration; a t.Cleanup-registered API restore is the safety net if
// the Terraform steps fail.
//
// Probed live against the production 26.0 box before this test was
// written (see docs/superpowers/specs/2026-07-23-webshare-device-tnc-design.md):
// webshare.update accepts a partial payload — sending {"search": true} then
// {"search": false} round-tripped cleanly with no other field disturbed, and
// left bindip/passkey/groups exactly as they were. This differs from the
// mail/ups precedent (which requires several fields on every update call);
// this test's self-skip guard below is nonetheless kept as a defensive
// mirror of that precedent, in case webshare.update's requirements change.
func TestAccWebshareConfig_setAndRestore(t *testing.T) {
	acctest.DisruptiveCheck(t)
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("truenas_webshare_config requires TrueNAS 26.0 or later (webshare namespace absent on 25.10, confirmed live)")
	}

	orig, ok := readWebshareConfigOriginal(t)
	if !ok {
		t.Skip("webshare.config could not be read on the target box; search set-and-restore requires a readable Webshare configuration")
	}
	t.Cleanup(func() { restoreWebshareConfigSearch(t, orig.Search) })

	toggled := !orig.Search

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccWebshareConfigConfig(toggled),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_webshare_config.test", "id", "webshare_config"),
					resource.TestCheckResourceAttr("truenas_webshare_config.test", "search", boolStr(toggled)),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccWebshareConfigConfig(orig.Search),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_webshare_config.test", "search", boolStr(orig.Search)),
				),
			},
			{
				ResourceName:      "truenas_webshare_config.test",
				ImportState:       true,
				ImportStateId:     "webshare_config",
				ImportStateVerify: true,
			},
		},
	})
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func testAccWebshareConfigConfig(search bool) string {
	return `
resource "truenas_webshare_config" "test" {
  search = ` + boolStr(search) + `
}
`
}

// TestAccWebshareConfig_coverage sets passkey and bindip, which the search
// toggle test leaves alone, and asserts they read back with an empty
// post-apply plan. 26.0+ only and DisruptiveCheck-gated, like the toggle
// test.
//
// Safety: passkey is only ever moved between ENABLED and DISABLED, never
// REQUIRED (which could lock users out). bindip is only set to an address
// offered by webshare.bindip_choices (so it is valid on this box) and only
// when that differs from the current binding. groups is NOT exercised: it
// takes AD/LDAP group names, which need a directory service on the box, so
// it stays ratcheted. All four fields (search, bindip, passkey, groups) are
// restored from t.Cleanup and re-read to confirm.
func TestAccWebshareConfig_coverage(t *testing.T) {
	acctest.DisruptiveCheck(t)
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("truenas_webshare_config requires TrueNAS 26.0 or later (webshare namespace absent on 25.10, confirmed live)")
	}

	type cfgShape struct {
		Search  bool     `json:"search"`
		Bindip  []string `json:"bindip"`
		Passkey string   `json:"passkey"`
		Groups  []string `json:"groups"`
	}
	raw, err := acctest.RestoreCall(context.Background(), "webshare.config")
	if err != nil {
		t.Skip("webshare.config could not be read on the target box")
	}
	var orig cfgShape
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing webshare.config response: %v", err)
	}
	if orig.Bindip == nil {
		orig.Bindip = []string{}
	}
	if orig.Groups == nil {
		orig.Groups = []string{}
	}

	t.Cleanup(func() {
		if _, err := acctest.RestoreCall(context.Background(), "webshare.update", map[string]any{
			"search":  orig.Search,
			"bindip":  orig.Bindip,
			"passkey": orig.Passkey,
			"groups":  orig.Groups,
		}); err != nil {
			t.Errorf("error restoring webshare config: %v", err)
			return
		}
		raw, err := acctest.RestoreCall(context.Background(), "webshare.config")
		if err != nil {
			t.Errorf("error re-reading webshare.config after restore: %v", err)
			return
		}
		var got cfgShape
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Errorf("error parsing webshare.config after restore: %v", err)
			return
		}
		if got.Search != orig.Search || got.Passkey != orig.Passkey ||
			fmt.Sprint(got.Bindip) != fmt.Sprint(orig.Bindip) || fmt.Sprint(got.Groups) != fmt.Sprint(orig.Groups) {
			t.Errorf("webshare config not restored: want %+v, got %+v", orig, got)
		}
	})

	passkey := "ENABLED"
	if orig.Passkey == "ENABLED" {
		passkey = "DISABLED"
	}

	newIP := ""
	if raw, err := acctest.RestoreCall(context.Background(), "webshare.bindip_choices"); err == nil {
		var choices map[string]string
		if json.Unmarshal(raw, &choices) == nil {
			for k := range choices {
				if k != "" && (len(orig.Bindip) != 1 || orig.Bindip[0] != k) {
					newIP = k
					break
				}
			}
		}
	}

	cfg := fmt.Sprintf(`
resource "truenas_webshare_config" "test" {
  passkey = %q
}
`, passkey)
	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr("truenas_webshare_config.test", "passkey", passkey),
	}
	if newIP != "" {
		cfg = fmt.Sprintf(`
resource "truenas_webshare_config" "test" {
  passkey = %q
  bindip  = [%q]
}
`, passkey, newIP)
		checks = append(checks,
			resource.TestCheckResourceAttr("truenas_webshare_config.test", "bindip.#", "1"),
			resource.TestCheckResourceAttr("truenas_webshare_config.test", "bindip.0", newIP),
		)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + cfg,
				Check:  resource.ComposeTestCheckFunc(checks...),
			},
		},
	})
}

// TestAccWebshareConfig_mcp guards B-155: the TrueNAS 28.0 MCP server
// settings are managed and read back. The original values are restored
// through the API.
func TestAccWebshareConfig_mcp(t *testing.T) {
	acctest.DisruptiveCheck(t)
	if !acctest.ServerVersionAtLeast(t, 28, 0) {
		t.Skip("the Webshare MCP settings exist only on TrueNAS 28.0 and later")
	}
	type mcpShape struct {
		MCPEnabled       bool     `json:"mcp_enabled"`
		MCPAllowedGroups []string `json:"mcp_allowed_groups"`
		MCPAllowWrite    bool     `json:"mcp_allow_write"`
	}
	raw, err := acctest.RestoreCall(context.Background(), "webshare.config")
	if err != nil {
		t.Fatal(err)
	}
	var orig mcpShape
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatal(err)
	}
	if orig.MCPAllowedGroups == nil {
		orig.MCPAllowedGroups = []string{}
	}
	t.Cleanup(func() {
		if _, err := acctest.RestoreCall(context.Background(), "webshare.update", map[string]any{
			"mcp_enabled": orig.MCPEnabled, "mcp_allowed_groups": orig.MCPAllowedGroups, "mcp_allow_write": orig.MCPAllowWrite,
		}); err != nil {
			t.Errorf("error restoring webshare MCP settings: %v", err)
		}
	})
	const r = "truenas_webshare_config.test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
resource "truenas_webshare_config" "test" {
  mcp_enabled        = true
  mcp_allowed_groups = ["truenas_webshare"]
  mcp_allow_write    = true
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(r, "mcp_enabled", "true"),
					resource.TestCheckResourceAttr(r, "mcp_allowed_groups.#", "1"),
					resource.TestCheckResourceAttr(r, "mcp_allowed_groups.0", "truenas_webshare"),
					resource.TestCheckResourceAttr(r, "mcp_allow_write", "true"),
				),
			},
			{
				Config: acctest.ProviderConfig() + `
resource "truenas_webshare_config" "test" {
  mcp_enabled        = false
  mcp_allowed_groups = []
  mcp_allow_write    = false
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(r, "mcp_enabled", "false"),
					resource.TestCheckResourceAttr(r, "mcp_allowed_groups.#", "0"),
				),
			},
			{
				ResourceName:      r,
				ImportState:       true,
				ImportStateId:     "webshare_config",
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccWebshareConfig_mcpRejectedBefore28: on TrueNAS 26.0/27.x setting an
// MCP field is a clear error, raised before anything is written (so this
// runs without TRUENAS_DISRUPTIVE).
func TestAccWebshareConfig_mcpRejectedBefore28(t *testing.T) {
	acctest.PreCheck(t)
	if !acctest.ServerVersionAtLeast(t, 26, 0) || acctest.ServerVersionAtLeast(t, 28, 0) {
		t.Skip("needs TrueNAS 26.0 or 27.x")
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
resource "truenas_webshare_config" "test" {
  mcp_enabled = false
}
`,
				ExpectError: regexp.MustCompile(`mcp_enabled\s+requires\s+TrueNAS\s+28.0\s+or\s+later`),
			},
		},
	})
}

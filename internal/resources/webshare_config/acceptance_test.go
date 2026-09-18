// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package webshare_config_test

import (
	"context"
	"encoding/json"
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

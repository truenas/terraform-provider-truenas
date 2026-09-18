// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package lxc_config_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccLXCConfigDataSource_basic reads the current TrueNAS LXC
// configuration through the truenas_lxc_config datasource only. It never
// writes. Requires TrueNAS 26.0+: the lxc namespace does not exist on
// earlier releases (probed live — TrueNAS 25.10 returns "Method does not
// exist" for lxc.config), so this test self-skips cleanly via
// acctest.ServerVersionAtLeast on any older box, matching this resource's
// own version-gate diagnostic (see resource.go's checkVersion /
// model.go's versionGateDiagnostics).
func TestAccLXCConfigDataSource_basic(t *testing.T) {
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("truenas_lxc_config requires TrueNAS 26.0 or later (lxc namespace absent on 25.10, confirmed live)")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_lxc_config" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_lxc_config.test", "id", "lxc_config"),
					resource.TestCheckResourceAttrSet("data.truenas_lxc_config.test", "v4_network"),
					resource.TestCheckResourceAttrSet("data.truenas_lxc_config.test", "v6_network"),
				),
			},
		},
	})
}

// lxcConfigOriginal captures the fields TestAccLXCConfig_setAndRestore
// touches, as read from lxc.config before the test runs.
type lxcConfigOriginal struct {
	PreferredPool *string `json:"preferred_pool"`
	V4Network     string  `json:"v4_network"`
}

// readLXCConfigOriginal reads the box's current lxc.config via
// acctest.RestoreCall (job:false, probed live on TrueNAS 26.0), so the test
// can decide whether it is safe to run at all (preferred_pool must be null
// — see the decisive-probe rationale on TestAccLXCConfig_setAndRestore) and
// restore the exact original v4_network afterward. Returns ok=false (rather
// than failing the test outright) when the read itself errors, so the
// caller can self-skip — mirroring the mail/ups/catalog_config precedent.
func readLXCConfigOriginal(t *testing.T) (lxcConfigOriginal, bool) {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "lxc.config")
	if err != nil {
		return lxcConfigOriginal{}, false
	}
	var orig lxcConfigOriginal
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing lxc.config response: %v", err)
	}
	return orig, true
}

// restoreLXCConfigV4Network sends v4_network back to its original value via
// lxc.update. lxc.update is job:false (probed live on TrueNAS 26.0), so the
// plain acctest.RestoreCall (which wraps a job-unaware CallRead) is
// sufficient here — no CallJob polling needed. It deliberately sends
// v4_network ONLY: this resource's docker-pool safety rule means the test
// (and its restore) must never send preferred_pool at all, so a no-op
// restore of the pool the box already has is not even attempted.
func restoreLXCConfigV4Network(t *testing.T, v4Network string) {
	t.Helper()
	if _, err := acctest.RestoreCall(context.Background(), "lxc.update", map[string]any{
		"v4_network": v4Network,
	}); err != nil {
		t.Fatalf("error restoring lxc v4_network: %v", err)
	}
}

// TestAccLXCConfig_setAndRestore drives the singleton truenas_lxc_config
// resource's "v4_network" field through one changed CIDR, then back to the
// value read from the box before the test ran, then imports it. It requires
// TF_ACC=1, TRUENAS_DISRUPTIVE=1 (acctest.DisruptiveCheck), and TrueNAS
// TrueNAS 26.0+ (self-skip via acctest.ServerVersionAtLeast — the lxc
// namespace does not exist on 25.10, confirmed live), since it mutates the
// box's live LXC network configuration; a t.Cleanup-registered API restore
// is the safety net if the Terraform steps fail.
//
// SAFETY: this test — and its cleanup — NEVER sets or reads back
// "preferred_pool" through Terraform. That field selects the ZFS pool LXC
// uses for instance/image datasets; changing it on a box where LXC is
// already in use is a real migration operation, not a cosmetic toggle (see
// docker_config's identical "pool" rule, which this mirrors). Per the
// decisive probe run against the production 26.0 box before this test was
// written (see the design doc, docs/superpowers/specs/
// 2026-07-23-lxc-config-design.md): lxc.config's preferred_pool read back
// null there (LXC not in use), so mutating v4_network alone was judged safe
// — this test still checks that live, at run time, and self-skips if
// preferred_pool is ever non-null (LXC has since been configured on the
// target box), rather than assuming the probe result still holds.
func TestAccLXCConfig_setAndRestore(t *testing.T) {
	acctest.DisruptiveCheck(t)
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("truenas_lxc_config requires TrueNAS 26.0 or later (lxc namespace absent on 25.10, confirmed live)")
	}

	orig, ok := readLXCConfigOriginal(t)
	if !ok {
		t.Skip("lxc.config could not be read on the target box; v4_network set-and-restore requires a readable LXC configuration")
	}
	if orig.PreferredPool != nil {
		t.Skip("lxc.config's preferred_pool is set (LXC is in use) on the target box; skipping to avoid any risk of disturbing an active LXC pool/network configuration — see the in-file safety rationale")
	}
	t.Cleanup(func() { restoreLXCConfigV4Network(t, orig.V4Network) })

	toggled := "172.201.0.0/24"
	if toggled == orig.V4Network {
		toggled = "172.202.0.0/24"
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccLXCConfigConfig(toggled),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_lxc_config.test", "id", "lxc_config"),
					resource.TestCheckResourceAttr("truenas_lxc_config.test", "v4_network", toggled),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccLXCConfigConfig(orig.V4Network),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_lxc_config.test", "v4_network", orig.V4Network),
				),
			},
			{
				ResourceName:      "truenas_lxc_config.test",
				ImportState:       true,
				ImportStateId:     "lxc_config",
				ImportStateVerify: true,
			},
		},
	})
}

func testAccLXCConfigConfig(v4Network string) string {
	return `
resource "truenas_lxc_config" "test" {
  v4_network = "` + v4Network + `"
}
`
}

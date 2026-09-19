// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package tn_connect_config_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccTnConnectConfigDataSource_basic reads the current TrueNAS Connect
// configuration through the truenas_tn_connect_config datasource only. It
// never writes: tn_connect.config governs whether this system is enrolled
// with the TrueNAS Connect cloud service, and its real settings must not be
// touched by a plain acceptance test run. No version-gate skip is needed:
// probed live, tn_connect.config is present on BOTH TrueNAS 25.10 and 26.0
// (unlike webshare/lxc_config/container_device, which are 26.0-only).
func TestAccTnConnectConfigDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_tn_connect_config" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_tn_connect_config.test", "id", "tn_connect_config"),
					resource.TestCheckResourceAttrSet("data.truenas_tn_connect_config.test", "enabled"),
					resource.TestCheckResourceAttrSet("data.truenas_tn_connect_config.test", "status"),
					resource.TestCheckResourceAttrSet("data.truenas_tn_connect_config.test", "status_reason"),
					resource.TestCheckResourceAttrSet("data.truenas_tn_connect_config.test", "registration_details"),
				),
			},
		},
	})
}

// TestAccTnConnectConfig_setAndRestore is intentionally skipped
// unconditionally.
//
// DECISIVE PROBE RESULT (both releases, live, 2026-07-23) — the full
// transcript is inline below:
//
//  1. tn_connect.update's own "accepts" schema (core.get_methods), probed
//     against the TrueNAS 26.0 production box (192.168.1.68, this resource's
//     primary target), exposes EXACTLY ONE writable property: "enabled".
//     There is no cosmetic/informational field (no "ips", no "interfaces",
//     nothing) that a Tier 2 set-and-restore test could safely mutate
//     without touching "enabled" on that release — the resource's own
//     safety contract forbids that unconditionally. This alone makes a
//     genuine cosmetic-field Tier 2 test structurally impossible on 26.0,
//     regardless of the box's current enrollment state.
//  2. Separately, and independently decisive on its own: that same 26.0
//     box's tn_connect.config already reports {"enabled": true, "status":
//     "CONFIGURED", "tier": "FOUNDATION", ...} — a real, already-enrolled
//     TrueNAS Connect account, not a disposable test fixture. No update
//     call of any shape was attempted against it; even a would-be no-op
//     payload carries unacceptable risk against a live production
//     enrollment given finding (1) already answers the question.
//  3. TrueNAS 25.10's tn_connect.update accepts a richer schema (also "ips",
//     "interfaces", "use_all_interfaces"), and a supplementary live probe
//     against the disposable 25.10 test box (192.168.1.249, enabled=false)
//     confirmed sending {"ips": ["127.0.0.1"]} alone leaves "enabled" at
//     false, untouched, and round-trips cleanly when restored to its
//     original value ([]). This provider's schema deliberately does NOT
//     expose "ips"/"interfaces"/"use_all_interfaces" as writable at all
//     (see the resource's schema Description), so this finding does not
//     open a path to a Tier 2 test on 25.10 either — there being no
//     writable cosmetic attribute in THIS resource's schema on any
//     release, by design, not just by probed API capability.
//
// Per the design doc's decision rule ("Tier 2 cosmetic toggle ONLY if probe
// proves a field is side-effect-free while enabled=false; else
// documented-skip with probe transcript"), this is a permanent,
// unconditional skip — mirroring truenas_cloud_backup's
// TestAccCloudBackup_basic precedent — rather than a flaky/parameterized
// one. It is never reachable from resource.Test, so it can never call
// tn_connect.update with any payload, let alone {"enabled": true}.
func TestAccTnConnectConfig_setAndRestore(t *testing.T) {
	t.Skip("tn_connect.update accepts ONLY \"enabled\" on TrueNAS 26.0 (probed live via core.get_methods against the production box) — there is no cosmetic field this resource could safely mutate in a Tier 2 set-and-restore test without touching \"enabled\", which this resource's safety contract forbids unconditionally. That same 26.0 box is also already enrolled (enabled=true, tier=FOUNDATION) — a real production TrueNAS Connect account, not a disposable fixture. See the doc comment on TestAccTnConnectConfig_setAndRestore above for the full probe transcript across both TrueNAS 25.10 and 26.0.")
}

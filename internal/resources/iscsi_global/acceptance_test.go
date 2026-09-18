// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_global_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccISCSIGlobalDataSource_basic reads the current TrueNAS iSCSI global
// configuration through the truenas_iscsi_global datasource only. It never
// writes: the iSCSI global configuration serves live storage, and the box's
// real configuration must not be overwritten by an acceptance test run.
//
// Requires:
//   - TF_ACC=1
//   - TRUENAS_API_KEY (or TRUENAS_USERNAME+TRUENAS_PASSWORD) set for
//     acctest.PreCheck
//   - A running TrueNAS instance
func TestAccISCSIGlobalDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_iscsi_global" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_iscsi_global.test", "id", "iscsi_global"),
					resource.TestCheckResourceAttrSet("data.truenas_iscsi_global.test", "basename"),
					resource.TestCheckResourceAttrSet("data.truenas_iscsi_global.test", "listen_port"),
				),
			},
		},
	})
}

// iscsiGlobalOriginal captures the one field
// TestAccISCSIGlobal_setAndRestore touches. PoolAvailThreshold is nullable
// on the wire (nil means "no threshold configured").
type iscsiGlobalOriginal struct {
	PoolAvailThreshold *int64 `json:"pool_avail_threshold"`
}

// readISCSIGlobalOriginal reads the box's current pool_avail_threshold via
// iscsi.global.config, so the test can restore it exactly afterward.
func readISCSIGlobalOriginal(t *testing.T) iscsiGlobalOriginal {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "iscsi.global.config")
	if err != nil {
		t.Fatalf("error reading current iscsi global config: %v", err)
	}
	var orig iscsiGlobalOriginal
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing iscsi.global.config response: %v", err)
	}
	return orig
}

// restoreISCSIGlobal sends only pool_avail_threshold back to its original
// value (nil if it was originally unset) via iscsi.global.update. It runs
// from t.Cleanup, so it restores the box even if the Terraform steps
// themselves fail partway through. basename, listen_port, isns_servers, and
// alua are never touched by this test.
func restoreISCSIGlobal(t *testing.T, orig iscsiGlobalOriginal) {
	t.Helper()
	var v any
	if orig.PoolAvailThreshold != nil {
		v = *orig.PoolAvailThreshold
	}
	if _, err := acctest.Client().Call(context.Background(), "iscsi.global.update", map[string]any{
		"pool_avail_threshold": v,
	}); err != nil {
		t.Fatalf("error restoring iscsi global pool_avail_threshold: %v", err)
	}
}

// TestAccISCSIGlobal_setAndRestore drives the singleton
// truenas_iscsi_global resource's "pool_avail_threshold" field through a
// test value and back to the value read from the box before the test ran,
// then imports it. It requires TF_ACC=1 and TRUENAS_DISRUPTIVE=1
// (acctest.DisruptiveCheck), since it mutates the box's live iSCSI global
// configuration; a t.Cleanup-registered API restore is the safety net if
// the Terraform steps fail.
//
// Safety: pool_avail_threshold is advisory-only — it only controls the
// pool-capacity ALERT threshold used to warn about low free space backing
// iSCSI extents, and setting or clearing it does not affect existing
// targets, extents, or active iSCSI sessions. basename, listen_port,
// isns_servers, and alua (which can disrupt active iSCSI sessions or
// discovery) are never touched.
//
// The model represents a null pool_avail_threshold as the state value 0
// (see responseToModel / updatePayload's three-way handling: explicit 0 in
// config sends JSON nil to the API). So when the original value is nil, the
// terraform-restore step below sets 0, which round-trips back to nil on the
// wire exactly like the box's original unset state.
func TestAccISCSIGlobal_setAndRestore(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readISCSIGlobalOriginal(t)
	t.Cleanup(func() { restoreISCSIGlobal(t, orig) })

	var origValue int64
	if orig.PoolAvailThreshold != nil {
		origValue = *orig.PoolAvailThreshold
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccISCSIGlobalConfig(80),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_global.test", "id", "iscsi_global"),
					resource.TestCheckResourceAttr("truenas_iscsi_global.test", "pool_avail_threshold", "80"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccISCSIGlobalConfig(origValue),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_global.test", "pool_avail_threshold", fmt.Sprintf("%d", origValue)),
				),
			},
			{
				ResourceName:      "truenas_iscsi_global.test",
				ImportState:       true,
				ImportStateId:     "iscsi_global",
				ImportStateVerify: true,
			},
		},
	})
}

func testAccISCSIGlobalConfig(poolAvailThreshold int64) string {
	return fmt.Sprintf(`
resource "truenas_iscsi_global" "test" {
  pool_avail_threshold = %d
}
`, poolAvailThreshold)
}

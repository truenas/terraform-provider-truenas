// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package truecommand_config_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccTrueCommandConfigDataSource_basic reads the current TrueNAS
// TrueCommand configuration through the truenas_truecommand_config
// datasource only. It never writes. No version-gate skip is needed: probed
// live, truecommand.config is present, with an identical shape, on BOTH
// TrueNAS 25.10.4 HA and 26.0.
func TestAccTrueCommandConfigDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_truecommand_config" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_truecommand_config.test", "id", "truecommand_config"),
					resource.TestCheckResourceAttrSet("data.truenas_truecommand_config.test", "enabled"),
					resource.TestCheckResourceAttrSet("data.truenas_truecommand_config.test", "status"),
					resource.TestCheckResourceAttrSet("data.truenas_truecommand_config.test", "status_reason"),
				),
			},
		},
	})
}

// trueCommandConfigOriginal captures the field TestAccTrueCommandConfig_setAndRestore
// touches, as read from truecommand.config before the test runs.
type trueCommandConfigOriginal struct {
	APIKey *string `json:"api_key"`
}

// readTrueCommandConfigOriginal reads the box's current truecommand.config
// via acctest.RestoreCall (job:false, probed live and identical on both
// TrueNAS 25.10.4 HA and 26.0), so the test can restore the exact original
// "api_key" value afterward. Returns ok=false (rather than failing the test
// outright) when the read itself errors, so the caller can self-skip —
// mirroring the webshare_config/mail/ups/lxc_config precedent.
func readTrueCommandConfigOriginal(t *testing.T) (trueCommandConfigOriginal, bool) {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "truecommand.config")
	if err != nil {
		return trueCommandConfigOriginal{}, false
	}
	var orig trueCommandConfigOriginal
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing truecommand.config response: %v", err)
	}
	return orig, true
}

// restoreTrueCommandConfigAPIKey sends "api_key" back to its original value
// (which may be nil/null) via truecommand.update, bypassing HCL's inability
// to express an explicit null distinct from an omitted attribute (see
// model.go's updatePayload doc comment). truecommand.update is job:false
// (probed live, identical on both releases) and accepts a genuine partial
// update — confirmed live: sending only "api_key" leaves "enabled"
// untouched (see model.go's decisive-probe doc comment) — so the plain
// acctest.RestoreCall (which wraps a job-unaware CallRead) is sufficient
// here.
func restoreTrueCommandConfigAPIKey(t *testing.T, apiKey *string) {
	t.Helper()
	if _, err := acctest.RestoreCall(context.Background(), "truecommand.update", map[string]any{
		"api_key": apiKey,
	}); err != nil {
		t.Fatalf("error restoring truecommand api_key: %v", err)
	}
}

// TestAccTrueCommandConfig_setAndRestore drives the singleton
// truenas_truecommand_config resource's "api_key" field through a
// schema-valid throwaway 16-character value, then restores the box's
// original api_key (read before the test ran) directly via
// acctest.RestoreCall in t.Cleanup, then imports it. "enabled" is always
// explicit false in every Terraform step in this test — see the resource's
// schema description and model.go's safety doc comment for why this
// provider's own tests never set it true.
//
// Unlike truenas_tn_connect_config's TestAccTnConnectConfig_setAndRestore
// (a permanent, unconditional skip), this test IS live per the DECISIVE
// probe evidence in model.go's doc comment: with the box's live config at
// enabled=false, an api_key-only truecommand.update call left "enabled"
// untouched, left "status"/"status_reason" at "DISABLED"/"Truecommand
// service is disabled." (no outbound connection attempt), and the new value
// round-tripped verbatim on the next read — confirmed identically on both
// TrueNAS 25.10.4 HA and 26.0 before this test was written. Gated behind
// acctest.DisruptiveCheck (TRUENAS_DISRUPTIVE=1) since it mutates the box's
// live TrueCommand configuration regardless; a t.Cleanup-registered API
// restore is the safety net if the Terraform steps fail.
func TestAccTrueCommandConfig_setAndRestore(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig, ok := readTrueCommandConfigOriginal(t)
	if !ok {
		t.Skip("truecommand.config could not be read on the target box; api_key set-and-restore requires a readable TrueCommand configuration")
	}
	t.Cleanup(func() { restoreTrueCommandConfigAPIKey(t, orig.APIKey) })

	const probeAPIKey = "abcd1234abcd1234" // 16 chars, satisfies the probed minLength/maxLength constraint

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccTrueCommandConfigConfig(probeAPIKey),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_truecommand_config.test", "id", "truecommand_config"),
					resource.TestCheckResourceAttr("truenas_truecommand_config.test", "enabled", "false"),
					resource.TestCheckResourceAttr("truenas_truecommand_config.test", "api_key", probeAPIKey),
					resource.TestCheckResourceAttr("truenas_truecommand_config.test", "status", "DISABLED"),
				),
			},
			{
				ResourceName:      "truenas_truecommand_config.test",
				ImportState:       true,
				ImportStateId:     "truecommand_config",
				ImportStateVerify: true,
			},
		},
	})
}

func testAccTrueCommandConfigConfig(apiKey string) string {
	return `
resource "truenas_truecommand_config" "test" {
  enabled = false
  api_key = "` + apiKey + `"
}
`
}

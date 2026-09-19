// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package failover_config_test

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccFailoverConfigDataSource_basic reads the current failover
// configuration and live HA status through the truenas_failover_config
// datasource only. It never writes. Gated by acctest.HACheck: requires
// TRUENAS_HA=1, TRUENAS_HA_ALLOWED_ENDPOINT matching TRUENAS_ENDPOINT, and
// a live failover.licensed=true probe — see acctest.HACheck's doc comment.
func TestAccFailoverConfigDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.HACheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_failover_config" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_failover_config.test", "id", "failover_config"),
					resource.TestCheckResourceAttrSet("data.truenas_failover_config.test", "disabled"),
					resource.TestCheckResourceAttrSet("data.truenas_failover_config.test", "master"),
					resource.TestCheckResourceAttrSet("data.truenas_failover_config.test", "timeout"),
					resource.TestCheckResourceAttrSet("data.truenas_failover_config.test", "status"),
					resource.TestCheckResourceAttrSet("data.truenas_failover_config.test", "node"),
					resource.TestCheckResourceAttr("data.truenas_failover_config.test", "disabled_reasons.#", "0"),
				),
			},
		},
	})
}

// failoverConfigOriginal captures the field TestAccFailoverConfig_setAndRestore
// touches, as read from failover.config before the test runs.
type failoverConfigOriginal struct {
	Timeout int64 `json:"timeout"`
}

// readFailoverConfigOriginal reads the box's current failover.config via
// acctest.RestoreCall (job:false, probed live on TrueNAS 25.10.4 HA), so the
// test can restore the exact original "timeout" value afterward.
func readFailoverConfigOriginal(t *testing.T) failoverConfigOriginal {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "failover.config")
	if err != nil {
		t.Fatalf("error reading failover.config: %v", err)
	}
	var orig failoverConfigOriginal
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing failover.config response: %v", err)
	}
	return orig
}

// restoreFailoverConfigTimeout sends "timeout" back to its original value
// via failover.update. Probed live: failover.update accepts a genuine
// partial update — sending only "timeout" leaves "disabled"/"master"
// untouched — so the plain acctest.RestoreCall (which wraps a job-unaware
// CallRead) is sufficient here, no CallJob polling needed, and no other
// field needs to be resent.
func restoreFailoverConfigTimeout(t *testing.T, timeout int64) {
	t.Helper()
	if _, err := acctest.RestoreCall(context.Background(), "failover.update", map[string]any{
		"timeout": timeout,
	}); err != nil {
		t.Fatalf("error restoring failover timeout: %v", err)
	}
}

// TestAccFailoverConfig_setAndRestore drives the singleton
// truenas_failover_config resource's "timeout" field (the ONLY field this
// provider's own tests ever touch — see model.go's updatePayload doc
// comment for why "disabled"/"master" are never exercised by a committed
// test) through a different value, then back to the value read from the
// box before the test ran, then imports it.
//
// Gated by acctest.HACheck (TRUENAS_HA=1, endpoint guard, live
// failover.licensed probe) AND acctest.DisruptiveCheck (TRUENAS_DISRUPTIVE=1),
// since it mutates the box's live failover configuration; a
// t.Cleanup-registered API restore is the safety net if the Terraform
// steps fail.
//
// Probed live against the disposable TrueNAS 25.10.4 HA box before this test
// was written (see .superpowers/sdd/task-1-report.md): failover.update
// accepts a partial payload — sending {"timeout": N} alone round-tripped
// cleanly with "disabled"/"master" left exactly as they were, and negative/
// very large timeout values were both accepted with no server-side bound
// enforced, so no client-side range validation is imposed here either — the
// schema matches the API's own permissiveness.
func TestAccFailoverConfig_setAndRestore(t *testing.T) {
	acctest.HACheck(t)
	acctest.DisruptiveCheck(t)

	orig := readFailoverConfigOriginal(t)
	t.Cleanup(func() { restoreFailoverConfigTimeout(t, orig.Timeout) })

	toggled := orig.Timeout + 1

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccFailoverConfigConfig(toggled),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_failover_config.test", "id", "failover_config"),
					resource.TestCheckResourceAttr("truenas_failover_config.test", "timeout", int64Str(toggled)),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccFailoverConfigConfig(orig.Timeout),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_failover_config.test", "timeout", int64Str(orig.Timeout)),
				),
			},
			{
				ResourceName:      "truenas_failover_config.test",
				ImportState:       true,
				ImportStateId:     "failover_config",
				ImportStateVerify: true,
			},
		},
	})
}

func int64Str(v int64) string {
	return strconv.FormatInt(v, 10)
}

func testAccFailoverConfigConfig(timeout int64) string {
	return `
resource "truenas_failover_config" "test" {
  timeout = ` + int64Str(timeout) + `
}
`
}

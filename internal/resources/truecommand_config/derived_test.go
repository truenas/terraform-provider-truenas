// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package truecommand_config_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func restoreTrueCommandOnCleanup(t *testing.T) {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "truecommand.config")
	if err != nil {
		t.Fatalf("read truecommand.config: %v", err)
	}
	var orig struct {
		Enabled bool    `json:"enabled"`
		APIKey  *string `json:"api_key"`
	}
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("parse truecommand.config: %v", err)
	}
	if orig.Enabled {
		t.Skip("TrueCommand is enabled on this system; this test only runs where it is disabled")
	}
	t.Cleanup(func() {
		if _, err := acctest.RestoreCall(context.Background(), "truecommand.update", map[string]any{"enabled": orig.Enabled, "api_key": orig.APIKey}); err != nil {
			t.Errorf("restore truecommand: %v", err)
		}
	})
}

// TestAccTrueCommandConfig_MigrateToWriteOnly guards B-062: moving from
// api_key to the documented write-only api_key_wo used to fail (the plan kept
// the old api_key via UseStateForUnknown while the apply stored null).
func TestAccTrueCommandConfig_MigrateToWriteOnly(t *testing.T) {
	acctest.DisruptiveCheck(t)
	restoreTrueCommandOnCleanup(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: acctest.ProviderConfig() + `
resource "truenas_truecommand_config" "test" {
  enabled = false
  api_key = "abcd1234abcd1234"
}
`},
			{Config: acctest.ProviderConfig() + `
resource "truenas_truecommand_config" "test" {
  enabled            = false
  api_key_wo         = "efgh5678efgh5678"
  api_key_wo_version = 1
}
`},
		},
	})
}

// TestAccTrueCommandConfig_EnableStaleStatus guards B-063: status and
// status_reason are derived from enabled, so toggling enabled must plan them
// unknown instead of carrying "DISABLED" forward. The key is a throwaway
// 16-character string; the cleanup disables TrueCommand and puts the
// original key back.
//
// Enabling makes the box contact iX Portal with the throwaway key (status goes
// to "Pending Confirmation From iX Portal" or FAILED), so the test also needs
// TRUENAS_TEST_TRUECOMMAND_ENABLE=1.
func TestAccTrueCommandConfig_EnableStaleStatus(t *testing.T) {
	acctest.DisruptiveCheck(t)
	if os.Getenv("TRUENAS_TEST_TRUECOMMAND_ENABLE") != "1" {
		t.Skip("set TRUENAS_TEST_TRUECOMMAND_ENABLE=1 to let this test enable TrueCommand with a throwaway key")
	}
	restoreTrueCommandOnCleanup(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: acctest.ProviderConfig() + `
resource "truenas_truecommand_config" "test" {
  enabled = false
  api_key = "abcd1234abcd1234"
}
`},
			{Config: acctest.ProviderConfig() + `
resource "truenas_truecommand_config" "test" {
  enabled = true
  api_key = "abcd1234abcd1234"
}
`},
		},
	})
}

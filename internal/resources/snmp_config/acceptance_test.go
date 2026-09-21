// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snmp_config_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccSNMPConfigDataSource_basic reads the current TrueNAS SNMP
// configuration through the truenas_snmp_config datasource only. It never
// writes: SNMP is a system-critical monitoring singleton, and the box's
// real SNMP configuration must not be overwritten by an acceptance test run.
//
// Requires:
//   - TF_ACC=1
//   - TRUENAS_API_KEY (or TRUENAS_USERNAME+TRUENAS_PASSWORD) set for
//     acctest.PreCheck
//   - A running TrueNAS instance
func TestAccSNMPConfigDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_snmp_config" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_snmp_config.test", "id", "snmp_config"),
					resource.TestCheckResourceAttrSet("data.truenas_snmp_config.test", "community"),
				),
			},
		},
	})
}

// snmpConfigOriginal captures the one field TestAccSNMPConfig_setAndRestore
// touches.
type snmpConfigOriginal struct {
	Location string `json:"location"`
}

// readSNMPConfigOriginal reads the box's current location via snmp.config,
// so the test can restore it exactly afterward.
func readSNMPConfigOriginal(t *testing.T) snmpConfigOriginal {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "snmp.config")
	if err != nil {
		t.Fatalf("error reading current snmp config: %v", err)
	}
	var orig snmpConfigOriginal
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing snmp.config response: %v", err)
	}
	return orig
}

// restoreSNMPConfig sends only location back to its original value via
// snmp.update. It runs from t.Cleanup, so it restores the box even if the
// Terraform steps themselves fail partway through.
func restoreSNMPConfig(t *testing.T, orig snmpConfigOriginal) {
	t.Helper()
	if _, err := acctest.Client().Call(context.Background(), "snmp.update", map[string]any{
		"location": orig.Location,
	}); err != nil {
		t.Fatalf("error restoring snmp location: %v", err)
	}
}

// TestAccSNMPConfig_setAndRestore drives the singleton truenas_snmp_config
// resource's "location" field (a cosmetic, low-risk descriptive string)
// through a test value and back to the value read from the box before the
// test ran, then imports it. It requires TF_ACC=1 and TRUENAS_DISRUPTIVE=1
// (acctest.DisruptiveCheck), since it mutates the box's live SNMP
// configuration; a t.Cleanup-registered API restore is the safety net if the
// Terraform steps fail.
func TestAccSNMPConfig_setAndRestore(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readSNMPConfigOriginal(t)
	t.Cleanup(func() { restoreSNMPConfig(t, orig) })

	testValue := acctest.RandName("tf-acc-location")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccSNMPConfigConfig(testValue),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_snmp_config.test", "id", "snmp_config"),
					resource.TestCheckResourceAttr("truenas_snmp_config.test", "location", testValue),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccSNMPConfigConfig(orig.Location),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_snmp_config.test", "location", orig.Location),
				),
			},
			{
				ResourceName:            "truenas_snmp_config.test",
				ImportState:             true,
				ImportStateId:           "snmp_config",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"v3_password", "v3_privpassphrase"},
			},
		},
	})
}

func testAccSNMPConfigConfig(location string) string {
	return fmt.Sprintf(`
resource "truenas_snmp_config" "test" {
  location = %q
}
`, location)
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_config_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccSMBConfigDataSource_basic reads the current TrueNAS SMB
// configuration through the truenas_smb_config datasource only. It never
// writes: SMB is a system-critical singleton service (existing shares and
// domain membership may depend on it), and the box's real SMB configuration
// must not be overwritten by an acceptance test run.
//
// Requires:
//   - TF_ACC=1
//   - TRUENAS_API_KEY (or TRUENAS_USERNAME+TRUENAS_PASSWORD) set for
//     acctest.PreCheck
//   - A running TrueNAS instance
func TestAccSMBConfigDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_smb_config" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_smb_config.test", "id", "smb_config"),
					resource.TestCheckResourceAttrSet("data.truenas_smb_config.test", "netbiosname"),
					resource.TestCheckResourceAttrSet("data.truenas_smb_config.test", "server_sid"),
				),
			},
		},
	})
}

// smbConfigOriginal captures the one field TestAccSMBConfig_setAndRestore
// touches.
type smbConfigOriginal struct {
	Description string `json:"description"`
}

// readSMBConfigOriginal reads the box's current description via
// acctest.RestoreCall (smb.update is job:false, probed live), so the test
// can restore it exactly afterward.
func readSMBConfigOriginal(t *testing.T) smbConfigOriginal {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "smb.config")
	if err != nil {
		t.Fatalf("error reading current smb config: %v", err)
	}
	var orig smbConfigOriginal
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing smb.config response: %v", err)
	}
	return orig
}

// restoreSMBConfig sends only description back to its original value via
// smb.update, through acctest.RestoreCall so a t.Cleanup-time stale
// connection on the long-lived acctest.Client() (see RestoreCall's doc
// comment) is retried and reconnected rather than failing the whole test
// with "not connected". It runs from t.Cleanup, so it restores the box
// even if the Terraform steps themselves fail partway through. workgroup,
// netbiosname, minimum_protocol, bindip, and admin_group are never touched
// by this test.
func restoreSMBConfig(t *testing.T, orig smbConfigOriginal) {
	t.Helper()
	if _, err := acctest.RestoreCall(context.Background(), "smb.update", map[string]any{
		"description": orig.Description,
	}); err != nil {
		t.Fatalf("error restoring smb description: %v", err)
	}
}

// TestAccSMBConfig_setAndRestore drives the singleton truenas_smb_config
// resource's "description" field (an inert, cosmetic label) through a test
// value and back to the value read from the box before the test ran, then
// imports it. It requires TF_ACC=1 and TRUENAS_DISRUPTIVE=1
// (acctest.DisruptiveCheck), since it mutates the box's live SMB
// configuration; a t.Cleanup-registered API restore is the safety net if the
// Terraform steps fail. workgroup, netbiosname, minimum_protocol, bindip,
// and admin_group are never touched: changing those on a live system can
// disrupt SMB clients or domain membership.
func TestAccSMBConfig_setAndRestore(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readSMBConfigOriginal(t)
	t.Cleanup(func() { restoreSMBConfig(t, orig) })

	testValue := acctest.RandName("tf-acc-description")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccSMBConfigConfig(testValue),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_smb_config.test", "id", "smb_config"),
					resource.TestCheckResourceAttr("truenas_smb_config.test", "description", testValue),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccSMBConfigConfig(orig.Description),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_smb_config.test", "description", orig.Description),
				),
			},
			{
				ResourceName:      "truenas_smb_config.test",
				ImportState:       true,
				ImportStateId:     "smb_config",
				ImportStateVerify: true,
			},
		},
	})
}

func testAccSMBConfigConfig(description string) string {
	return fmt.Sprintf(`
resource "truenas_smb_config" "test" {
  description = %q
}
`, description)
}

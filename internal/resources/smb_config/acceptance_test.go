package smb_config_test

import (
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
//   - A running TrueNAS SCALE instance
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

// TestAccSMBConfig_basic is intentionally skipped by default.
// truenas_smb_config is a SINGLETON resource that manages a system-critical
// service: bringing it under Terraform management mutates the box's actual
// SMB configuration (smb.update), and a naive create/update/destroy
// acceptance test risks disrupting existing SMB shares or domain membership
// on whatever system runs it (e.g. by rewriting workgroup, netbiosname, or
// minimum_protocol).
//
// If this test is ever enabled against a disposable/non-production TrueNAS
// instance, it should:
//  1. Read the current config via the datasource first.
//  2. Only touch "description" (a low-risk, additive string field) in the
//     resource config, driving it through a value and back to the value the
//     datasource observed in step 1, so the net effect on the box is a
//     no-op. Never touch workgroup, netbiosname, minimum_protocol, bindip,
//     or admin_group in an automated test: changing those on a live system
//     can disrupt SMB clients or domain membership.
//  3. Use ImportState with ImportStateId "smb_config" to verify import
//     normalizes any ID to the fixed singleton ID.
func TestAccSMBConfig_basic(t *testing.T) {
	t.Skip("truenas_smb_config manages a system-critical service; skipped to avoid mutating the target box's SMB configuration and risking disruption of existing shares or domain membership. See comment on TestAccSMBConfig_basic for how to safely enable this against a disposable instance.")
}

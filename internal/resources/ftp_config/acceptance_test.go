package ftp_config_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccFTPConfigDataSource_basic reads the current TrueNAS FTP
// configuration through the truenas_ftp_config datasource only. It never
// writes: FTP is a singleton service, and the box's real FTP configuration
// must not be overwritten by an acceptance test run.
//
// Requires:
//   - TF_ACC=1
//   - TRUENAS_API_KEY (or TRUENAS_USERNAME+TRUENAS_PASSWORD) set for
//     acctest.PreCheck
//   - A running TrueNAS SCALE instance
func TestAccFTPConfigDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_ftp_config" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_ftp_config.test", "id", "ftp_config"),
					resource.TestCheckResourceAttrSet("data.truenas_ftp_config.test", "port"),
				),
			},
		},
	})
}

// TestAccFTPConfig_basic is intentionally skipped by default.
// truenas_ftp_config is a SINGLETON resource: bringing it under Terraform
// management mutates the box's actual FTP configuration (ftp.update), and a
// naive create/update/destroy acceptance test risks disrupting FTP service
// for whatever system runs it.
//
// If this test is ever enabled against a disposable/non-production TrueNAS
// instance, it should:
//  1. Read the current config via the datasource first.
//  2. Only touch a low-risk, additive field (e.g. "banner" or "options") in
//     the resource config, driving it through a value and back to the value
//     the datasource observed in step 1, so the net effect on the box is a
//     no-op. Never touch port, defaultroot, onlyanonymous, or onlylocal in
//     an automated test: changing those on a live system can disrupt FTP
//     access.
//  3. Use ImportState with ImportStateId "ftp_config" to verify import
//     normalizes any ID to the fixed singleton ID.
func TestAccFTPConfig_basic(t *testing.T) {
	t.Skip("truenas_ftp_config manages a singleton service configuration; skipped to avoid mutating the target box's FTP configuration. See comment on TestAccFTPConfig_basic for how to safely enable this against a disposable instance.")
}

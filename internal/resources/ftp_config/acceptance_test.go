// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ftp_config_test

import (
	"context"
	"encoding/json"
	"fmt"
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
//   - A running TrueNAS instance
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

// ftpConfigOriginal captures the one field TestAccFTPConfig_setAndRestore
// touches.
type ftpConfigOriginal struct {
	Banner string `json:"banner"`
}

// readFTPConfigOriginal reads the box's current banner via ftp.config, so
// the test can restore it exactly afterward.
func readFTPConfigOriginal(t *testing.T) ftpConfigOriginal {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "ftp.config")
	if err != nil {
		t.Fatalf("error reading current ftp config: %v", err)
	}
	var orig ftpConfigOriginal
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing ftp.config response: %v", err)
	}
	return orig
}

// restoreFTPConfig sends only banner back to its original value via
// ftp.update. It runs from t.Cleanup, so it restores the box even if the
// Terraform steps themselves fail partway through. port, defaultroot,
// onlyanonymous, and onlylocal are never touched by this test.
func restoreFTPConfig(t *testing.T, orig ftpConfigOriginal) {
	t.Helper()
	if _, err := acctest.Client().Call(context.Background(), "ftp.update", map[string]any{
		"banner": orig.Banner,
	}); err != nil {
		t.Fatalf("error restoring ftp banner: %v", err)
	}
}

// TestAccFTPConfig_setAndRestore drives the singleton truenas_ftp_config
// resource's "banner" field (a cosmetic, low-risk login banner string)
// through a test value and back to the value read from the box before the
// test ran, then imports it. It requires TF_ACC=1 and TRUENAS_DISRUPTIVE=1
// (acctest.DisruptiveCheck), since it mutates the box's live FTP
// configuration; a t.Cleanup-registered API restore is the safety net if the
// Terraform steps fail. port, defaultroot, onlyanonymous, and onlylocal are
// never touched: changing those on a live system can disrupt FTP access.
func TestAccFTPConfig_setAndRestore(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readFTPConfigOriginal(t)
	t.Cleanup(func() { restoreFTPConfig(t, orig) })

	testValue := acctest.RandName("tf-acc-banner")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccFTPConfigConfig(testValue),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_ftp_config.test", "id", "ftp_config"),
					resource.TestCheckResourceAttr("truenas_ftp_config.test", "banner", testValue),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccFTPConfigConfig(orig.Banner),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_ftp_config.test", "banner", orig.Banner),
				),
			},
			{
				ResourceName:      "truenas_ftp_config.test",
				ImportState:       true,
				ImportStateId:     "ftp_config",
				ImportStateVerify: true,
			},
		},
	})
}

func testAccFTPConfigConfig(banner string) string {
	return fmt.Sprintf(`
resource "truenas_ftp_config" "test" {
  banner = %q
}
`, banner)
}

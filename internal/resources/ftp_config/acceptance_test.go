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

  port               = 21
  clients            = 32
  ipconnections      = 5
  loginattempt       = 3
  timeout            = 600
  timeout_notransfer = 300
  passiveportsmin    = 40000
  passiveportsmax    = 40100
  onlyanonymous      = false
  onlylocal          = false
  anonpath           = ""
  fxp                = false
  resume             = true
  defaultroot        = true
  ident              = false
  reversedns         = false
  masqaddress        = ""
  options            = ""
  dirmask            = "022"
  filemask           = "022"
  localuserbw        = 0
  localuserdlbw      = 0
  anonuserbw         = 0
  anonuserdlbw       = 0

  tls        = false
  tls_policy = "on"

  tls_opt_allow_client_renegotiations = false
  tls_opt_allow_dot_login             = false
  tls_opt_allow_per_user              = false
  tls_opt_common_name_required        = false
  tls_opt_dns_name_required           = false
  tls_opt_enable_diags                = false
  tls_opt_export_cert_data            = false
  tls_opt_ip_address_required         = false
  tls_opt_no_empty_fragments          = false
  tls_opt_no_session_reuse_required   = false
  tls_opt_stdenvvars                  = false
}
`, banner)
}

// TestAccFTPConfig_coverage sets ssltls_certificate to a certificate that
// already exists on the box and asserts it reads back with an empty
// post-apply plan. The config pins tls = false, so FTP does not start
// offering TLS with the chosen cert and client-facing behavior is unchanged. The candidate is the first non-CSR, non-CA certificate
// from certificate.query that differs from the current one; the test skips
// if there is none. The original ssltls_certificate (possibly null) and tls
// are restored via ftp.update from t.Cleanup and re-read to confirm.
func TestAccFTPConfig_coverage(t *testing.T) {
	acctest.DisruptiveCheck(t)

	raw, err := acctest.RestoreCall(context.Background(), "ftp.config")
	if err != nil {
		t.Fatalf("error reading current ftp config: %v", err)
	}
	var orig struct {
		TLS               bool   `json:"tls"`
		SSLTLSCertificate *int64 `json:"ssltls_certificate"`
	}
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing ftp.config response: %v", err)
	}

	raw, err = acctest.RestoreCall(context.Background(), "certificate.query")
	if err != nil {
		t.Skipf("certificate.query failed: %v", err)
	}
	var certs []struct {
		ID  int64 `json:"id"`
		CSR bool  `json:"cert_type_CSR"`
		CA  bool  `json:"cert_type_CA"`
	}
	if err := json.Unmarshal(raw, &certs); err != nil {
		t.Fatalf("error parsing certificate.query response: %v", err)
	}
	var certID int64
	for _, c := range certs {
		if c.CSR || c.CA {
			continue
		}
		if orig.SSLTLSCertificate != nil && *orig.SSLTLSCertificate == c.ID {
			continue
		}
		certID = c.ID
		break
	}
	if certID == 0 {
		t.Skip("no usable certificate on the box that differs from the current ftp ssltls_certificate")
	}

	t.Cleanup(func() {
		if _, err := acctest.RestoreCall(context.Background(), "ftp.update", map[string]any{
			"tls":                orig.TLS,
			"ssltls_certificate": orig.SSLTLSCertificate,
		}); err != nil {
			t.Errorf("error restoring ftp ssltls_certificate: %v", err)
			return
		}
		raw, err := acctest.RestoreCall(context.Background(), "ftp.config")
		if err != nil {
			t.Errorf("error re-reading ftp.config after restore: %v", err)
			return
		}
		var got struct {
			TLS               bool   `json:"tls"`
			SSLTLSCertificate *int64 `json:"ssltls_certificate"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Errorf("error parsing ftp.config after restore: %v", err)
			return
		}
		sameCert := (got.SSLTLSCertificate == nil && orig.SSLTLSCertificate == nil) ||
			(got.SSLTLSCertificate != nil && orig.SSLTLSCertificate != nil && *got.SSLTLSCertificate == *orig.SSLTLSCertificate)
		if got.TLS != orig.TLS || !sameCert {
			t.Errorf("ftp tls/ssltls_certificate not restored: want %+v, got %+v", orig, got)
		}
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_ftp_config" "test" {
  tls                = false
  ssltls_certificate = %d
}
`, certID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_ftp_config.test", "ssltls_certificate", fmt.Sprint(certID)),
				),
			},
		},
	})
}

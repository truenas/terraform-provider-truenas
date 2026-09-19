// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_config_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccKerberosConfigDataSource_basic reads the current TrueNAS Kerberos
// configuration through the truenas_kerberos_config datasource only. It
// never writes: the Kerberos configuration is system-wide directory-service
// configuration, and the box's real krb5.conf aux settings must not be
// overwritten by an acceptance test run.
func TestAccKerberosConfigDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_kerberos_config" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_kerberos_config.test", "id", "kerberos_config"),
				),
			},
		},
	})
}

// kerberosConfigOriginal captures the fields
// TestAccKerberosConfig_setAndRestore touches.
type kerberosConfigOriginal struct {
	AppdefaultsAux string `json:"appdefaults_aux"`
	LibdefaultsAux string `json:"libdefaults_aux"`
}

// readKerberosConfigOriginal reads the box's current appdefaults_aux/
// libdefaults_aux via kerberos.config, so the test can restore them exactly
// afterward.
func readKerberosConfigOriginal(t *testing.T) kerberosConfigOriginal {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "kerberos.config")
	if err != nil {
		t.Fatalf("error reading current Kerberos config: %v", err)
	}
	var orig kerberosConfigOriginal
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing kerberos.config response: %v", err)
	}
	return orig
}

// restoreKerberosConfig sends appdefaults_aux/libdefaults_aux back to their
// original values via kerberos.update. It runs from t.Cleanup, so it
// restores the box even if the Terraform steps themselves fail partway
// through.
func restoreKerberosConfig(t *testing.T, orig kerberosConfigOriginal) {
	t.Helper()
	if _, err := acctest.Client().Call(context.Background(), "kerberos.update", map[string]any{
		"appdefaults_aux": orig.AppdefaultsAux,
		"libdefaults_aux": orig.LibdefaultsAux,
	}); err != nil {
		t.Fatalf("error restoring Kerberos config: %v", err)
	}
}

// TestAccKerberosConfig_setAndRestore drives the singleton
// truenas_kerberos_config resource's "appdefaults_aux" field through a
// marker value and back to the value read from the box before the test
// ran, then imports it. It requires TF_ACC=1 and TRUENAS_DISRUPTIVE=1
// (acctest.DisruptiveCheck), since it mutates the box's live Kerberos
// configuration; a t.Cleanup-registered API restore is the safety net if
// the Terraform steps fail.
//
// The marker MUST be a recognized MIT krb5.conf [appdefaults] "key = value"
// line, not an arbitrary comment string: kerberos.update parses
// appdefaults_aux server-side and (a) crashes with a raw "list index out of
// range" APIError on any line that doesn't split on "=" (e.g. a bare "#
// comment"), and (b) cleanly rejects (EINVAL) any key it doesn't recognize.
// Both were confirmed live against a TrueNAS 25.10 box — see
// task-5-report.md. "no_addresses" is a real, harmless appdefaults key
// (whether Kerberos ignores IP addresses in service principal names), so
// this test toggles it, mirroring resilver_config's boolean-toggle
// set-and-restore pattern instead of using a free-form marker string.
func TestAccKerberosConfig_setAndRestore(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readKerberosConfigOriginal(t)
	t.Cleanup(func() { restoreKerberosConfig(t, orig) })

	marker := "no_addresses = true"
	if orig.AppdefaultsAux == marker {
		marker = "no_addresses = false"
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccKerberosConfigConfig(marker),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_kerberos_config.test", "id", "kerberos_config"),
					resource.TestCheckResourceAttr("truenas_kerberos_config.test", "appdefaults_aux", marker),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccKerberosConfigConfig(orig.AppdefaultsAux),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_kerberos_config.test", "appdefaults_aux", orig.AppdefaultsAux),
				),
			},
			{
				ResourceName:      "truenas_kerberos_config.test",
				ImportState:       true,
				ImportStateId:     "kerberos_config",
				ImportStateVerify: true,
			},
		},
	})
}

func testAccKerberosConfigConfig(appdefaultsAux string) string {
	return fmt.Sprintf(`
resource "truenas_kerberos_config" "test" {
  appdefaults_aux = %q
}
`, appdefaultsAux)
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package certificate_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func csrConfig(name, extra string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_certificate" "test" {
  name        = %q
  create_type = "CERTIFICATE_CREATE_CSR"
  key_type    = "RSA"
  key_length  = 2048
  common      = "tffix3-csr.example.com"
%s
}
`, name, extra)
}

func expectEmptyPlan() resource.ConfigPlanChecks {
	return resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
}

// TestAccCertificate_csrSanForms: the server reports IP SANs as
// "IP Address:<ip>" and names as "DNS:<name>"; a bare IP or an "IP:"/"DNS:"
// prefixed entry failed the apply (B-060). (No import step: a CSR's create_type
// reads back null on import and is recorded by one in-place update, by design.)
func TestAccCertificate_csrSanForms(t *testing.T) {
	name := acctest.RandName("tffix3shtasks-san")
	cfg := csrConfig(name, `  san = ["tffix3-csr.example.com", "10.0.0.1", "IP:10.0.0.2", "DNS:alt.tffix3.example.com", "2001:db8::7"]`)
	n := "truenas_certificate.test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCertificateDestroyed(name, ""),
		Steps: []resource.TestStep{
			{Config: cfg, Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(n, "san.1", "10.0.0.1"),
				resource.TestCheckResourceAttr(n, "san.4", "2001:db8::7"),
			)},
			{Config: cfg, ConfigPlanChecks: expectEmptyPlan()},
		},
	})
}

// TestAccCertificate_csrEmailCase: the server lower-cases the e-mail domain;
// an upper-case domain failed the apply (B-127).
func TestAccCertificate_csrEmailCase(t *testing.T) {
	name := acctest.RandName("tffix3shtasks-email")
	cfg := csrConfig(name, `
  san   = ["tffix3-csr.example.com"]
  email = "Admin@Example.COM"
`)
	n := "truenas_certificate.test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCertificateDestroyed(name, ""),
		Steps: []resource.TestStep{
			{Config: cfg, Check: resource.TestCheckResourceAttr(n, "email", "Admin@Example.COM")},
			{Config: cfg, ConfigPlanChecks: expectEmptyPlan()},
		},
	})
}

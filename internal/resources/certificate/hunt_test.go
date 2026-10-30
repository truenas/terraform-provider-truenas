// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package certificate_test

// Adversarial bug-hunt tests: drive the two lab-safe create paths with their
// full attribute surface and assert the read-back re-plans empty, then
// import. Key material is generated in-test (genSelfSigned, defined in the
// acceptance test). The ACME path is not covered here: it needs the gated
// TRUENAS_ACME infrastructure and is exercised by the acceptance suite.

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// Imported self-signed cert + key, with add_to_trusted_store turned on (the
// one in-place-updatable flag for an IMPORTED entry).
func TestHuntCertificate_allFieldsImported(t *testing.T) {
	acctest.DisruptiveCheck(t)

	name := acctest.RandName("tfhunt-cert-imported")
	certPEM, keyPEM := genSelfSigned(t, "tfhunt-imported.example.com")

	cfg := acctest.ProviderConfig() + testHuntCertificateImportedConfig(name, certPEM, keyPEM)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCertificateDestroyed(name, ""),
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:            "truenas_certificate.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"passphrase"},
			},
		},
	})
}

func testHuntCertificateImportedConfig(name, certPEM, keyPEM string) string {
	return fmt.Sprintf(`
resource "truenas_certificate" "test" {
  name                 = %q
  create_type          = "CERTIFICATE_CREATE_IMPORTED"
  certificate          = %q
  privatekey           = %q
  add_to_trusted_store = true
}
`, name, certPEM, keyPEM)
}

// Server-generated RSA CSR with every distinguished-name field, a non-default
// digest algorithm, a key passphrase, and a multi-entry SAN.
func TestHuntCertificate_allFieldsCSR(t *testing.T) {
	acctest.DisruptiveCheck(t)

	name := acctest.RandName("tfhunt-cert-csr")

	cfg := acctest.ProviderConfig() + testHuntCertificateCSRConfig(name)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCertificateDestroyed(name, ""),
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:      "truenas_certificate.test",
				ImportState:       true,
				ImportStateVerify: true,
				// create_type cannot be told apart from an imported CSR on
				// import; passphrase is write-only by convention; the API does
				// not round-trip add_to_trusted_store for a CSR-only entry.
				// digest_algorithm comes back null for a CSR, but the provider
				// derives it from the CSR's signature algorithm (F1).
				ImportStateVerifyIgnore: []string{"create_type", "passphrase", "add_to_trusted_store"},
			},
		},
	})
}

func testHuntCertificateCSRConfig(name string) string {
	return fmt.Sprintf(`
resource "truenas_certificate" "test" {
  name                 = %q
  create_type          = "CERTIFICATE_CREATE_CSR"
  key_type             = "RSA"
  key_length           = 2048
  digest_algorithm     = "SHA512"
  common               = "tfhunt-csr.example.com"
  san                  = ["tfhunt-csr.example.com", "alt.tfhunt-csr.example.com"]
  city                 = "Testville"
  state                = "California"
  country              = "US"
  organization         = "TF Hunt Org"
  organizational_unit  = "QA"
  email                = "csr@example.com"
  add_to_trusted_store = false
  passphrase           = "hunt-key-passphrase"
}
`, name)
}

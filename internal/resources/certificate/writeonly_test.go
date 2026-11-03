// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package certificate_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// The name carries the version: TrueNAS requires certificate names to be
// unique, so with create_before_destroy the replacement must not reuse the
// old name while the old certificate still exists.
func testAccCertificateWriteOnlyConfig(name, certPEM, keyPEM string, version int) string {
	return fmt.Sprintf(`
resource "truenas_certificate" "test" {
  name                   = "%s-v%d"
  create_type            = "CERTIFICATE_CREATE_IMPORTED"
  certificate_wo         = %q
  privatekey_wo          = %q
  certificate_wo_version = %d

  lifecycle {
    create_before_destroy = true
  }
}
`, name, version, certPEM, keyPEM, version)
}

// TestAccCertificate_importedWriteOnly (#65): certificate_wo / privatekey_wo
// import a certificate without the key ever reaching state; the public PEM is
// still read back; a version bump rotates by replacement (create before
// destroy) and the key stays out of state across it; an existing certificate
// imports into a write-only configuration with a one-shot in-place record of
// the version.
func TestAccCertificate_importedWriteOnly(t *testing.T) {
	name := acctest.RandName("tf-acc-cert-wo")
	cert1, key1 := genSelfSigned(t, "tf-acc-wo-1.example.com")
	cert2, key2 := genSelfSigned(t, "tf-acc-wo-2.example.com")
	var firstID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		CheckDestroy:             testAccCheckCertificateDestroyed(name+"-v1", name+"-v2"),
		Steps: append([]resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccCertificateWriteOnlyConfig(name, cert1, key1, 1),
				Check: resource.ComposeTestCheckFunc(
					func(s *terraform.State) error {
						firstID = s.RootModule().Resources["truenas_certificate.test"].Primary.ID
						return nil
					},
					resource.TestCheckResourceAttr("truenas_certificate.test", "certificate", cert1),
					resource.TestCheckNoResourceAttr("truenas_certificate.test", "privatekey"),
					resource.TestCheckNoResourceAttr("truenas_certificate.test", "certificate_wo"),
					resource.TestCheckNoResourceAttr("truenas_certificate.test", "privatekey_wo"),
					resource.TestCheckResourceAttr("truenas_certificate.test", "certificate_wo_version", "1"),
					resource.TestCheckResourceAttrSet("truenas_certificate.test", "privatekey_path"),
					resource.TestCheckResourceAttrSet("truenas_certificate.test", "fingerprint"),
				),
			},
			{
				Config:           acctest.ProviderConfig() + testAccCertificateWriteOnlyConfig(name, cert1, key1, 1),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			// rotate: new material + version bump -> replace (create before destroy)
			{
				Config: acctest.ProviderConfig() + testAccCertificateWriteOnlyConfig(name, cert2, key2, 2),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_certificate.test", plancheck.ResourceActionReplace),
				}},
				Check: resource.ComposeTestCheckFunc(
					func(s *testing.T) resource.TestCheckFunc {
						return func(st *terraform.State) error {
							if id := st.RootModule().Resources["truenas_certificate.test"].Primary.ID; id == firstID {
								return fmt.Errorf("rotation kept id %s: the certificate was not replaced", id)
							}
							return nil
						}
					}(t),
					resource.TestCheckResourceAttr("truenas_certificate.test", "certificate", cert2),
					resource.TestCheckNoResourceAttr("truenas_certificate.test", "privatekey"),
					resource.TestCheckResourceAttr("truenas_certificate.test", "certificate_wo_version", "2"),
				),
			},
			{
				ResourceName:      "truenas_certificate.test",
				ImportState:       true,
				ImportStateVerify: true,
				// write-only inputs and their trigger are never read back;
				// create_type is derived; passphrase is never echoed. An import
				// cannot know the configuration is write-only, so it reads the
				// key into privatekey; the first apply (which records the
				// version) drops it again.
				ImportStateVerifyIgnore: []string{"certificate_wo", "privatekey_wo", "certificate_wo_version", "passphrase", "privatekey"},
			},
			// import then plan: the version is recorded in place and the key the
			// import read back is dropped from state; never a replacement
		}, acctest.ImportReapplyNoop("truenas_certificate.test",
			acctest.ProviderConfig()+testAccCertificateWriteOnlyConfig(name, cert2, key2, 2), "certificate_wo_version", "privatekey")...),
	})
}

// The write-only inputs are refused at plan time for create types that do not
// take them.
func TestAccCertificate_writeOnlyWrongCreateType(t *testing.T) {
	_, key := genSelfSigned(t, "tf-acc-wo-bad.example.com")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		Steps: []resource.TestStep{{
			Config: acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_certificate" "test" {
  name                   = %q
  create_type            = "CERTIFICATE_CREATE_CSR"
  key_type               = "RSA"
  key_length             = 2048
  common                 = "x.example.com"
  san                    = ["x.example.com"]
  privatekey_wo          = %q
  certificate_wo_version = 1
}
`, acctest.RandName("tf-acc-cert-wo-bad"), key),
			ExpectError: regexp.MustCompile(`privatekey_wo is only valid for create_type`),
		}},
	})
}

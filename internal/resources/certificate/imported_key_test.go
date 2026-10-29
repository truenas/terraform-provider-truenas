// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package certificate_test

import (
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TrueNAS re-encodes imported keys (PKCS#1/SEC1 to PKCS#8, decrypted,
// trailing newline added). Each of these failed apply with "inconsistent
// result" and left the certificate tainted (B-013); the configured key text
// must now stay in state with an empty re-plan.

// importedKeySteps applies cfg, checks the configured key text is in state,
// and re-plans expecting no change.
func importedKeySteps(cfg, keyPEM string) []resource.TestStep {
	return []resource.TestStep{
		{
			Config: cfg,
			Check:  resource.TestCheckResourceAttr("truenas_certificate.test", "privatekey", keyPEM),
		},
		{
			Config: cfg,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
			},
		},
	}
}

// PKCS#1 RSA key (tls_private_key.private_key_pem for RSA).
func TestAccCertificate_ImportedPKCS1Key(t *testing.T) {
	name := acctest.RandName("tffix-cert-pkcs1")
	certPEM, keyPEM := selfSignedCertPEM(t, "tffix-pkcs1.example.com", "pkcs1")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCertificateDestroyed(name, ""),
		Steps:                    importedKeySteps(importedCertConfig(name, certPEM, keyPEM), keyPEM),
	})
}

// SEC1 EC key ("EC PRIVATE KEY", tls_private_key ECDSA / openssl ecparam).
func TestAccCertificate_ImportedSEC1Key(t *testing.T) {
	name := acctest.RandName("tffix-cert-sec1")
	certPEM, keyPEM := selfSignedCertPEM(t, "tffix-sec1.example.com", "sec1")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCertificateDestroyed(name, ""),
		Steps:                    importedKeySteps(importedCertConfig(name, certPEM, keyPEM), keyPEM),
	})
}

// PKCS#8 key without the trailing newline (e.g. trimspace(file(...)) or a
// heredoc). The server appends "\n".
func TestAccCertificate_ImportedKeyNoTrailingNewline(t *testing.T) {
	name := acctest.RandName("tffix-cert-nonl")
	certPEM, keyPEM := selfSignedCertPEM(t, "tffix-nonl.example.com", "pkcs8")
	keyPEM = keyPEM[:len(keyPEM)-1]
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCertificateDestroyed(name, ""),
		Steps:                    importedKeySteps(importedCertConfig(name, certPEM, keyPEM), keyPEM),
	})
}

// Passphrase-protected key: the server stores it decrypted.
func TestAccCertificate_ImportedEncryptedKey(t *testing.T) {
	name := acctest.RandName("tffix-cert-enc")
	certPEM, keyPEM := selfSignedCertPEM(t, "tffix-enc.example.com", "pkcs1")
	block, _ := pem.Decode([]byte(keyPEM))
	//nolint:staticcheck // legacy PEM encryption is what the test needs: the stdlib has no PKCS#8 encryption.
	enc, err := x509.EncryptPEMBlock(rand.Reader, block.Type, block.Bytes, []byte("tffix-pass"), x509.PEMCipherAES256)
	if err != nil {
		t.Fatal(err)
	}
	encPEM := string(pem.EncodeToMemory(enc))
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_certificate" "test" {
  name        = %q
  create_type = "CERTIFICATE_CREATE_IMPORTED"
  certificate = %q
  privatekey  = %q
  passphrase  = "tffix-pass"
}
`, name, certPEM, encPEM)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCertificateDestroyed(name, ""),
		Steps:                    importedKeySteps(cfg, encPEM),
	})
}

// Import of a certificate whose configured key is PKCS#1: import reads the
// server's PKCS#8 text, and the next apply must record the configured text in
// place, not replace the certificate.
func TestAccCertificate_ImportedPKCS1KeyImportThenPlan(t *testing.T) {
	name := acctest.RandName("tffix-cert-pkcs1imp")
	certPEM, keyPEM := selfSignedCertPEM(t, "tffix-pkcs1imp.example.com", "pkcs1")
	cfg := importedCertConfig(name, certPEM, keyPEM)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCertificateDestroyed(name, ""),
		Steps:                    acctest.ImportReapplyNoop("truenas_certificate.test", cfg, "privatekey"),
	})
}

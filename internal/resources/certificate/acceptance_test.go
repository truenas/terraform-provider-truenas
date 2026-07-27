// Copyright (c) iXsystems, Inc.
// SPDX-License-Identifier: MPL-2.0

package certificate_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"regexp"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// genSelfSigned generates an RSA 2048 self-signed certificate + PKCS#8
// private key entirely in Go (no external tooling), for the
// CERTIFICATE_CREATE_IMPORTED acceptance path. RSA 2048 was confirmed live
// against TrueNAS 25.10 as an accepted import (certificate.create
// with create_type=CERTIFICATE_CREATE_IMPORTED and this exact PEM shape
// round-tripped cleanly).
func genSelfSigned(t *testing.T, cn string) (certPEM, keyPEM string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		DNSNames:     []string{cn},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("creating self-signed certificate: %v", err)
	}
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))

	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshaling private key: %v", err)
	}
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))

	return certPEM, keyPEM
}

// TestAccCertificate_imported exercises the full Tier 1 contract for
// create_type=CERTIFICATE_CREATE_IMPORTED: import a self-signed RSA 2048
// cert+key generated in-test, verify the read-back fields, rename in place
// (the one field certificate.update accepts besides renew_days/
// add_to_trusted_store, confirmed live), import by id, and verify
// destruction via a live certificate.query. Never touches certificate id 1
// (the box's live UI certificate) — this test only ever creates new
// certificates with random tf-acc-* names.
func TestAccCertificate_imported(t *testing.T) {
	name := acctest.RandName("tf-acc-cert-imported")
	renamed := acctest.RandName("tf-acc-cert-imported-renamed")
	certPEM, keyPEM := genSelfSigned(t, "tf-acc-imported.example.com")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCertificateDestroyed(name, renamed),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccCertificateImportedConfig(name, certPEM, keyPEM),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_certificate.test", "id"),
					resource.TestCheckResourceAttr("truenas_certificate.test", "name", name),
					resource.TestCheckResourceAttr("truenas_certificate.test", "create_type", "CERTIFICATE_CREATE_IMPORTED"),
					resource.TestCheckResourceAttr("truenas_certificate.test", "certificate", certPEM),
					resource.TestCheckResourceAttr("truenas_certificate.test", "privatekey", keyPEM),
					resource.TestCheckResourceAttr("truenas_certificate.test", "key_type", "RSA"),
					resource.TestCheckResourceAttr("truenas_certificate.test", "key_length", "2048"),
					resource.TestCheckResourceAttrSet("truenas_certificate.test", "fingerprint"),
					resource.TestCheckResourceAttr("truenas_certificate.test", "expired", "false"),
					resource.TestCheckResourceAttrSet("truenas_certificate.test", "certificate_path"),
					resource.TestCheckResourceAttrSet("truenas_certificate.test", "privatekey_path"),
				),
			},
			// Update in place: rename (the one field certificate.update
			// accepts, confirmed live). Cert/key material is unchanged.
			{
				Config: acctest.ProviderConfig() + testAccCertificateImportedConfig(renamed, certPEM, keyPEM),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_certificate.test", "name", renamed),
					resource.TestCheckResourceAttr("truenas_certificate.test", "certificate", certPEM),
				),
			},
			{
				ResourceName:      "truenas_certificate.test",
				ImportState:       true,
				ImportStateVerify: true,
				// "create_type" is never returned by certificate.query/
				// get_instance (probed live) and cannot be recovered on
				// import; "passphrase" is accepted on create but never
				// echoed back by any read method either.
				ImportStateVerifyIgnore: []string{"create_type", "passphrase"},
			},
		},
	})
}

func testAccCertificateImportedConfig(name, certPEM, keyPEM string) string {
	return fmt.Sprintf(`
resource "truenas_certificate" "test" {
  name        = %q
  create_type = "CERTIFICATE_CREATE_IMPORTED"
  certificate = %q
  privatekey  = %q
}
`, name, certPEM, keyPEM)
}

// TestAccCertificate_csr exercises the CERTIFICATE_CREATE_CSR path: TrueNAS
// generates a new RSA 2048 key pair and CSR on-box. Verifies the CSR PEM is
// read back non-empty, then destroys.
func TestAccCertificate_csr(t *testing.T) {
	name := acctest.RandName("tf-acc-cert-csr")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCertificateDestroyed(name, ""),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccCertificateCSRConfig(name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_certificate.test", "id"),
					resource.TestCheckResourceAttr("truenas_certificate.test", "name", name),
					resource.TestCheckResourceAttr("truenas_certificate.test", "create_type", "CERTIFICATE_CREATE_CSR"),
					resource.TestCheckResourceAttr("truenas_certificate.test", "key_type", "RSA"),
					resource.TestCheckResourceAttr("truenas_certificate.test", "key_length", "2048"),
					resource.TestCheckResourceAttrSet("truenas_certificate.test", "csr"),
					resource.TestMatchResourceAttr("truenas_certificate.test", "csr", certRequestPattern),
					resource.TestCheckResourceAttrSet("truenas_certificate.test", "privatekey"),
					resource.TestCheckResourceAttrSet("truenas_certificate.test", "csr_path"),
				),
			},
		},
	})
}

func testAccCertificateCSRConfig(name string) string {
	return fmt.Sprintf(`
resource "truenas_certificate" "test" {
  name        = %q
  create_type = "CERTIFICATE_CREATE_CSR"
  key_type    = "RSA"
  key_length  = 2048
  common      = "tf-acc-csr.example.com"
  san         = ["tf-acc-csr.example.com"]
}
`, name)
}

// certificateSummary is the subset of certificate.query fields this test
// package needs directly (outside of the provider's own resource code).
type certificateSummary struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// testAccCheckCertificateDestroyed verifies neither "name" nor "altName"
// (used across a rename step; pass "" when not applicable) exists in
// certificate.query after the test completes.
func testAccCheckCertificateDestroyed(name, altName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		for _, n := range []string{name, altName} {
			if n == "" {
				continue
			}
			raw, err := c.Call(context.Background(), "certificate.query", [][]any{{"name", "=", n}})
			if err != nil {
				return fmt.Errorf("error checking certificate %s: %v", n, err)
			}
			var results []certificateSummary
			if err := json.Unmarshal(raw, &results); err != nil {
				return fmt.Errorf("error parsing certificate.query response: %v", err)
			}
			if len(results) > 0 {
				return fmt.Errorf("certificate %s still exists (id %d)", n, results[0].ID)
			}
		}
		return nil
	}
}

// certRequestPattern matches a PEM-encoded CSR ("CERTIFICATE REQUEST",
// probed live as the CSR PEM block type certificate.create returns).
var certRequestPattern = regexp.MustCompile(`-----BEGIN CERTIFICATE REQUEST-----`)

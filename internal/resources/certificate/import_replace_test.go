// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package certificate_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// selfSignedCertPEM builds a self-signed cert and returns the key in the
// requested PEM encoding: "pkcs8", "pkcs1" (RSA PRIVATE KEY), "sec1"
// (EC PRIVATE KEY). These are exactly what hashicorp/tls's
// tls_private_key.private_key_pem emits (PKCS#1 for RSA, SEC1 for ECDSA).
func selfSignedCertPEM(t *testing.T, cn, keyEnc string, ips ...net.IP) (string, string) {
	t.Helper()
	var signer any
	var pub any
	var keyPEM string
	switch keyEnc {
	case "sec1":
		k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		der, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			t.Fatal(err)
		}
		keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}))
		signer, pub = k, &k.PublicKey
	default:
		k, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatal(err)
		}
		if keyEnc == "pkcs1" {
			keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}))
		} else {
			der, err := x509.MarshalPKCS8PrivateKey(k)
			if err != nil {
				t.Fatal(err)
			}
			keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
		}
		signer, pub = k, &k.PublicKey
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		DNSNames:     []string{cn},
		IPAddresses:  ips,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, pub, signer)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), keyPEM
}

func importedCertConfig(name, certPEM, keyPEM string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_certificate" "test" {
  name        = %q
  create_type = "CERTIFICATE_CREATE_IMPORTED"
  certificate = %q
  privatekey  = %q
}
`, name, certPEM, keyPEM)
}

// Import -> re-apply identical config must be a no-op. create_type (and the
// other never-returned RequiresReplace inputs) come back null on import, so
// the next plan replaces the certificate.
func TestAccCertificate_ImportedImportThenPlan(t *testing.T) {
	name := acctest.RandName("tfhunt-cert-imp")
	certPEM, keyPEM := selfSignedCertPEM(t, "tfhunt-imp.example.com", "pkcs8")
	cfg := importedCertConfig(name, certPEM, keyPEM)
	var id string
	steps := []resource.TestStep{{Config: cfg, Check: captureCertID("truenas_certificate.test", &id)}}
	// create_type is derived on import for IMPORTED; everything else round-trips.
	steps = append(steps, acctest.ImportReapplyNoop("truenas_certificate.test", cfg)...)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCertificateDestroyed(name, ""),
		Steps:                    steps,
	})
}

func TestAccCertificate_CSRImportThenPlan(t *testing.T) {
	name := acctest.RandName("tfhunt-cert-csrimp")
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_certificate" "test" {
  name        = %q
  create_type = "CERTIFICATE_CREATE_CSR"
  key_type    = "EC"
  ec_curve    = "SECP256R1"
  san         = ["tfhunt-csrimp.example.com"]
}
`, name)
	var id string
	steps := []resource.TestStep{{Config: cfg, Check: captureCertID("truenas_certificate.test", &id)}}
	// A CSR's create_type (CSR vs IMPORTED_CSR) and ec_curve are not returned:
	// recorded in place after import, never a replacement.
	steps = append(steps, acctest.ImportReapplyNoop("truenas_certificate.test", cfg, "create_type", "ec_curve")...)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCertificateDestroyed(name, ""),
		Steps:                    steps,
	})
}

func captureCertID(addr string, id *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[addr]
		if !ok {
			return fmt.Errorf("%s not in state", addr)
		}
		*id = rs.Primary.ID
		return nil
	}
}

// Copyright TrueNAS 2026
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

// genCSR generates an RSA 2048 certificate signing request + its PKCS#8
// private key entirely in Go (no external tooling), for the
// CERTIFICATE_CREATE_IMPORTED_CSR acceptance path. Mirrors genSelfSigned's
// style above, swapping x509.CreateCertificate for x509.CreateCertificateRequest.
func genCSR(t *testing.T, cn string) (csrPEM, keyPEM string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}
	tmpl := &x509.CertificateRequest{
		Subject:  pkix.Name{CommonName: cn},
		DNSNames: []string{cn},
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, tmpl, key)
	if err != nil {
		t.Fatalf("creating certificate signing request: %v", err)
	}
	csrPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}))

	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshaling private key: %v", err)
	}
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))

	return csrPEM, keyPEM
}

// TestAccCertificate_importedCSR exercises the full Tier 1 contract for
// create_type=CERTIFICATE_CREATE_IMPORTED_CSR: import an externally-generated
// RSA 2048 CSR + its private key (both generated in-test, unlike
// CERTIFICATE_CREATE_CSR where TrueNAS generates the key pair on-box),
// verify the read-back fields, rename in place, import by id, and verify
// destruction via a live certificate.query. Confirmed live against TrueNAS
// 26.0 (cmd/debug_api csrimportprobe): certificate.create accepts the
// payload keys "CSR" (uppercase, per the method's own accepts schema) and
// "privatekey"; "certificate" comes back null (no cert has been signed yet,
// same as CERTIFICATE_CREATE_CSR); "common"/"san" are parsed back from the
// CSR's subject/SAN extension; "csr" and "privatekey" both round-trip
// byte-for-byte through certificate.get_instance (neither masked). Never
// touches certificate id 1 (the box's live UI certificate) — this test only
// ever creates new certificates with random tf-acc-* names.
func TestAccCertificate_importedCSR(t *testing.T) {
	name := acctest.RandName("tf-acc-cert-importedcsr")
	renamed := acctest.RandName("tf-acc-cert-importedcsr-renamed")
	cn := "tf-acc-importedcsr.example.com"
	csrPEM, keyPEM := genCSR(t, cn)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCertificateDestroyed(name, renamed),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccCertificateImportedCSRConfig(name, csrPEM, keyPEM),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_certificate.test", "id"),
					resource.TestCheckResourceAttr("truenas_certificate.test", "name", name),
					resource.TestCheckResourceAttr("truenas_certificate.test", "create_type", "CERTIFICATE_CREATE_IMPORTED_CSR"),
					resource.TestCheckResourceAttr("truenas_certificate.test", "csr", csrPEM),
					resource.TestCheckResourceAttr("truenas_certificate.test", "privatekey", keyPEM),
					resource.TestCheckResourceAttr("truenas_certificate.test", "key_type", "RSA"),
					resource.TestCheckResourceAttr("truenas_certificate.test", "key_length", "2048"),
					resource.TestCheckResourceAttr("truenas_certificate.test", "common", cn),
					resource.TestCheckResourceAttrSet("truenas_certificate.test", "csr_path"),
					resource.TestCheckResourceAttrSet("truenas_certificate.test", "privatekey_path"),
				),
			},
			// Update in place: rename. CSR/key material is unchanged, and
			// (unlike CERTIFICATE_CREATE_IMPORTED) add_to_trusted_store is
			// never sent here since certificate.update rejects it outright
			// for a CSR-only entry (confirmed by model.go's updatePayload
			// doc comment); "name" alone is always accepted.
			{
				Config: acctest.ProviderConfig() + testAccCertificateImportedCSRConfig(renamed, csrPEM, keyPEM),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_certificate.test", "name", renamed),
					resource.TestCheckResourceAttr("truenas_certificate.test", "csr", csrPEM),
				),
			},
			{
				ResourceName:      "truenas_certificate.test",
				ImportState:       true,
				ImportStateVerify: true,
				// Same ignore set as TestAccCertificate_imported: "create_type"
				// is never returned by certificate.query/get_instance and
				// cannot be recovered on import; "passphrase" is accepted on
				// create but never echoed back by any read method either.
				ImportStateVerifyIgnore: []string{"create_type", "passphrase"},
			},
		},
	})
}

func testAccCertificateImportedCSRConfig(name, csrPEM, keyPEM string) string {
	return fmt.Sprintf(`
resource "truenas_certificate" "test" {
  name        = %q
  create_type = "CERTIFICATE_CREATE_IMPORTED_CSR"
  csr         = %q
  privatekey  = %q
}
`, name, csrPEM, keyPEM)
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

// certPattern matches a PEM-encoded signed certificate.
var certPattern = regexp.MustCompile(`-----BEGIN CERTIFICATE-----`)

// acmeShellScript renders the DNS-01 "shell" authenticator script proven in
// Phase 1. TrueNAS invokes it positionally as
// `<script> set|unset <domain> <validation_name> <validation_content>`
// (running as user nobody); it publishes/clears the challenge TXT record via
// the challtestsrv management API (set-txt/clear-txt want the FQDN with a
// trailing dot). mgmt is the challtestsrv base URL (acctest.ACMEChalltestsrv).
func acmeShellScript(mgmt string) string {
	return fmt.Sprintf(`#!/bin/sh
# TrueNAS ACME DNS-01 shell authenticator -> pebble-challtestsrv (acceptance test fixture).
action="$1"; validation_name="$3"; validation_content="$4"
mgmt=%q
host="${validation_name}."
if [ "$action" = "set" ]; then
  curl -s -X POST "$mgmt/set-txt" -d "{\"host\":\"$host\",\"value\":\"$validation_content\"}"
else
  curl -s -X POST "$mgmt/clear-txt" -d "{\"host\":\"$host\"}"
fi
`, mgmt)
}

// TestAccCertificate_acmeIssuance drives a full, live ACME certificate order
// end to end through Terraform against a real ACME CA (a Pebble test server),
// using the DNS-01 "shell" authenticator wired to pebble-challtestsrv. It is
// gated by acctest.ACMECheck (TRUENAS_ACME=1 plus TRUENAS_ACME_DIRECTORY /
// TRUENAS_ACME_CHALLTESTSRV / TRUENAS_ACME_CA_PEM) and self-skips cleanly
// without that infrastructure.
//
// Two pieces of setup can't be expressed as provider resources and are done
// out-of-band via the API before the Terraform steps (each with a t.Cleanup),
// exactly as proven in Phase 1 (see .superpowers/sdd/acme-integration-report.md):
//
//  1. The shell script must be a real executable file within a pool volume.
//     A throwaway dataset is created to hold it, the script is uploaded via
//     the HTTP upload endpoint (acctest.UploadFile), and the dataset is
//     destroyed on cleanup (which removes the script).
//  2. The ACME client must trust the directory endpoint's TLS. The CA PEM is
//     imported into the trusted store via certificate.create IMPORTED +
//     add_to_trusted_store (a cert-only import, no private key — the
//     provider's own IMPORTED path requires a key, so this is done via the
//     API), and deleted on cleanup (which reverts the trust).
//
// The Terraform config then creates a truenas_acme_dns_authenticator (shell),
// a truenas_certificate CSR, and a truenas_certificate CERTIFICATE_CREATE_ACME
// referencing both; the check asserts a real signed certificate was issued.
func TestAccCertificate_acmeIssuance(t *testing.T) {
	acctest.ACMECheck(t)

	ctx := context.Background()
	c := acctest.Client()

	suffix := acctest.RandName("")[1:] // drop the leading "-" from RandName("")
	dsName := fmt.Sprintf("%s/tf-acc-acme-%s", acctest.TestPool(), suffix)
	scriptPath := "/mnt/" + dsName + "/dns-01.sh"
	domain := fmt.Sprintf("tf-acc-acme-%s.example.com", suffix)
	authName := "tf-acc-acme-auth-" + suffix
	csrName := "tf-acc-acme-csr-" + suffix
	acmeName := "tf-acc-acme-cert-" + suffix

	// 1. Throwaway dataset to hold the DNS-01 script (mount dir is o+rx, so
	// the authenticator's "nobody" user can traverse+execute the script).
	if _, err := c.Call(ctx, "pool.dataset.create", map[string]any{"name": dsName}); err != nil {
		t.Fatalf("create dataset %s: %v", dsName, err)
	}
	t.Cleanup(func() {
		if _, err := c.Call(context.Background(), "pool.dataset.delete", dsName, map[string]any{"recursive": true}); err != nil {
			t.Logf("cleanup: delete dataset %s: %v", dsName, err)
		}
	})
	acctest.UploadFile(t, scriptPath, []byte(acmeShellScript(acctest.ACMEChalltestsrv())), 0o755)

	// 2. Trust the ACME directory's TLS by importing its CA PEM into the
	// trusted store (cert-only import; the pebble-acme directory cert is
	// CA:TRUE so no private key is required).
	caRaw, err := c.CallJob(ctx, "certificate.create", map[string]any{
		"name":                 "tf_acc_acme_ca_" + suffix,
		"create_type":          "CERTIFICATE_CREATE_IMPORTED",
		"certificate":          acctest.ACMECAPEM(t),
		"add_to_trusted_store": true,
	})
	if err != nil {
		t.Fatalf("import ACME CA into trusted store: %v", err)
	}
	var caCert struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(caRaw, &caCert); err != nil {
		t.Fatalf("parse CA import response: %v", err)
	}
	t.Cleanup(func() {
		if _, err := c.CallJob(context.Background(), "certificate.delete", caCert.ID); err != nil {
			t.Logf("cleanup: delete trusted-store CA cert %d: %v", caCert.ID, err)
		}
	})

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAcmeIssuanceDestroyed(authName, csrName, acmeName),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccCertificateAcmeConfig(
					authName, scriptPath, csrName, domain, acmeName, acctest.ACMEDirectory()),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_certificate.acme", "id"),
					resource.TestCheckResourceAttr("truenas_certificate.acme", "name", acmeName),
					resource.TestCheckResourceAttr("truenas_certificate.acme", "create_type", "CERTIFICATE_CREATE_ACME"),
					// A real signed certificate was issued (not a CSR): the PEM
					// is present, fingerprint is set, and it is not expired.
					resource.TestMatchResourceAttr("truenas_certificate.acme", "certificate", certPattern),
					resource.TestCheckResourceAttrSet("truenas_certificate.acme", "fingerprint"),
					resource.TestCheckResourceAttr("truenas_certificate.acme", "expired", "false"),
					resource.TestCheckResourceAttrSet("truenas_certificate.acme", "certificate_path"),
					// The referenced CSR fixture is a real CSR.
					resource.TestMatchResourceAttr("truenas_certificate.csr", "csr", certRequestPattern),
				),
			},
		},
	})
}

func testAccCertificateAcmeConfig(authName, scriptPath, csrName, domain, acmeName, directory string) string {
	return fmt.Sprintf(`
resource "truenas_acme_dns_authenticator" "auth" {
  name = %[1]q
  attributes = jsonencode({
    authenticator = "shell"
    script        = %[2]q
    user          = "nobody"
    delay         = 10
  })
}

resource "truenas_certificate" "csr" {
  name        = %[3]q
  create_type = "CERTIFICATE_CREATE_CSR"
  key_type    = "RSA"
  key_length  = 2048
  common      = %[4]q
  san         = [%[4]q]
}

resource "truenas_certificate" "acme" {
  name               = %[5]q
  create_type        = "CERTIFICATE_CREATE_ACME"
  acme_directory_uri = %[6]q
  csr_id             = truenas_certificate.csr.id
  tos                = true
  renew_days         = 10
  dns_mapping = {
    %[4]q = truenas_acme_dns_authenticator.auth.id
  }
}
`, authName, scriptPath, csrName, domain, acmeName, directory)
}

// testAccCheckAcmeIssuanceDestroyed verifies the authenticator and both
// certificates (CSR fixture + issued ACME cert) are gone after the test.
func testAccCheckAcmeIssuanceDestroyed(authName, csrName, acmeName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		for _, n := range []string{csrName, acmeName} {
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
		raw, err := c.Call(context.Background(), "acme.dns.authenticator.query", [][]any{{"name", "=", authName}})
		if err != nil {
			return fmt.Errorf("error checking ACME DNS authenticator %s: %v", authName, err)
		}
		var auths []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &auths); err != nil {
			return fmt.Errorf("error parsing acme.dns.authenticator.query response: %v", err)
		}
		if len(auths) > 0 {
			return fmt.Errorf("ACME DNS authenticator %s still exists (id %d)", authName, auths[0].ID)
		}
		return nil
	}
}

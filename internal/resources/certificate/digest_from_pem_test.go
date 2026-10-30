// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package certificate

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"testing"
)

// Hunt F1: certificate.query returns digest_algorithm null for a CSR, so an
// imported CSR with digest_algorithm in config planned an update forever. The
// digest is derived from the signature algorithm of the CSR PEM instead.
func TestDigestFromPEM(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ecKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	csr := func(key any, alg x509.SignatureAlgorithm) *string {
		der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
			Subject: pkix.Name{CommonName: "tfhunt.example.com"}, SignatureAlgorithm: alg,
		}, key)
		if err != nil {
			t.Fatal(err)
		}
		s := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}))
		return &s
	}
	empty := ""
	junk := "-----BEGIN CERTIFICATE REQUEST-----\nbm90IGEgY3Ny\n-----END CERTIFICATE REQUEST-----\n"
	cases := []struct {
		name string
		csr  *string
		want string
	}{
		{"RSA SHA512", csr(rsaKey, x509.SHA512WithRSA), "SHA512"},
		{"RSA SHA256", csr(rsaKey, x509.SHA256WithRSA), "SHA256"},
		{"EC SHA384", csr(ecKey, x509.ECDSAWithSHA384), "SHA384"},
		{"nil", nil, ""},
		{"empty", &empty, ""},
		{"unparsable", &junk, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := digestFromPEM(tc.csr, nil); got != tc.want {
				t.Errorf("digestFromPEM = %q, want %q", got, tc.want)
			}
		})
	}
}

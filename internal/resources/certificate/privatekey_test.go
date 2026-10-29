// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package certificate

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func pemString(typ string, der []byte) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der}))
}

func pkcs8PEM(t *testing.T, k any) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	return pemString("PRIVATE KEY", der)
}

func certFor(t *testing.T, k *rsa.PrivateKey) string {
	t.Helper()
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "x.example.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &k.PublicKey, k)
	if err != nil {
		t.Fatal(err)
	}
	return pemString("CERTIFICATE", der)
}

// TestSamePrivateKey covers the re-encodings TrueNAS applies to an imported
// key (B-013) and checks a different key is still reported as different.
func TestSamePrivateKey(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	otherRSA, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ecDER, err := x509.MarshalECPrivateKey(ecKey)
	if err != nil {
		t.Fatal(err)
	}
	serverRSA := pkcs8PEM(t, rsaKey)
	serverEC := pkcs8PEM(t, ecKey)
	ecParams := pemString("EC PARAMETERS", []byte{0x06, 0x08, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x03, 0x01, 0x07})
	encrypted := "-----BEGIN ENCRYPTED PRIVATE KEY-----\nMIIBAA==\n-----END ENCRYPTED PRIVATE KEY-----\n"
	cert := certFor(t, rsaKey)
	otherCert := certFor(t, otherRSA)

	cases := []struct {
		name               string
		cfg, server, bound string
		want               bool
	}{
		{"identical text", serverRSA, serverRSA, "", true},
		{"pkcs1 vs pkcs8", pemString("RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(rsaKey)), serverRSA, "", true},
		{"sec1 vs pkcs8", pemString("EC PRIVATE KEY", ecDER), serverEC, "", true},
		{"sec1 with ec parameters", ecParams + pemString("EC PRIVATE KEY", ecDER), serverEC, "", true},
		{"no trailing newline", serverRSA[:len(serverRSA)-1], serverRSA, "", true},
		{"different key", pkcs8PEM(t, otherRSA), serverRSA, "", false},
		{"encrypted, server key matches cert", encrypted, serverRSA, cert, true},
		{"encrypted, server key does not match cert", encrypted, serverRSA, otherCert, false},
		{"encrypted, no bound cert", encrypted, serverRSA, "", false},
		{"garbage config", "not a key", serverRSA, cert, false},
		{"garbage server", serverRSA, "not a key", cert, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := samePrivateKey(tc.cfg, tc.server, tc.bound); got != tc.want {
				t.Errorf("samePrivateKey = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestResponseToModel_KeepsConfiguredKeyEncoding verifies responseToModel
// keeps the configured PKCS#1 text when the server returns the same key as
// PKCS#8, and takes the server value when the key differs.
func TestResponseToModel_KeepsConfiguredKeyEncoding(t *testing.T) {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pkcs1 := pemString("RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(k))
	for _, tc := range []struct {
		name, server, want string
	}{
		{"same key", pkcs8PEM(t, k), pkcs1},
		{"different key", pkcs8PEM(t, other), pkcs8PEM(t, other)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := tc.server
			api := &certificateAPI{ID: 1, Name: "x", CertType: "CERTIFICATE", Privatekey: &server}
			m := &CertificateModel{
				CreateType: types.StringValue(CreateTypeImported),
				Privatekey: types.StringValue(pkcs1),
			}
			if d := responseToModel(context.Background(), api, m); d.HasError() {
				t.Fatalf("responseToModel: %v", d)
			}
			if m.Privatekey.ValueString() != tc.want {
				t.Errorf("privatekey = %q, want %q", m.Privatekey.ValueString(), tc.want)
			}
		})
	}
}

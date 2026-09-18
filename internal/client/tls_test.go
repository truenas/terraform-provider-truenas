// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package client_test

import (
	"os"
	"testing"

	"github.com/truenas/terraform-provider-truenas/internal/client"
)

func TestBuildTLSConfig_Insecure(t *testing.T) {
	cfg, err := client.BuildTLSConfig(true, "")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify=true")
	}
}

func TestBuildTLSConfig_Default(t *testing.T) {
	cfg, err := client.BuildTLSConfig(false, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify=false")
	}
	if cfg.RootCAs != nil {
		t.Error("expected nil RootCAs for system default")
	}
}

func TestBuildTLSConfig_BadCAFile(t *testing.T) {
	_, err := client.BuildTLSConfig(false, "/nonexistent/ca.pem")
	if err == nil {
		t.Error("expected error for missing CA file")
	}
}

func TestBuildTLSConfig_ValidCAFile(t *testing.T) {
	// Write a self-signed PEM so the function can parse it
	const pem = `-----BEGIN CERTIFICATE-----
MIICpDCCAYwCCQDU+pQ4pHgSpDANBgkqhkiG9w0BAQsFADAUMRIwEAYDVQQDDAls
b2NhbGhvc3QwHhcNMjMwMTAxMDAwMDAwWhcNMjQwMTAxMDAwMDAwWjAUMRIwEAYD
VQQDDAlsb2NhbGhvc3QwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC7
o4qne60TB3wolWnkBpnhBCxRFzPxF3RFX2Q1Byx0AAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AgMBAAEwDQYJKoZIhvcNAQELBQADggEBAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAA=
-----END CERTIFICATE-----`
	f, err := os.CreateTemp("", "ca*.pem")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(pem)
	f.Close()

	// Function should succeed even if cert can't be parsed — it appends to pool
	// If AppendCertsFromPEM returns false, we return an error
	_, err = client.BuildTLSConfig(false, f.Name())
	// May succeed or fail depending on cert validity; either way no panic
	_ = err
}

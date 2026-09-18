// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package client_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// TestLiveSCRAM authenticates against a real TrueNAS box via SCRAM-SHA-512.
// Gated on the acceptance-test env vars plus TRUENAS_USERNAME (the API key
// owner); skipped otherwise. Requires TrueNAS 26.0+.
func TestLiveSCRAM(t *testing.T) {
	endpoint := os.Getenv("TRUENAS_ENDPOINT")
	apiKey := os.Getenv("TRUENAS_API_KEY")
	username := os.Getenv("TRUENAS_USERNAME")
	if os.Getenv("TF_ACC") == "" || endpoint == "" || apiKey == "" || username == "" {
		t.Skip("Set TF_ACC=1, TRUENAS_ENDPOINT, TRUENAS_API_KEY, TRUENAS_USERNAME for the live SCRAM test")
	}

	tlsCfg, err := client.BuildTLSConfig(true, "")
	if err != nil {
		t.Fatal(err)
	}
	c := client.New(endpoint, tlsCfg)
	ctx := context.Background()

	if err := c.Connect(ctx, func(ctx context.Context) error {
		choices, err := client.MechanismChoices(ctx, c)
		if err != nil {
			t.Skipf("auth.mechanism_choices failed (pre-26.0 server?): %v", err)
		}
		t.Logf("server mechanisms: %v", choices)
		return client.AuthAPIKeySCRAM(ctx, c, username, apiKey)
	}); err != nil {
		t.Fatalf("SCRAM authentication failed: %v", err)
	}
	defer c.Close()

	raw, err := c.Call(ctx, "system.version_short")
	if err != nil {
		t.Fatalf("authenticated call failed: %v", err)
	}
	t.Logf("authenticated via SCRAM; server version %s", raw)
}

// liveScramAttempt runs one SCRAM exchange against the real box with the
// given credentials and returns the authentication error (nil on success).
func liveScramAttempt(t *testing.T, endpoint, username, apiKey string) error {
	t.Helper()
	tlsCfg, err := client.BuildTLSConfig(true, "")
	if err != nil {
		t.Fatal(err)
	}
	c := client.New(endpoint, tlsCfg)
	authErr := make(chan error, 1)
	err = c.Connect(context.Background(), func(ctx context.Context) error {
		e := client.AuthAPIKeySCRAM(ctx, c, username, apiKey)
		authErr <- e
		return e
	})
	defer c.Close()
	if err == nil {
		return nil
	}
	return <-authErr
}

// TestLiveSCRAM_WrongCredentialsRejected verifies the REAL middleware
// rejects SCRAM exchanges built from a wrong secret and from a wrong
// username — the negative half of the protocol, judged by the server
// itself rather than any simulated one. Same gating as TestLiveSCRAM.
func TestLiveSCRAM_WrongCredentialsRejected(t *testing.T) {
	endpoint := os.Getenv("TRUENAS_ENDPOINT")
	apiKey := os.Getenv("TRUENAS_API_KEY")
	username := os.Getenv("TRUENAS_USERNAME")
	if os.Getenv("TF_ACC") == "" || endpoint == "" || apiKey == "" || username == "" {
		t.Skip("Set TF_ACC=1, TRUENAS_ENDPOINT, TRUENAS_API_KEY, TRUENAS_USERNAME for live SCRAM tests")
	}
	// Only meaningful on a SCRAM-capable server; TestLiveSCRAM covers the
	// capability check.
	if err := liveScramAttempt(t, endpoint, username, apiKey); err != nil {
		t.Skipf("baseline SCRAM login failed (server without SCRAM?): %v", err)
	}

	// Same key ID, corrupted secret: the proof cannot verify.
	keyID, secret, ok := strings.Cut(apiKey, "-")
	if !ok {
		t.Fatalf("unexpected API key format")
	}
	corrupted := fmt.Sprintf("%s-X%s", keyID, secret[1:])
	if err := liveScramAttempt(t, endpoint, username, corrupted); err == nil {
		t.Fatal("server accepted a SCRAM proof built from a corrupted secret")
	}

	// Wrong username with the right key.
	if err := liveScramAttempt(t, endpoint, "not-the-owner", apiKey); err == nil {
		t.Fatal("server accepted a SCRAM exchange for the wrong username")
	}
}

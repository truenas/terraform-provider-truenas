package client_test

import (
	"context"
	"os"
	"testing"

	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// TestLiveSCRAM authenticates against a real TrueNAS box via SCRAM-SHA-512.
// Gated on the acceptance-test env vars plus TRUENAS_USERNAME (the API key
// owner); skipped otherwise. Requires SCALE 26.0+.
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

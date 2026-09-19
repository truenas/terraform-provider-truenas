// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package api_key_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// testAccAPIKeyUsername returns the local user to own the acceptance-test
// API key: TRUENAS_USERNAME when set, falling back to truenas_admin (the
// TrueNAS default local admin account, confirmed present via a live
// user.query probe).
func testAccAPIKeyUsername() string {
	if v := os.Getenv("TRUENAS_USERNAME"); v != "" {
		return v
	}
	return "truenas_admin"
}

// TestAccAPIKey_basic creates an API key for the acceptance-test user,
// proves the created key actually authenticates against the real server
// (opening a second client.Client and calling system.version_short through
// it — the SCRAM path on TrueNAS 26.0+), renames the key in place, imports it
// by numeric id, and verifies destruction.
func TestAccAPIKey_basic(t *testing.T) {
	username := testAccAPIKeyUsername()
	name := acctest.RandName("tf-acc-key")
	renamed := name + "-renamed"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAPIKeyDestroyed,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccAPIKeyConfig(name, username),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_api_key.test", "id"),
					resource.TestCheckResourceAttr("truenas_api_key.test", "name", name),
					resource.TestCheckResourceAttr("truenas_api_key.test", "username", username),
					resource.TestCheckResourceAttrSet("truenas_api_key.test", "key"),
					resource.TestCheckResourceAttrSet("truenas_api_key.test", "created_at"),
					resource.TestCheckResourceAttr("truenas_api_key.test", "local", "true"),
					resource.TestCheckResourceAttr("truenas_api_key.test", "revoked", "false"),
					// The end-to-end proof: authenticate a brand new
					// client with exactly the key value stored in state,
					// and confirm the server accepts it. This is the one
					// extra login this test performs (auth rate limit is
					// ~20/60s/IP — a single extra login here is fine).
					testAccCheckAPIKeyAuthenticates("truenas_api_key.test", username),
				),
			},
			// Rename in place — must not require replacement.
			{
				Config: acctest.ProviderConfig() + testAccAPIKeyConfig(renamed, username),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_api_key.test", "name", renamed),
					resource.TestCheckResourceAttrSet("truenas_api_key.test", "key"),
				),
			},
			// Import by numeric id. The plaintext key is unknowable on
			// import (TrueNAS never re-exposes it), so it must be ignored.
			{
				ResourceName:            "truenas_api_key.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"key"},
			},
		},
	})
}

func testAccAPIKeyConfig(name, username string) string {
	return fmt.Sprintf(`
resource "truenas_api_key" "test" {
  name     = %q
  username = %q
}
`, name, username)
}

// testAccCheckAPIKeyAuthenticates reads the plaintext "key" attribute out
// of Terraform state, opens a fresh client.Client against the acceptance
// test server, authenticates with exactly that key (via AuthAPIKeyAuto, so
// it upgrades to SCRAM-SHA-512 when the server offers it), and calls
// system.version_short — proving the key TrueNAS handed back really works,
// not just that create/read round-trips a string.
func testAccCheckAPIKeyAuthenticates(resourceName, username string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		keyVal := rs.Primary.Attributes["key"]
		if keyVal == "" {
			return fmt.Errorf("resource %s has an empty key attribute", resourceName)
		}

		tlsCfg, err := client.BuildTLSConfig(true, "")
		if err != nil {
			return fmt.Errorf("building TLS config: %w", err)
		}
		c := client.New(acctest.Endpoint(), tlsCfg)
		defer c.Close()

		ctx := context.Background()
		authFn := func(ctx context.Context) error { return client.AuthAPIKeyAuto(ctx, c, username, keyVal) }
		if err := c.Connect(ctx, authFn); err != nil {
			return fmt.Errorf("newly-created API key failed to authenticate: %w", err)
		}

		raw, err := c.CallRead(ctx, "system.version_short")
		if err != nil {
			return fmt.Errorf("system.version_short via the newly-created API key failed: %w", err)
		}
		var version string
		if err := json.Unmarshal(raw, &version); err != nil {
			return fmt.Errorf("parsing system.version_short response: %w", err)
		}
		if version == "" {
			return fmt.Errorf("system.version_short returned an empty version string")
		}
		return nil
	}
}

// testAccCheckAPIKeyDestroyed queries api_key by the id captured in the
// pre-destroy state (the resource's name has changed by the time this
// runs, and TrueNAS assigns no other stable external handle), matching the
// established pattern used elsewhere in this provider (e.g. nvmet_port).
func testAccCheckAPIKeyDestroyed(s *terraform.State) error {
	rs, ok := s.RootModule().Resources["truenas_api_key.test"]
	if !ok {
		return fmt.Errorf("resource truenas_api_key.test not found in pre-destroy state")
	}
	idStr, ok := rs.Primary.Attributes["id"]
	if !ok {
		return fmt.Errorf("truenas_api_key.test has no id attribute in pre-destroy state")
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return fmt.Errorf("parsing api key id %q: %v", idStr, err)
	}

	c := acctest.Client()
	raw, err := c.Call(context.Background(), "api_key.query", [][]any{{"id", "=", id}})
	if err != nil {
		return fmt.Errorf("error checking api key id=%d: %v", id, err)
	}
	var results []struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(raw, &results); err != nil {
		return fmt.Errorf("error parsing api_key.query response: %v", err)
	}
	if len(results) > 0 {
		return fmt.Errorf("api key id=%d still exists", id)
	}
	return nil
}

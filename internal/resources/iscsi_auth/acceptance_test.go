// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_auth_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// testAccISCSIAuthTag is the CHAP group tag used by this test. It is
// distinct from any live tag on the box and from the tag used by the
// end-to-end iSCSI test (999).
const testAccISCSIAuthTag = 998

// TestAccISCSIAuth_basic tests create, update, and import of an iSCSI CHAP
// auth entry.
func TestAccISCSIAuth_basic(t *testing.T) {
	user := acctest.RandName("tf-acc-chapuser")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckISCSIAuthDestroyed(),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccISCSIAuthConfig(user, "tfacc-secret-1ch", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_auth.test", "user", user),
					resource.TestCheckResourceAttr("truenas_iscsi_auth.test", "tag", fmt.Sprintf("%d", testAccISCSIAuthTag)),
					resource.TestCheckResourceAttrSet("truenas_iscsi_auth.test", "id"),
				),
			},
			// Update the secret and pin discovery_auth to NONE explicitly.
			{
				Config: acctest.ProviderConfig() + testAccISCSIAuthConfig(user, "tfacc-secret-2ch", "NONE"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_auth.test", "discovery_auth", "NONE"),
				),
			},
			{
				ResourceName:            "truenas_iscsi_auth.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"secret", "peersecret"},
			},
		},
	})
}

// TestAccISCSIAuth_mutual sets a consistent mutual-CHAP combination:
// peeruser, peersecret (write-only) with discovery_auth = CHAP_MUTUAL, plus
// the secret_wo_version / peersecret_wo_version rotation triggers, then bumps
// the triggers to rotate both secrets.
func TestAccISCSIAuth_mutual(t *testing.T) {
	user := acctest.RandName("tf-acc-chapm")
	peer := acctest.RandName("tf-acc-peer")
	cfg := func(secret, peerSecret string, ver int) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_iscsi_auth" "test" {
  tag                   = %d
  user                  = %q
  secret                = %q
  secret_wo_version     = %d
  peeruser              = %q
  peersecret            = %q
  peersecret_wo_version = %d
  discovery_auth        = "CHAP_MUTUAL"
}
`, testAccISCSIAuthTag, user, secret, ver, peer, peerSecret, ver)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckISCSIAuthDestroyed(),
		Steps: []resource.TestStep{
			{
				Config: cfg("tfacc-secret-1m", "tfacc-peersec-1", 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_auth.test", "user", user),
					resource.TestCheckResourceAttr("truenas_iscsi_auth.test", "peeruser", peer),
					resource.TestCheckResourceAttr("truenas_iscsi_auth.test", "discovery_auth", "CHAP_MUTUAL"),
					resource.TestCheckResourceAttr("truenas_iscsi_auth.test", "secret_wo_version", "1"),
					resource.TestCheckResourceAttr("truenas_iscsi_auth.test", "peersecret_wo_version", "1"),
				),
			},
			// Rotate both secrets by bumping the version triggers.
			{
				Config: cfg("tfacc-secret-2m", "tfacc-peersec-2", 2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_auth.test", "secret_wo_version", "2"),
					resource.TestCheckResourceAttr("truenas_iscsi_auth.test", "peersecret_wo_version", "2"),
					resource.TestCheckResourceAttr("truenas_iscsi_auth.test", "peeruser", peer),
				),
			},
			{
				ResourceName:      "truenas_iscsi_auth.test",
				ImportState:       true,
				ImportStateVerify: true,
				// Write-only secrets and their version triggers are never
				// returned by the API.
				ImportStateVerifyIgnore: []string{"secret", "peersecret", "secret_wo_version", "peersecret_wo_version"},
			},
		},
	})
}

func testAccISCSIAuthConfig(user, secret, discoveryAuth string) string {
	discoveryAuthLine := ""
	if discoveryAuth != "" {
		discoveryAuthLine = fmt.Sprintf("  discovery_auth = %q\n", discoveryAuth)
	}
	return fmt.Sprintf(`
resource "truenas_iscsi_auth" "test" {
  tag    = %d
  user   = %q
  secret = %q
%s}
`, testAccISCSIAuthTag, user, secret, discoveryAuthLine)
}

func testAccCheckISCSIAuthDestroyed() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "iscsi.auth.query", [][]any{{"tag", "=", testAccISCSIAuthTag}})
		if err != nil {
			return fmt.Errorf("error checking iscsi auth tag %d: %v", testAccISCSIAuthTag, err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing iscsi.auth.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("iscsi auth tag %d still exists", testAccISCSIAuthTag)
		}
		return nil
	}
}

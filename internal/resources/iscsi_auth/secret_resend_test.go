// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_auth_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccISCSIAuth_secretsSentOnlyOnVersionChange: an update that does not
// change secret_wo_version / peersecret_wo_version leaves the stored CHAP
// secrets alone (a changed value is not applied); bumping a version sends that
// secret; turning mutual CHAP on sends the peer secret, because the server
// requires it with a peer user. (B-141)
func TestAccISCSIAuth_secretsSentOnlyOnVersionChange(t *testing.T) {
	user := acctest.RandName("tfaccauthwo")
	cfg := func(chapUser, secret string, ver int, peer string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_iscsi_auth" "test" {
  tag               = 9312
  user              = %q
  secret            = %q
  secret_wo_version = %d
%s
}
`, chapUser, secret, ver, peer)
	}
	mutual := func(peerSecret, ver string) string {
		return fmt.Sprintf(`  peeruser       = "tfaccpeerwo"
  peersecret     = %q
  discovery_auth = "CHAP_MUTUAL"
%s`, peerSecret, ver)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		Steps: []resource.TestStep{
			{Config: cfg(user, "tfaccwosecretA", 1, ""), Check: checkServerAuth(t, "secret", "tfaccwosecretA")},
			{
				Config: cfg(user+"b", "tfaccwosecretB", 1, ""),
				Check: resource.ComposeTestCheckFunc(
					checkServerAuth(t, "user", user+"b"),
					checkServerAuth(t, "secret", "tfaccwosecretA"),
				),
			},
			{Config: cfg(user+"b", "tfaccwosecretB", 2, ""), Check: checkServerAuth(t, "secret", "tfaccwosecretB")},
			// Mutual CHAP on: the peer secret goes out without a version.
			{
				Config: cfg(user+"b", "tfaccwosecretB", 2, mutual("tfaccwopeersecA", "")),
				Check: resource.ComposeTestCheckFunc(
					checkServerAuth(t, "peeruser", "tfaccpeerwo"),
					checkServerAuth(t, "peersecret", "tfaccwopeersecA"),
					checkServerAuth(t, "secret", "tfaccwosecretB"),
				),
			},
			{
				Config: cfg(user, "tfaccwosecretB", 2, mutual("tfaccwopeersecB", "")),
				Check: resource.ComposeTestCheckFunc(
					checkServerAuth(t, "user", user),
					checkServerAuth(t, "peersecret", "tfaccwopeersecA"),
				),
			},
			{
				Config: cfg(user, "tfaccwosecretB", 2, mutual("tfaccwopeersecB", "  peersecret_wo_version = 1")),
				Check: resource.ComposeTestCheckFunc(
					checkServerAuth(t, "peersecret", "tfaccwopeersecB"),
					checkServerAuth(t, "secret", "tfaccwosecretB"),
				),
			},
		},
	})
}

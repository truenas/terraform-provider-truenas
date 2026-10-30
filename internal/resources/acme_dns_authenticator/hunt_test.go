// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acme_dns_authenticator_test

// Adversarial bug-hunt test: set every API-accepted attribute (name,
// attributes, and the write-only secret overlay attributes_secrets_wo +
// attributes_secrets_wo_version) at once, assert the read-back re-plans empty,
// then import. Like the acceptance suite this runs ungated (acme.dns.
// authenticator.create does not validate the cloudflare credentials against the
// real DNS provider), so a syntactically valid but fake api_token is used.

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntAcmeDnsAuthenticator_allFields creates a cloudflare authenticator
// whose secret is supplied through the write-only overlay (so attributes
// itself carries only the authenticator discriminator), asserts an empty
// re-plan, and imports.
func TestHuntAcmeDnsAuthenticator_allFields(t *testing.T) {
	name := acctest.RandName("tfhunt-acme-dns")

	c := acctest.ProviderConfig() + `
resource "truenas_acme_dns_authenticator" "test" {
  name = "` + name + `"
  attributes = jsonencode({
    authenticator = "cloudflare"
  })
  attributes_secrets_wo         = jsonencode({ api_token = "tfhunt-dummy-token" })
  attributes_secrets_wo_version = 1
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAcmeDnsAuthenticatorDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: c,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_acme_dns_authenticator.test", "id"),
					resource.TestCheckResourceAttr("truenas_acme_dns_authenticator.test", "name", name),
				),
			},
			{
				Config:           c,
				ConfigPlanChecks: expectEmptyPlan(),
			},
			{
				ResourceName:      "truenas_acme_dns_authenticator.test",
				ImportState:       true,
				ImportStateVerify: true,
				// attributes: the server returns the stored document with its own
				// null keys / compaction, so the raw ImportStateVerify string
				// compare differs even though the plan is a no-op (B-120). The
				// write-only overlay fields are never read back.
				ImportStateVerifyIgnore: []string{"attributes", "attributes_secrets_wo", "attributes_secrets_wo_version"},
			},
		},
	})
}

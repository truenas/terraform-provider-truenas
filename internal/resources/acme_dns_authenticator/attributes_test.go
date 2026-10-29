// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acme_dns_authenticator_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func expectEmptyPlan() resource.ConfigPlanChecks {
	return resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
}

// TestAccAcmeDnsAuthenticator_importPlan: import stored the server document
// (with server-added null keys), so the plan re-sent the credentials (B-120).
func TestAccAcmeDnsAuthenticator_importPlan(t *testing.T) {
	name := acctest.RandName("tffix3shtasks-acme")
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_acme_dns_authenticator" "test" {
  name = %q
  attributes = jsonencode({
    authenticator = "cloudflare"
    api_token     = "tffix3-token"
  })
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAcmeDnsAuthenticatorDestroyed(name),
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ResourceName: "truenas_acme_dns_authenticator.test", ImportState: true, ImportStateKind: resource.ImportBlockWithID},
		},
	})
}

// TestAccAcmeDnsAuthenticator_heredocWriteOnly: with the write-only overlay,
// Read rewrote attributes as compact JSON, so a heredoc spelling diffed on
// every plan (B-061). Also without the overlay.
func TestAccAcmeDnsAuthenticator_heredocWriteOnly(t *testing.T) {
	name := acctest.RandName("tffix3shtasks-acme")
	wo := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_acme_dns_authenticator" "test" {
  name = %q
  attributes = <<-EOT
    {"authenticator": "cloudflare"}
  EOT
  attributes_secrets_wo         = jsonencode({ api_token = "tffix3-secret" })
  attributes_secrets_wo_version = 1
}
`, name)
	plain := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_acme_dns_authenticator" "test" {
  name = %q
  attributes = <<-EOT
    {"authenticator": "cloudflare", "api_token": "tffix3-token"}
  EOT
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAcmeDnsAuthenticatorDestroyed(name),
		Steps: []resource.TestStep{
			{Config: wo},
			{Config: wo, ConfigPlanChecks: expectEmptyPlan()},
			{Config: plain},
			{Config: plain, ConfigPlanChecks: expectEmptyPlan()},
		},
	})
}

// roProviderConfig is a provider block (the only one in the configuration)
// that uses a READONLY_ADMIN API key from TRUENAS_RO_API_KEY /
// TRUENAS_RO_USERNAME. The in-process test provider serves every provider
// block from one instance, so the read-only steps must not also declare the
// admin provider.
func roProviderConfig(t *testing.T) string {
	key, user := os.Getenv("TRUENAS_RO_API_KEY"), os.Getenv("TRUENAS_RO_USERNAME")
	if key == "" || user == "" {
		t.Skip("TRUENAS_RO_API_KEY and TRUENAS_RO_USERNAME (a READONLY_ADMIN API key) are not set")
	}
	return fmt.Sprintf(`
provider "truenas" {
  endpoint = %q
  insecure = true
  api_key  = %q
  username = %q
}
`, acctest.Endpoint(), key, user)
}

func noRedactedValue(addr string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[addr]
		if !ok {
			return fmt.Errorf("%s not in state", addr)
		}
		for k, v := range rs.Primary.Attributes {
			if strings.Contains(v, "********") {
				return fmt.Errorf("%s.%s holds the redaction mask: %q", addr, k, v)
			}
		}
		return nil
	}
}

// TestAccAcmeDnsAuthenticator_readOnlyKey: a READONLY_ADMIN session gets the
// whole attributes object as "********"; the decode failed, so refresh and the
// data source failed (B-031). Now the resource keeps its value (no change
// planned) and the data source reports null. The object is created with the
// admin key; the read-only step's configuration has only the read-only
// provider; the last step switches back so destroy runs as admin.
func TestAccAcmeDnsAuthenticator_readOnlyKey(t *testing.T) {
	ro := roProviderConfig(t)
	name := acctest.RandName("tffix3shtasks-acme")
	res := fmt.Sprintf(`
resource "truenas_acme_dns_authenticator" "test" {
  name = %q
  attributes = jsonencode({
    authenticator = "cloudflare"
    api_token     = "tffix3-token"
  })
}
`, name)
	ds := fmt.Sprintf(`
data "truenas_acme_dns_authenticator" "ro" {
  name = %q
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAcmeDnsAuthenticatorDestroyed(name),
		Steps: []resource.TestStep{
			{Config: acctest.ProviderConfig() + res},
			{
				Config: ro + res + ds,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_acme_dns_authenticator.test", plancheck.ResourceActionNoop),
				}},
				Check: resource.ComposeTestCheckFunc(
					noRedactedValue("data.truenas_acme_dns_authenticator.ro"),
					noRedactedValue("truenas_acme_dns_authenticator.test"),
					resource.TestCheckNoResourceAttr("data.truenas_acme_dns_authenticator.ro", "attributes"),
				),
			},
			{Config: acctest.ProviderConfig() + res},
		},
	})
}

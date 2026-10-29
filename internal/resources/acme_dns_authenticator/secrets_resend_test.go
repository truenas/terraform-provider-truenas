// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acme_dns_authenticator_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func checkAcmeToken(want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, _ := strconv.ParseInt(s.RootModule().Resources["truenas_acme_dns_authenticator.test"].Primary.ID, 10, 64)
		raw, err := acctest.Client().CallRead(context.Background(), "acme.dns.authenticator.get_instance", id)
		if err != nil {
			return err
		}
		var got struct {
			Attributes map[string]any `json:"attributes"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			return err
		}
		if got.Attributes["api_token"] != want {
			return fmt.Errorf("stored api_token = %v, want %q", got.Attributes["api_token"], want)
		}
		return nil
	}
}

// TestAccAcmeDnsAuthenticator_secretsSentOnlyOnVersionChange:
// acme.dns.authenticator.update replaces attributes whole, so the secret keys
// go out on every update; with the same attributes_secrets_wo_version they are
// the stored values, so a changed overlay is not applied; bumping the version
// sends it. (B-141)
func TestAccAcmeDnsAuthenticator_secretsSentOnlyOnVersionChange(t *testing.T) {
	name := acctest.RandName("tf-acc-acmewo")
	cfg := func(n, token string, version int) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_acme_dns_authenticator" "test" {
  name                          = %q
  attributes                    = jsonencode({ authenticator = "cloudflare" })
  attributes_secrets_wo         = jsonencode({ api_token = %q })
  attributes_secrets_wo_version = %d
}
`, n, token, version)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		CheckDestroy:             testAccCheckAcmeDnsAuthenticatorDestroyed(name + "r"),
		Steps: []resource.TestStep{
			{Config: cfg(name, "tfwo-token-a", 1), Check: checkAcmeToken("tfwo-token-a")},
			{
				Config: cfg(name+"r", "tfwo-token-b", 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_acme_dns_authenticator.test", "name", name+"r"),
					checkAcmeToken("tfwo-token-a"),
				),
			},
			{Config: cfg(name+"r", "tfwo-token-b", 2), Check: checkAcmeToken("tfwo-token-b")},
		},
	})
}

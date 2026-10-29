// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package keychain_ssh_keypair_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccKeychainSSHKeyPair_privateKeyNoTrailingNewline: a private key supplied
// without its final newline (trimspace(file(...)), a secret-store value) is
// stored by the server with "\n" appended. The apply must succeed and the
// re-plan must be empty, not a replacement. (B-054)
func TestAccKeychainSSHKeyPair_privateKeyNoTrailingNewline(t *testing.T) {
	name := acctest.RandName("tf-acc-kp-nl")
	priv, _ := genEd25519OpenSSHKeyPair(t, "tfacc")
	priv = strings.TrimRight(priv, "\n")
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_keychain_ssh_keypair" "test" {
  name        = %q
  private_key = %q
}
`, name, priv)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

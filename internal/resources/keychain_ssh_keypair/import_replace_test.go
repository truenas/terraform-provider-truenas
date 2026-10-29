// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package keychain_ssh_keypair_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// Import of a generate=true key pair: generate reads back null, it is
// RequiresReplace, so adopting the key replaces (regenerates) it.
func TestAccKeychainSSHKeyPair_importGenerated(t *testing.T) {
	name := acctest.RandName("tfhunt-kp")
	res := fmt.Sprintf(`
resource "truenas_keychain_ssh_keypair" "test" {
  name     = %q
  generate = true
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps:                    forgetImportNoop("truenas_keychain_ssh_keypair.test", "truenas_keychain_ssh_keypair", "name", name, acctest.ProviderConfig(), res),
	})
}

// Import of a supplied-key pair whose config says generate = false.
func TestAccKeychainSSHKeyPair_importSuppliedGenerateFalse(t *testing.T) {
	name := acctest.RandName("tfhunt-kp")
	priv, _ := genEd25519OpenSSHKeyPair(t, "tfhunt")
	res := fmt.Sprintf(`
resource "truenas_keychain_ssh_keypair" "test" {
  name        = %q
  generate    = false
  private_key = %q
}
`, name, priv)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps:                    forgetImportNoop("truenas_keychain_ssh_keypair.test", "truenas_keychain_ssh_keypair", "name", name, acctest.ProviderConfig(), res),
	})
}

// forgetImportNoop: create base+res, forget res via a removed block, then
// re-adopt it with an import block (id looked up through the data source) and
// require that the import plans as a pure no-op (no update, no replace).
func forgetImportNoop(addr, dsType, lookupAttr, lookup, base, res string) []resource.TestStep {
	removed := fmt.Sprintf(`
removed {
  from = %s
  lifecycle {
    destroy = false
  }
}
`, addr)
	imp := fmt.Sprintf(`
data %q "lookup" {
  %s = %q
}
import {
  to = %s
  id = tostring(data.%s.lookup.id)
}
`, dsType, lookupAttr, lookup, addr, dsType)
	return []resource.TestStep{
		{Config: base + res},
		{Config: base + removed},
		{
			Config: base + res + imp,
			ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
				// generate is never returned: recorded in place, never a replacement (fix C1).
				acctest.ExpectNoReplace(addr),
				acctest.ExpectOnlyAttrChanges(addr, "generate"),
			}},
		},
		{Config: base + res, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}
}

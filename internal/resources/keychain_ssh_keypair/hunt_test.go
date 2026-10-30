// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package keychain_ssh_keypair_test

// Adversarial bug-hunt tests: exercise both create paths' full attribute
// surface and assert the read-back re-plans empty, then import. The key
// material is generated in-test (genEd25519OpenSSHKeyPair, defined in the
// acceptance test) and the supplied/generated config builders are reused.
// "generate" has no wire counterpart and reads back null on import, so it is
// ignored there.

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// Supplied-key path: name + an OpenSSH private key, generate resolved to
// false, public_key derived server-side.
func TestHuntKeychainSSHKeyPair_allFieldsSupplied(t *testing.T) {
	acctest.DisruptiveCheck(t)

	name := acctest.RandName("tfhunt-kp-supplied")
	priv, _ := genEd25519OpenSSHKeyPair(t, "tfhunt")

	cfg := acctest.ProviderConfig() + testAccKeychainSSHKeyPairSuppliedConfig(name, priv)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckKeychainSSHKeyPairDestroyed,
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:            "truenas_keychain_ssh_keypair.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"generate"},
			},
		},
	})
}

// Generated path: name + generate=true, key pair produced on-box.
func TestHuntKeychainSSHKeyPair_allFieldsGenerated(t *testing.T) {
	acctest.DisruptiveCheck(t)

	name := acctest.RandName("tfhunt-kp-gen")

	cfg := acctest.ProviderConfig() + testAccKeychainSSHKeyPairGeneratedConfig(name)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckKeychainSSHKeyPairDestroyed,
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:            "truenas_keychain_ssh_keypair.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"generate"},
			},
		},
	})
}

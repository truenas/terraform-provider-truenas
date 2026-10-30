// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package api_key_test

// Adversarial bug-hunt test: set every API-accepted api_key attribute to a
// valid non-default value (name, owning username, and a non-midnight future
// expiry), apply, assert an empty re-plan, then import. The plaintext key is
// returned only at creation and can never be recovered on import, so it is
// ignored there. expires_at uses an explicit UTC "Z" form so the imported
// server read-back matches byte-for-byte.

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestHuntAPIKey_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)

	username := testAccAPIKeyUsername()
	name := acctest.RandName("tfhunt-key")

	cfg := acctest.ProviderConfig() + testHuntAPIKeyAllConfig(name, username)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAPIKeyDestroyed,
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:            "truenas_api_key.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"key"},
			},
		},
	})
}

func testHuntAPIKeyAllConfig(name, username string) string {
	return `
resource "truenas_api_key" "test" {
  name       = "` + name + `"
  username   = "` + username + `"
  expires_at = "2035-06-15T12:30:00Z"
}
`
}

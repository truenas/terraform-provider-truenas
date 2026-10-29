// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package api_key_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func expiresConfig(name, expires string) string {
	e := ""
	if expires != "" {
		e = fmt.Sprintf("expires_at = %q", expires)
	}
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_api_key" "test" {
  name     = %q
  username = %q
  %s
}
`, name, testAccAPIKeyUsername(), e)
}

// TestAccAPIKey_expiresUnset: removing expires_at from config plans an update
// that makes the key non-expiring again. (B-111)
func TestAccAPIKey_expiresUnset(t *testing.T) {
	name := acctest.RandName("tf-acc-key-exp")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: expiresConfig(name, "2035-01-01T00:00:00Z")},
			{
				Config: expiresConfig(name, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_api_key.test", plancheck.ResourceActionUpdate),
				}},
				Check: resource.TestCheckNoResourceAttr("truenas_api_key.test", "expires_at"),
			},
			{Config: expiresConfig(name, ""), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

// TestAccAPIKey_expiresOffset: an RFC3339 value with a non-Z offset, or with
// fractional seconds, reads back as the same instant in UTC "Z" form. The apply
// must succeed and re-plan empty. (B-059)
func TestAccAPIKey_expiresOffset(t *testing.T) {
	name := acctest.RandName("tf-acc-key-off")
	empty := resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: expiresConfig(name, "2035-01-01T02:00:00+02:00")},
			{Config: expiresConfig(name, "2035-01-01T02:00:00+02:00"), ConfigPlanChecks: empty},
			// The same instant in another spelling is not a change.
			{Config: expiresConfig(name, "2035-01-01T00:00:00.000Z"), ConfigPlanChecks: empty},
		},
	})
}

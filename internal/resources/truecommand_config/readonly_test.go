// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package truecommand_config_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// readOnlyProvider returns the only provider block of a step that runs with a
// READONLY_ADMIN API key (TRUENAS_RO_API_KEY / TRUENAS_RO_USERNAME), or skips
// when none is configured. A step's config must hold exactly one provider
// block: the in-process test provider is a single instance, so with an admin
// block and an aliased read-only block in the same config every resource and
// data source would run with whichever block was configured last.
func readOnlyProvider(t *testing.T) string {
	t.Helper()
	key, user := os.Getenv("TRUENAS_RO_API_KEY"), os.Getenv("TRUENAS_RO_USERNAME")
	if key == "" || user == "" {
		t.Skip("set TRUENAS_RO_API_KEY and TRUENAS_RO_USERNAME (a READONLY_ADMIN user's API key) to run")
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

func noMask(addr, attr string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[addr]
		if !ok {
			return fmt.Errorf("%s not in state", addr)
		}
		if v := rs.Primary.Attributes[attr]; v == "********" {
			return fmt.Errorf("%s.%s holds the READONLY_ADMIN redaction mask as data", addr, attr)
		}
		return nil
	}
}

// TestAccTrueCommandConfig_readOnlyAdmin guards B-029: a READONLY_ADMIN key
// gets api_key as "********" (even when no key is set). The data source must
// not report the mask, and a refresh of the resource with that key must not
// store it in state or plan a change.
func TestAccTrueCommandConfig_readOnlyAdmin(t *testing.T) {
	acctest.PreCheck(t)
	ro := readOnlyProvider(t)
	const r = "truenas_truecommand_config.test"
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: ro + `
data "truenas_truecommand_config" "x" {
}
`,
				Check: noMask("data.truenas_truecommand_config.x", "api_key"),
			},
			// Adopt with the full-admin key; nothing is configured, so nothing
			// is written.
			{Config: acctest.ProviderConfig() + `
resource "truenas_truecommand_config" "test" {}
`},
			{
				Config: ro + `
resource "truenas_truecommand_config" "test" {
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				Check:            noMask(r, "api_key"),
			},
			{Config: acctest.ProviderConfig() + `
resource "truenas_truecommand_config" "test" {}
`},
		},
	})
}

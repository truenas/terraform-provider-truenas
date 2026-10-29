// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_general_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
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

// TestAccSystemGeneral_readOnlyAdmin guards B-030: a READONLY_ADMIN key gets
// ui_certificate as "********" instead of the id. The data source must read
// (with ui_certificate null), and refreshing the resource with that key must
// keep the real id and plan nothing.
func TestAccSystemGeneral_readOnlyAdmin(t *testing.T) {
	acctest.PreCheck(t)
	ro := readOnlyProvider(t)
	const r = "truenas_system_general.test"
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: ro + `
data "truenas_system_general" "x" {
}
`,
				// ui_certificate_name is readable on 26.0+ only (25.10 redacts
				// the whole certificate object).
				Check: resource.TestCheckNoResourceAttr("data.truenas_system_general.x", "ui_certificate"),
			},
			// Adopt the singleton with the full-admin key (no attributes set,
			// so nothing is written), then refresh and plan with the
			// READONLY_ADMIN key.
			{Config: acctest.ProviderConfig() + `
resource "truenas_system_general" "test" {}
`, Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttrSet(r, "ui_certificate"),
				resource.TestCheckResourceAttrSet(r, "ui_certificate_name"),
			)},
			{
				Config: ro + `
resource "truenas_system_general" "test" {
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(r, "ui_certificate"),
					resource.TestCheckResourceAttrSet(r, "ui_certificate_name"),
				),
			},
			// Destroy with the full-admin provider (destroy makes no API call).
			{Config: acctest.ProviderConfig() + `
resource "truenas_system_general" "test" {}
`},
		},
	})
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ntp_server_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNTPServer_importWithForce: create with force=true (unreachable
// address), drop from state, import, and re-plan the same config: must be
// empty. force is write-only, so import has nothing to miss (B-116).
func TestAccNTPServer_importWithForce(t *testing.T) {
	acctest.DisruptiveCheck(t)
	cfg := acctest.ProviderConfig() + `
resource "truenas_ntp_server" "test" {
  address = "192.0.2.55"
  force   = true
  prefer  = false
}
`
	var id string
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, Check: func(s *terraform.State) error {
				id = s.RootModule().Resources["truenas_ntp_server.test"].Primary.ID
				return nil
			}},
			{Config: acctest.ProviderConfig() + `
removed {
  from = truenas_ntp_server.test
  lifecycle {
    destroy = false
  }
}
`},
			{Config: cfg, ResourceName: "truenas_ntp_server.test", ImportState: true, ImportStatePersist: true,
				ImportStateIdFunc: func(*terraform.State) (string, error) { return id, nil }},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
			}},
		},
	})
}

// TestAccNTPServerDataSource_ambiguous guards B-131: TrueNAS accepts two NTP
// servers with the same address, and the data source must then fail rather
// than return one of them at random.
func TestAccNTPServerDataSource_ambiguous(t *testing.T) {
	acctest.DisruptiveCheck(t)
	servers := acctest.ProviderConfig() + `
resource "truenas_ntp_server" "a" {
  address = "192.0.2.78"
  force   = true
}
resource "truenas_ntp_server" "b" {
  address = "192.0.2.78"
  force   = true
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: servers},
			{
				Config: servers + `
data "truenas_ntp_server" "x" {
  address    = "192.0.2.78"
  depends_on = [truenas_ntp_server.a, truenas_ntp_server.b]
}
`,
				ExpectError: regexp.MustCompile(`Several\s+NTP\s+servers\s+match`),
			},
		},
	})
}

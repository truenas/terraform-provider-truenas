// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ntp_server_test

// Adversarial bug-hunt tests: apply every API-accepted value and assert an
// empty re-plan, then import. ntp_server is a list resource; Terraform
// destroys the created server on cleanup.

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// Every bool on, min/max poll at the edges, force to accept an unreachable
// server. Assert the read-back re-plans empty, then import.
func TestHuntNTPServer_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)
	c := acctest.ProviderConfig() + `
resource "truenas_ntp_server" "test" {
  address = "hunt.ntp.example.com"
  burst   = true
  iburst  = true
  prefer  = true
  minpoll = 6
  maxpoll = 9
  force   = true
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c},
			{Config: c, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:            "truenas_ntp_server.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"force"},
			},
		},
	})
}

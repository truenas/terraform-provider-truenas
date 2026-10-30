// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package reporting_exporter_test

// Adversarial bug-hunt test: apply every API-accepted value and assert an
// empty re-plan, then import. reporting_exporter is a create/delete resource
// (not a singleton), so it is created under its own random name and cleaned
// up by CheckDestroy — no box-wide config is mutated, matching the existing
// acceptance test's non-disruptive contract (no acctest.DisruptiveCheck).
// testAccCheckReportingExporterDestroyed is defined in acceptance_test.go.

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntReportingExporter_allFields sets every GRAPHITE attribute (required
// and optional) to a non-default value and asserts the read-back re-plans
// empty, then imports. The destination is the RFC 5737 TEST-NET-1
// documentation range so nothing real is ever contacted even with enabled =
// true.
func TestHuntReportingExporter_allFields(t *testing.T) {
	name := acctest.RandName("tf-hunt-exporter")
	c := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_reporting_exporter" "test" {
  name    = %q
  enabled = true
  attributes = {
    destination_ip            = "192.0.2.50"
    destination_port          = 2004
    namespace                 = "tfhunt"
    prefix                    = "tfhuntprefix"
    update_every              = 5
    buffer_on_failures        = 20
    send_names_instead_of_ids = false
    matching_charts           = "cpu.*"
  }
}
`, name)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckReportingExporterDestroyed(name),
		Steps: []resource.TestStep{
			{Config: c},
			{Config: c, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:      "truenas_reporting_exporter.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

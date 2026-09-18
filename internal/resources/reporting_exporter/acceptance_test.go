// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package reporting_exporter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccReportingExporter_basic exercises the full Tier 1 contract for a
// GRAPHITE reporting exporter: create pointed at the 192.0.2.0/24 TEST-NET-1
// documentation range (RFC 5737) so nothing real is ever contacted, update
// in place, import, and destroy verification via a live
// reporting.exporters.query.
func TestAccReportingExporter_basic(t *testing.T) {
	name := acctest.RandName("tf-acc-exporter")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckReportingExporterDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccReportingExporterConfig(name, false, 2003),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_reporting_exporter.test", "id"),
					resource.TestCheckResourceAttr("truenas_reporting_exporter.test", "name", name),
					resource.TestCheckResourceAttr("truenas_reporting_exporter.test", "enabled", "false"),
					resource.TestCheckResourceAttr("truenas_reporting_exporter.test", "attributes.destination_ip", "192.0.2.50"),
					resource.TestCheckResourceAttr("truenas_reporting_exporter.test", "attributes.destination_port", "2003"),
					resource.TestCheckResourceAttr("truenas_reporting_exporter.test", "attributes.namespace", "tfacc"),
					resource.TestCheckResourceAttr("truenas_reporting_exporter.test", "attributes.prefix", "scale"),
					resource.TestCheckResourceAttr("truenas_reporting_exporter.test", "attributes.update_every", "1"),
					resource.TestCheckResourceAttr("truenas_reporting_exporter.test", "attributes.buffer_on_failures", "10"),
					resource.TestCheckResourceAttr("truenas_reporting_exporter.test", "attributes.send_names_instead_of_ids", "true"),
					resource.TestCheckResourceAttr("truenas_reporting_exporter.test", "attributes.matching_charts", "*"),
				),
			},
			// Update in place: toggle enabled and change the destination port.
			{
				Config: acctest.ProviderConfig() + testAccReportingExporterConfig(name, true, 2004),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_reporting_exporter.test", "enabled", "true"),
					resource.TestCheckResourceAttr("truenas_reporting_exporter.test", "attributes.destination_port", "2004"),
				),
			},
			{
				ResourceName:      "truenas_reporting_exporter.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccReportingExporterConfig(name string, enabled bool, port int) string {
	return fmt.Sprintf(`
resource "truenas_reporting_exporter" "test" {
  name    = %q
  enabled = %v
  attributes = {
    destination_ip   = "192.0.2.50"
    destination_port = %d
    namespace         = "tfacc"
  }
}
`, name, enabled, port)
}

// reportingExporterSummary is the subset of reporting.exporters.query fields
// this test package needs directly (outside of the provider's own resource
// code).
type reportingExporterSummary struct {
	ID int64 `json:"id"`
}

func testAccCheckReportingExporterDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "reporting.exporters.query", [][]any{{"name", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking reporting exporter %s: %v", name, err)
		}
		var results []reportingExporterSummary
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing reporting.exporters.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("reporting exporter %s still exists", name)
		}
		return nil
	}
}

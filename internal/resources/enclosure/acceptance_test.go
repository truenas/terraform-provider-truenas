// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package enclosure_test

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// firstEnclosureID returns the id of the first enclosure reported by an
// unfiltered enclosure2.query, skipping the test if the box reports none —
// probed live: this is exactly what a box with no enclosure hardware/
// Enterprise HA license returns (e.g. the cross-release 26.0 box used only
// for shape probing elsewhere in this provider's test matrix), a clean
// empty array rather than an error. Deliberately does NOT hardcode an id:
// probed live during this task, the id actually present on the disposable
// HA test box did not match an earlier probe's recorded value for the same
// physical enclosure, confirming VirtualSES ids are not a fixed constant
// across boots/probes.
func firstEnclosureID(t *testing.T) string {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "enclosure2.query")
	if err != nil {
		t.Fatalf("error reading enclosure2.query: %v", err)
	}
	var results []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &results); err != nil {
		t.Fatalf("error parsing enclosure2.query response: %v", err)
	}
	if len(results) == 0 {
		t.Skip("enclosure2.query reports no enclosures on this box")
	}
	return results[0].ID
}

// TestAccEnclosureDataSource_basic reads a real enclosure's descriptor
// through the truenas_enclosure datasource only. It never writes. Gated by
// acctest.HACheck: requires TRUENAS_HA=1, TRUENAS_HA_ALLOWED_ENDPOINT
// matching TRUENAS_ENDPOINT, and a live failover.licensed=true probe — see
// acctest.HACheck's doc comment.
func TestAccEnclosureDataSource_basic(t *testing.T) {
	acctest.HACheck(t)
	id := firstEnclosureID(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccEnclosureDataSourceConfig(id),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_enclosure.test", "id", id),
					resource.TestCheckResourceAttrSet("data.truenas_enclosure.test", "name"),
					resource.TestCheckResourceAttrSet("data.truenas_enclosure.test", "label"),
					resource.TestCheckResourceAttrSet("data.truenas_enclosure.test", "model"),
					resource.TestCheckResourceAttrSet("data.truenas_enclosure.test", "vendor"),
					resource.TestCheckResourceAttrSet("data.truenas_enclosure.test", "product"),
					resource.TestCheckResourceAttrSet("data.truenas_enclosure.test", "controller"),
					resource.TestCheckResourceAttrSet("data.truenas_enclosure.test", "status.#"),
				),
			},
		},
	})
}

// TestAccEnclosureDataSource_notFound verifies the id-filtered
// enclosure2.query call is surfaced as a clean, informative "not found"
// error rather than a crash or a confusing empty-object read — the same
// outcome probed live on the cross-release 26.0 box (no enclosure hardware/
// license there: enclosure2.query returns an empty array for every id).
// Read-only: never mutates anything.
func TestAccEnclosureDataSource_notFound(t *testing.T) {
	acctest.HACheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      acctest.ProviderConfig() + testAccEnclosureDataSourceConfig("tf-acc-nonexistent-enclosure-id"),
				ExpectError: regexp.MustCompile(`(?i)not found`),
			},
		},
	})
}

func testAccEnclosureDataSourceConfig(id string) string {
	return `
data "truenas_enclosure" "test" {
  id = ` + `"` + id + `"` + `
}
`
}

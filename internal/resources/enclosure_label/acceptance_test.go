// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package enclosure_label_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// firstEnclosureID returns the id of the first enclosure reported by an
// unfiltered enclosure2.query, skipping the test if the box reports none.
// Deliberately independent from the sibling internal/resources/enclosure
// package's identically-named test helper: this provider has no
// cross-package imports, including between _test packages (see this
// resource's model.go doc comments for why each package stays
// self-contained).
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

// readEnclosureLabel reads an enclosure's current "label" via
// acctest.RestoreCall (id-filtered enclosure2.query), so the test can
// capture the original before it runs and verify the restored value
// afterward, independent of anything the truenas_enclosure_label resource
// itself reports.
func readEnclosureLabel(t *testing.T, id string) string {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "enclosure2.query", [][]any{{"id", "=", id}})
	if err != nil {
		t.Fatalf("error reading enclosure2.query for id %q: %v", id, err)
	}
	var results []struct {
		Label string `json:"label"`
	}
	if err := json.Unmarshal(raw, &results); err != nil {
		t.Fatalf("error parsing enclosure2.query response: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("enclosure2.query returned no result for id %q", id)
	}
	return results[0].Label
}

// restoreEnclosureLabel sends label back to the enclosure via
// enclosure.label.set. Probed live: this call is synchronous (job: false)
// and the new value is visible in the very next enclosure2.query read — no
// settle-time/polling behavior like ipmi_lan's BMC network fields.
func restoreEnclosureLabel(t *testing.T, id, label string) {
	t.Helper()
	if _, err := acctest.RestoreCall(context.Background(), "enclosure.label.set", id, label); err != nil {
		t.Fatalf("RESTORE FAILED for enclosure %q label (want %q): %v", id, label, err)
	}
}

// TestAccEnclosureLabel_setAndRestore drives the truenas_enclosure_label
// resource's "label" field through a fresh RandName value, verifies it
// round-trips via Terraform (apply + import), then lets Terraform's own
// implicit end-of-test Destroy run — which, per this resource's own
// restore-on-destroy contract (see schema.go's resourceSchema
// Description), must independently restore the enclosure's ORIGINAL label.
// That is verified via a direct, live enclosure2.query read AFTER
// resource.Test returns, per this task's explicit acceptance requirement
// ("destroy → verify ORIGINAL label restored via live query").
//
// A t.Cleanup-registered direct-API restore is also registered BEFORE any
// mutating Terraform apply, as the safety net if the Terraform run itself
// (or the resource's own Delete) fails to complete — matching this
// project's set-and-restore house convention. Since it runs AFTER the
// explicit post-Test live-query assertion below, it is redundant in the
// success path (the resource's own Delete already restored the label) and
// only matters if something upstream failed.
//
// Gated by acctest.HACheck only (no acctest.DisruptiveCheck): unlike
// failover_config's "timeout" (live HA state) or ipmi_lan's "vlan"
// (out-of-band BMC network reachability), an enclosure's cosmetic "label"
// carries no risk of breaking connectivity or HA behavior, and the restore
// is both synchronous (confirmed live, no settle-time) and independently
// verified below — matching this task's own acceptance requirement
// ("Acceptance: HACheck; set label ... verify, restore"), which does not
// call for the additional TRUENAS_DISRUPTIVE=1 gate the other two
// resources' higher-risk mutations require.
func TestAccEnclosureLabel_setAndRestore(t *testing.T) {
	acctest.HACheck(t)

	id := firstEnclosureID(t)
	orig := readEnclosureLabel(t, id)
	t.Cleanup(func() { restoreEnclosureLabel(t, id, orig) })

	newLabel := acctest.RandName("tf-acc-enclosure")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccEnclosureLabelConfig(id, newLabel),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_enclosure_label.test", "id", id),
					resource.TestCheckResourceAttr("truenas_enclosure_label.test", "label", newLabel),
					resource.TestCheckResourceAttrSet("truenas_enclosure_label.test", "name"),
					testAccCheckEnclosureLabelLive(t, id, newLabel),
				),
			},
			{
				ResourceName:      "truenas_enclosure_label.test",
				ImportState:       true,
				ImportStateId:     id,
				ImportStateVerify: true,
			},
		},
	})

	got := readEnclosureLabel(t, id)
	if got != orig {
		t.Errorf("after Terraform destroy, enclosure %q label = %q, want restored original %q", id, got, orig)
	}
}

// testAccCheckEnclosureLabelLive returns a resource.TestCheckFunc that
// verifies the enclosure's label via a direct, live enclosure2.query read —
// independent confirmation that enclosure.label.set's effect is real and
// not merely reflected in Terraform's own state.
func testAccCheckEnclosureLabelLive(t *testing.T, id, want string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		if got := readEnclosureLabel(t, id); got != want {
			t.Errorf("live enclosure2.query label = %q, want %q", got, want)
		}
		return nil
	}
}

func testAccEnclosureLabelConfig(id, label string) string {
	return `
resource "truenas_enclosure_label" "test" {
  id    = ` + `"` + id + `"` + `
  label = ` + `"` + label + `"` + `
}
`
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_global_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNVMeTGlobalDataSource_basic reads the current TrueNAS NVMe-oF global
// configuration through the truenas_nvmet_global datasource only. It never
// writes: the NVMe-oF global configuration serves live storage, and the
// box's real configuration must not be overwritten by an acceptance test
// run.
//
// Requires:
//   - TF_ACC=1
//   - TRUENAS_API_KEY (or TRUENAS_USERNAME+TRUENAS_PASSWORD) set for
//     acctest.PreCheck
//   - A running TrueNAS instance
func TestAccNVMeTGlobalDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_nvmet_global" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_nvmet_global.test", "id", "nvmet_global"),
					resource.TestCheckResourceAttrSet("data.truenas_nvmet_global.test", "basenqn"),
				),
			},
		},
	})
}

// nvmetGlobalOriginal captures the one field
// TestAccNVMeTGlobal_setAndRestore touches.
type nvmetGlobalOriginal struct {
	XportReferral bool `json:"xport_referral"`
}

// readNVMeTGlobalOriginal reads the box's current xport_referral via
// nvmet.global.config, so the test can restore it exactly afterward.
func readNVMeTGlobalOriginal(t *testing.T) nvmetGlobalOriginal {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "nvmet.global.config")
	if err != nil {
		t.Fatalf("error reading current nvmet global config: %v", err)
	}
	var orig nvmetGlobalOriginal
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing nvmet.global.config response: %v", err)
	}
	return orig
}

// restoreNVMeTGlobal sends only xport_referral back to its original value
// via nvmet.global.update. It runs from t.Cleanup, so it restores the box
// even if the Terraform steps themselves fail partway through. basenqn,
// ana, kernel, and rdma are never touched by this test.
func restoreNVMeTGlobal(t *testing.T, orig nvmetGlobalOriginal) {
	t.Helper()
	if _, err := acctest.Client().Call(context.Background(), "nvmet.global.update", map[string]any{
		"xport_referral": orig.XportReferral,
	}); err != nil {
		t.Fatalf("error restoring nvmet global xport_referral: %v", err)
	}
}

// TestAccNVMeTGlobal_setAndRestore drives the singleton
// truenas_nvmet_global resource's "xport_referral" field through its
// opposite value and back to the value read from the box before the test
// ran, then imports it. It requires TF_ACC=1 and TRUENAS_DISRUPTIVE=1
// (acctest.DisruptiveCheck), since it mutates the box's live NVMe-oF global
// configuration; a t.Cleanup-registered API restore is the safety net if
// the Terraform steps fail.
//
// Safety: xport_referral only controls whether NVMe-oF discovery responses
// advertise port referrals (an advisory discovery-log hint), and does not
// gate or restart any port or subsystem; the box's NVMe-oF ports and
// subsystems used by other acceptance tests in this suite are unaffected by
// toggling it. basenqn, ana, kernel, and rdma (which can disrupt active
// NVMe-oF connections or discovery) are never touched.
func TestAccNVMeTGlobal_setAndRestore(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readNVMeTGlobalOriginal(t)
	t.Cleanup(func() { restoreNVMeTGlobal(t, orig) })

	testValue := !orig.XportReferral

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccNVMeTGlobalConfig(testValue),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nvmet_global.test", "id", "nvmet_global"),
					resource.TestCheckResourceAttr("truenas_nvmet_global.test", "xport_referral", fmt.Sprintf("%t", testValue)),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccNVMeTGlobalConfig(orig.XportReferral),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nvmet_global.test", "xport_referral", fmt.Sprintf("%t", orig.XportReferral)),
				),
			},
			{
				ResourceName:      "truenas_nvmet_global.test",
				ImportState:       true,
				ImportStateId:     "nvmet_global",
				ImportStateVerify: true,
			},
		},
	})
}

func testAccNVMeTGlobalConfig(xportReferral bool) string {
	return fmt.Sprintf(`
resource "truenas_nvmet_global" "test" {
  xport_referral = %t
}
`, xportReferral)
}

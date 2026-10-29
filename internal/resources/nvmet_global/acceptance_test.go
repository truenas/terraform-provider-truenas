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

// nvmetGlobalOriginal captures every writable field of the singleton, so
// the test can put the box back exactly as it found it.
type nvmetGlobalOriginal struct {
	Basenqn       string `json:"basenqn"`
	ANA           bool   `json:"ana"`
	Kernel        bool   `json:"kernel"`
	RDMA          bool   `json:"rdma"`
	XportReferral bool   `json:"xport_referral"`
}

// readNVMeTGlobalOriginal reads the box's current NVMe-oF global config via
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

// restoreNVMeTGlobal sends every writable field back to its original value
// via nvmet.global.update, then reads the config back and fails the test if
// anything differs. It runs from t.Cleanup, so it restores the box even if
// the Terraform steps themselves fail partway through.
func restoreNVMeTGlobal(t *testing.T, orig nvmetGlobalOriginal) {
	t.Helper()
	if _, err := acctest.RestoreCall(context.Background(), "nvmet.global.update", map[string]any{
		"basenqn":        orig.Basenqn,
		"ana":            orig.ANA,
		"kernel":         orig.Kernel,
		"rdma":           orig.RDMA,
		"xport_referral": orig.XportReferral,
	}); err != nil {
		t.Fatalf("error restoring nvmet global config: %v", err)
	}
	if now := readNVMeTGlobalOriginal(t); now != orig {
		t.Fatalf("nvmet global config not restored: got %+v, want %+v", now, orig)
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
// gate or restart any port or subsystem. The config pins basenqn, ana,
// kernel and rdma to the values read from the box, so they never change
// (the test used to hardcode rdma = false, which turned RDMA off on a box
// serving an RDMA port and left it off), and the cleanup restores and
// verifies all five fields.
func TestAccNVMeTGlobal_setAndRestore(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readNVMeTGlobalOriginal(t)
	t.Cleanup(func() { restoreNVMeTGlobal(t, orig) })

	testValue := !orig.XportReferral

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccNVMeTGlobalConfig(orig, testValue),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nvmet_global.test", "id", "nvmet_global"),
					resource.TestCheckResourceAttr("truenas_nvmet_global.test", "xport_referral", fmt.Sprintf("%t", testValue)),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccNVMeTGlobalConfig(orig, orig.XportReferral),
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

// testAccNVMeTGlobalConfig sets xport_referral and keeps every other field
// at the box's original value.
func testAccNVMeTGlobalConfig(orig nvmetGlobalOriginal, xportReferral bool) string {
	return fmt.Sprintf(`
resource "truenas_nvmet_global" "test" {
  xport_referral = %t

  ana     = %t
  basenqn = %q
  kernel  = %t
  rdma    = %t
}
`, xportReferral, orig.ANA, orig.Basenqn, orig.Kernel, orig.RDMA)
}

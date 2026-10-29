// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_device_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccVMDevice_readOnlyRedaction reads a DISPLAY device through a
// READONLY_ADMIN API key (TRUENAS_RO_API_KEY / TRUENAS_RO_USERNAME; skipped
// when unset). Such a session gets the password as "********". The resource,
// refreshed through the read-only provider, must plan no change (before the
// fix Read saw the mask as drift and rewrote state with it), and the data
// source must report the password as null, never the mask. (B-033)
func TestAccVMDevice_readOnlyRedaction(t *testing.T) {
	roKey, roUser := os.Getenv("TRUENAS_RO_API_KEY"), os.Getenv("TRUENAS_RO_USERNAME")
	if roKey == "" || roUser == "" {
		t.Skip("TRUENAS_RO_API_KEY and TRUENAS_RO_USERNAME (a READONLY_ADMIN key) are required")
	}
	vmName := "tffix3virtvmr" + strings.ReplaceAll(acctest.RandName(""), "-", "")
	// The in-process test provider serves every alias from one instance, so
	// two differently-credentialed provider blocks would share whichever was
	// configured last. Step 2 therefore configures only the read-only one.
	roProvider := fmt.Sprintf(`
provider "truenas" {
  endpoint = %q
  insecure = true
  api_key  = %q
  username = %q
}
`, acctest.Endpoint(), roKey, roUser)
	dev := `
resource "truenas_vm_device" "disp" {
  vm         = truenas_vm.test.id
  attributes = jsonencode({ dtype = "DISPLAY", type = "SPICE", bind = "0.0.0.0", port = 15930, web_port = 15931 })
  attributes_secrets_wo         = jsonencode({ password = "tffix3pw" })
  attributes_secrets_wo_version = 1
}
`
	cfg := acctest.ProviderConfig() + vmWithDevices(vmName, dev)
	roCfg := roProvider + vmWithDevices(vmName, dev+`
data "truenas_vm_device" "ro" {
  id = truenas_vm_device.disp.id
}
`)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				Config: roCfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_vm_device.disp", plancheck.ResourceActionNoop),
					plancheck.ExpectResourceAction("truenas_vm.test", plancheck.ResourceActionNoop),
				}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("data.truenas_vm_device.ro", "attributes", func(v string) error {
						if strings.Contains(v, "********") {
							return fmt.Errorf("data source returned the redaction mask as data: %s", v)
						}
						if !strings.Contains(v, `"password":null`) {
							return fmt.Errorf("data source attributes %s: want password null", v)
						}
						return nil
					}),
					resource.TestCheckResourceAttrWith("truenas_vm_device.disp", "attributes", func(v string) error {
						if strings.Contains(v, "********") {
							return fmt.Errorf("resource state holds the redaction mask: %s", v)
						}
						return nil
					}),
				),
			},
			// Back to the full-admin provider, so the post-test destroy is allowed.
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

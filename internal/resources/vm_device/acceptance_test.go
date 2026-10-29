// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_device_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccVMDevice_basic tests create, update, and import of a VM device (a
// DISPLAY device attached to its own RandName-suffixed, non-running,
// non-autostart VM fixture). It never touches any pre-existing VM.
func TestAccVMDevice_basic(t *testing.T) {
	vmName := "tfaccvmdev" + strings.ReplaceAll(acctest.RandName(""), "-", "") // vm_create.name: alphanumeric only

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckVMDeviceDestroyed,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccVMDeviceConfig(vmName, 0),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_vm_device.test", "id"),
					resource.TestCheckResourceAttrPair("truenas_vm_device.test", "vm", "truenas_vm.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_vm_device.test", "order"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccVMDeviceConfig(vmName, 5),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_vm_device.test", "order", "5"),
				),
			},
		},
	})
}

// TestAccVMDevice_importNoReplace guards GH #30: after import, a plan against
// the same config must not diff `attributes`. The imported state carries the
// server-normalized attributes blob; without the keepAttributesIfConfigMatchesState
// plan modifier the config's jsonencode differs from it and the plan shows an
// update (or churn). The empty-plan re-apply asserts it stays a no-op. The
// import-verify step ignores `attributes` because the server re-normalizes the
// JSON (key order, defaults) so the raw string cannot be state-compared.
// attributes_secrets_wo_version reads back null after import, so the first
// apply records it in place and sends the password overlay once.
func TestAccVMDevice_importNoReplace(t *testing.T) {
	vmName := "tfaccvmdevimp" + strings.ReplaceAll(acctest.RandName(""), "-", "")
	config := acctest.ProviderConfig() + testAccVMDeviceConfig(vmName, 5)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckVMDeviceDestroyed,
		Steps: append([]resource.TestStep{
			{Config: config},
		}, acctest.ImportReapplyNoop("truenas_vm_device.test", config, "attributes", "attributes_secrets_wo_version")...),
	})
}

func testAccVMDeviceConfig(vmName string, order int) string {
	orderLine := ""
	if order != 0 {
		orderLine = fmt.Sprintf("  order = %d\n", order)
	}
	return fmt.Sprintf(`
resource "truenas_vm" "test" {
  name      = %q
  memory    = 512
  vcpus     = 1
  autostart = false
  running   = false
}

resource "truenas_vm_device" "test" {
  vm = truenas_vm.test.id
  attributes = jsonencode({
    dtype      = "DISPLAY"
    type       = "SPICE"
    bind       = "0.0.0.0"
    resolution = "1024x768"
    wait       = false
    web        = false
    # Distinct SPICE/web ports. TrueNAS requires a password for display
    # devices; it goes in the write-only overlay, never in attributes.
    port     = 15900
    web_port = 15901
  })
  attributes_secrets_wo         = jsonencode({ password = "tfacc-spice-pw" })
  attributes_secrets_wo_version = 1
%s}
`, vmName, orderLine)
}

func testAccCheckVMDeviceDestroyed(s *terraform.State) error {
	rs, ok := s.RootModule().Resources["truenas_vm_device.test"]
	if !ok {
		return fmt.Errorf("resource truenas_vm_device.test not found in pre-destroy state")
	}
	idStr, ok := rs.Primary.Attributes["id"]
	if !ok {
		return fmt.Errorf("truenas_vm_device.test has no id attribute in pre-destroy state")
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return fmt.Errorf("parsing vm device id %q: %v", idStr, err)
	}

	c := acctest.Client()
	// CallRead (not Call) so the long-idle shared acctest client reconnects if
	// it went stale during the Terraform steps — a read query is idempotent.
	raw, err := c.CallRead(context.Background(), "vm.device.query", [][]any{{"id", "=", id}})
	if err != nil {
		return fmt.Errorf("error checking vm device id=%d: %v", id, err)
	}
	var results []struct {
		ID any `json:"id"`
	}
	if err := json.Unmarshal(raw, &results); err != nil {
		return fmt.Errorf("error parsing vm.device.query response: %v", err)
	}
	if len(results) > 0 {
		return fmt.Errorf("vm device id=%d still exists", id)
	}
	return nil
}

func testAccVMDeviceWOConfig(vmName, secret string) string {
	return fmt.Sprintf(`
resource "truenas_vm" "test" {
  name      = %q
  memory    = 512
  vcpus     = 1
  autostart = false
  running   = false
}

resource "truenas_vm_device" "test" {
  vm = truenas_vm.test.id
  # Non-secret attributes in the base; the SPICE password is supplied write-only.
  attributes = jsonencode({
    dtype      = "DISPLAY"
    type       = "SPICE"
    bind       = "0.0.0.0"
    resolution = "1024x768"
    wait       = false
    web        = false
    port       = 15900
    web_port   = 15901
  })
  attributes_secrets_wo         = jsonencode({ password = %q })
  attributes_secrets_wo_version = 1
}
`, vmName, secret)
}

// TestAccVMDevice_attributesSecretsWO guards the write-only secrets path (#36/#37
// class): the SPICE password is supplied via attributes_secrets_wo, merged into
// the API payload server-side, and must NOT appear in state — neither in the
// write-only attribute (always null in state) nor absorbed into `attributes`.
func TestAccVMDevice_attributesSecretsWO(t *testing.T) {
	vmName := "tfaccvmdevwo" + strings.ReplaceAll(acctest.RandName(""), "-", "")
	const secret = "tfacc-wo-spice-pw"
	cfg := acctest.ProviderConfig() + testAccVMDeviceWOConfig(vmName, secret)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckVMDeviceDestroyed,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					// Apply succeeded => the API accepted the merged payload (password included).
					resource.TestCheckResourceAttrSet("truenas_vm_device.test", "id"),
					// The write-only attribute is never stored.
					resource.TestCheckNoResourceAttr("truenas_vm_device.test", "attributes_secrets_wo"),
					// The secret must not be absorbed into the base attributes in state.
					resource.TestCheckResourceAttrWith("truenas_vm_device.test", "attributes", func(v string) error {
						if strings.Contains(v, secret) {
							return fmt.Errorf("secret leaked into attributes state: %s", v)
						}
						return nil
					}),
				),
			},
			// Re-apply the same config: no drift (the projected read excludes the secret).
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

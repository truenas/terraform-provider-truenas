package vm_device_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccVMDevice_basic tests create, update, and import of a VM device
// (a CDROM device attached to a pre-existing VM).
// It requires:
//   - TrueNAS_API_KEY set in the environment (checked by acctest.PreCheck)
//   - A running TrueNAS SCALE instance
//   - TF_ACC_VM_ID set to the ID of an existing VM to attach the device to
func TestAccVMDevice_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	vmID := os.Getenv("TF_ACC_VM_ID")
	if vmID == "" {
		t.Skip("TF_ACC_VM_ID not set: skipping truenas_vm_device acceptance test")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccVMDeviceConfig(vmID, `{"dtype": "CDROM", "path": "/mnt/tank/isos/tf-acc.iso"}`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_vm_device.test", "id"),
					resource.TestCheckResourceAttr("truenas_vm_device.test", "vm", vmID),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccVMDeviceConfig(vmID, `{"dtype": "CDROM", "path": "/mnt/tank/isos/tf-acc-updated.iso"}`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_vm_device.test", "id"),
				),
			},
			{
				ResourceName:            "truenas_vm_device.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"attributes"},
			},
		},
	})
}

func testAccVMDeviceConfig(vmID, attributes string) string {
	return fmt.Sprintf(`
resource "truenas_vm_device" "test" {
  vm         = %s
  attributes = %q
}
`, vmID, attributes)
}

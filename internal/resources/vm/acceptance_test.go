package vm_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccVM_basic tests create, update (including start/stop), and import of
// a VM. It requires TF_ACC=1 and credentials set via TRUENAS_API_KEY or
// TRUENAS_USERNAME+TRUENAS_PASSWORD.
func TestAccVM_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccVMConfig("vm-test", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_vm.test", "name", "vm-test"),
					resource.TestCheckResourceAttr("truenas_vm.test", "memory", "1073741824"),
					resource.TestCheckResourceAttr("truenas_vm.test", "running", "false"),
					resource.TestCheckResourceAttrSet("truenas_vm.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_vm.test", "status"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccVMConfig("vm-test", true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_vm.test", "running", "true"),
					resource.TestCheckResourceAttr("truenas_vm.test", "status", "RUNNING"),
				),
			},
			{
				ResourceName:            "truenas_vm.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"running"},
			},
		},
	})
}

func testAccVMConfig(name string, running bool) string {
	return fmt.Sprintf(`
resource "truenas_vm" "test" {
  name    = %q
  memory  = 1073741824
  vcpus   = 1
  cores   = 1
  threads = 1
  running = %v
}
`, name, running)
}

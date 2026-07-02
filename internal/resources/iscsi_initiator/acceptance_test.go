package iscsi_initiator_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccISCSIInitiator_basic tests create, update, and import of an iSCSI
// initiator group.
// It requires:
//   - TrueNAS_API_KEY set in the environment (checked by acctest.PreCheck)
//   - A running TrueNAS SCALE instance
func TestAccISCSIInitiator_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccISCSIInitiatorConfig("tf-acc-initiator", []string{}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_initiator.test", "comment", "tf-acc-initiator"),
					resource.TestCheckResourceAttrSet("truenas_iscsi_initiator.test", "id"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccISCSIInitiatorConfig("tf-acc-initiator-updated", []string{"iqn.2023-01.com.example:host1"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_initiator.test", "comment", "tf-acc-initiator-updated"),
					resource.TestCheckResourceAttr("truenas_iscsi_initiator.test", "initiators.0", "iqn.2023-01.com.example:host1"),
				),
			},
			{
				ResourceName:      "truenas_iscsi_initiator.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccISCSIInitiatorConfig(comment string, initiators []string) string {
	initsStr := "[]"
	if len(initiators) > 0 {
		initsStr = fmt.Sprintf(`["%s"]`, initiators[0])
		for _, iqn := range initiators[1:] {
			initsStr = fmt.Sprintf(`%s, "%s"`, initsStr, iqn)
		}
	}
	return fmt.Sprintf(`
resource "truenas_iscsi_initiator" "test" {
  comment    = %q
  initiators = %s
}
`, comment, initsStr)
}

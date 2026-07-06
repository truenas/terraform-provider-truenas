package iscsi_targetextent_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccISCSITargetExtent_basic tests create, update, and import of an
// iSCSI target/extent association.
// It requires:
//   - TrueNAS_API_KEY set in the environment (checked by acctest.PreCheck)
//   - A zvol at zvol/tank/iscsi-test-vol to exist on the target TrueNAS host
func TestAccISCSITargetExtent_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure zvol/tank/iscsi-test-vol exists")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccISCSITargetExtentConfig(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_iscsi_targetextent.test", "id"),
					resource.TestCheckResourceAttrPair("truenas_iscsi_targetextent.test", "target", "truenas_iscsi_target.test", "id"),
					resource.TestCheckResourceAttrPair("truenas_iscsi_targetextent.test", "extent", "truenas_iscsi_extent.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_iscsi_targetextent.test", "lunid"),
				),
			},
			{
				ResourceName:      "truenas_iscsi_targetextent.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccISCSITargetExtentConfig() string {
	return `
resource "truenas_iscsi_target" "test" {
  name = "tf-test-targetextent"
}

resource "truenas_iscsi_extent" "test" {
  name    = "tf-test-targetextent"
  type    = "DISK"
  disk    = "zvol/tank/iscsi-test-vol"
  enabled = true
}

resource "truenas_iscsi_targetextent" "test" {
  target = truenas_iscsi_target.test.id
  extent = truenas_iscsi_extent.test.id
}
`
}

package nfs_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestAccNFSShare_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccNFSShareConfig("/mnt/tank/nfs-test", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nfs_share.test", "path", "/mnt/tank/nfs-test"),
					resource.TestCheckResourceAttr("truenas_nfs_share.test", "enabled", "true"),
					resource.TestCheckResourceAttrSet("truenas_nfs_share.test", "id"),
				),
			},
			{
				Config: testAccNFSShareConfig("/mnt/tank/nfs-test", true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nfs_share.test", "ro", "true"),
				),
			},
			{
				ResourceName:      "truenas_nfs_share.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccNFSShareConfig(path string, ro bool) string {
	return fmt.Sprintf(`
resource "truenas_nfs_share" "test" {
  path = %q
  ro   = %v
}
`, path, ro)
}

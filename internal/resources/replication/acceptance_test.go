package replication_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestAccReplication_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccReplicationConfig(true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_replication_task.test", "name", "tf-acc-replication"),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "direction", "PUSH"),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "transport", "LOCAL"),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "enabled", "true"),
					resource.TestCheckResourceAttrSet("truenas_replication_task.test", "id"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccReplicationConfig(false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_replication_task.test", "enabled", "false"),
				),
			},
			{
				ResourceName:      "truenas_replication_task.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccReplicationConfig(enabled bool) string {
	return fmt.Sprintf(`
resource "truenas_replication_task" "test" {
  name              = "tf-acc-replication"
  direction         = "PUSH"
  transport         = "LOCAL"
  source_datasets   = ["tank/tf-acc-source"]
  target_dataset    = "tank/tf-acc-target"
  recursive         = false
  auto              = false
  retention_policy  = "SOURCE"
  enabled           = %v
}
`, enabled)
}

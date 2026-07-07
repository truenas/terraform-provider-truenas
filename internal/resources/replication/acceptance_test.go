package replication_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccReplication_basic tests create, update, and import of a LOCAL push
// replication task. Source and target datasets are own-created fixtures; the
// task provides also_include_naming_schema because the API requires a
// periodic-task binding, a naming schema, or a name regex for push tasks.
func TestAccReplication_basic(t *testing.T) {
	name := acctest.RandName("tf-acc-repl")
	srcDS := acctest.RandName("tf-acc-repl-src")
	dstDS := acctest.RandName("tf-acc-repl-dst")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckReplicationDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccReplicationConfig(name, srcDS, dstDS, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_replication_task.test", "name", name),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "direction", "PUSH"),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "transport", "LOCAL"),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "enabled", "true"),
					resource.TestCheckResourceAttrSet("truenas_replication_task.test", "id"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccReplicationConfig(name, srcDS, dstDS, false),
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

func testAccReplicationConfig(name, srcDS, dstDS string, enabled bool) string {
	pool := acctest.TestPool()
	return fmt.Sprintf(`
resource "truenas_dataset" "src" {
  name = "%[1]s/%[2]s"
}

resource "truenas_dataset" "dst" {
  name = "%[1]s/%[3]s"
}

resource "truenas_replication_task" "test" {
  name             = %[4]q
  direction        = "PUSH"
  transport        = "LOCAL"
  source_datasets  = [truenas_dataset.src.name]
  target_dataset   = truenas_dataset.dst.name
  recursive        = false
  auto             = false
  retention_policy = "SOURCE"
  enabled          = %[5]v

  also_include_naming_schema = ["auto-%%Y-%%m-%%d_%%H-%%M"]
}
`, pool, srcDS, dstDS, name, enabled)
}

// testAccCheckReplicationDestroyed verifies the task is gone from the API.
func testAccCheckReplicationDestroyed(name string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		raw, err := acctest.Client().Call(context.Background(), "replication.query",
			[][]any{{"name", "=", name}})
		if err != nil {
			return fmt.Errorf("querying replication tasks: %w", err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("decoding replication.query: %w", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("replication task %q still exists (id %d)", name, results[0].ID)
		}
		return nil
	}
}

package dataset_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

func TestAccDataset_basic(t *testing.T) {
	name := "tank/tf-acc-dataset-basic"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDatasetDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: testAccDatasetConfig(name, "lz4"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.test", "name", name),
					resource.TestCheckResourceAttr("truenas_dataset.test", "compression", "lz4"),
					resource.TestCheckResourceAttrSet("truenas_dataset.test", "mountpoint"),
					resource.TestCheckResourceAttrSet("truenas_dataset.test", "pool"),
				),
			},
			// Update compression
			{
				Config: testAccDatasetConfig(name, "zstd"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.test", "compression", "zstd"),
				),
			},
			// Import
			{
				ResourceName:      "truenas_dataset.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccDatasetConfig(name, compression string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name        = %q
  compression = %q
}
`, name, compression)
}

func testAccCheckDatasetDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		_, err := c.Call(context.Background(), "pool.dataset.get_instance", name)
		if err != nil && client.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("dataset %s still exists: %v", name, err)
	}
}

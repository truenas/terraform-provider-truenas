package zvol_test

import (
	"context"
	"fmt"
	"testing"

	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

func TestAccZvol_basic(t *testing.T) {
	name := "tank/tf-test-zvol"
	tfresource.Test(t, tfresource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckZvolDestroyed(name),
		Steps: []tfresource.TestStep{
			{
				Config: testAccZvolConfig(name, 1073741824, "lz4"),
				Check: tfresource.ComposeTestCheckFunc(
					tfresource.TestCheckResourceAttr("truenas_zvol.test", "name", name),
					tfresource.TestCheckResourceAttr("truenas_zvol.test", "volsize", "1073741824"),
					tfresource.TestCheckResourceAttr("truenas_zvol.test", "compression", "lz4"),
					tfresource.TestCheckResourceAttrSet("truenas_zvol.test", "pool"),
				),
			},
			{
				Config: testAccZvolConfig(name, 1073741824, "zstd"),
				Check: tfresource.ComposeTestCheckFunc(
					tfresource.TestCheckResourceAttr("truenas_zvol.test", "compression", "zstd"),
				),
			},
			{
				ResourceName:            "truenas_zvol.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"sparse"},
			},
		},
	})
}

func testAccZvolConfig(name string, volsize int64, compression string) string {
	return fmt.Sprintf(`
resource "truenas_zvol" "test" {
  name        = %q
  volsize     = %d
  compression = %q
}
`, name, volsize, compression)
}

func testAccCheckZvolDestroyed(name string) tfresource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		_, err := c.Call(context.Background(), "pool.dataset.get_instance", name)
		if err != nil {
			if client.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("error checking zvol %s: %v", name, err)
		}
		return fmt.Errorf("zvol %s still exists", name)
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package webshare_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccWebshare_dataSourceNameCase looks a share up by its name in upper
// case. The server keeps webshare names unique regardless of case, and the
// lookup used an exact "=" filter, so it reported "not found" (B-154).
func TestAccWebshare_dataSourceNameCase(t *testing.T) {
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("truenas_webshare requires TrueNAS 26.0 or later")
	}
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tffix3shtasks"))
	name := acctest.RandName("tffix3shtasks")
	cfg := acctest.ProviderConfig() + testAccWebshareConfig(ds, name, true) + fmt.Sprintf(`
data "truenas_webshare" "upper" {
  name       = %q
  depends_on = [truenas_webshare.test]
}
`, strings.ToUpper(name))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: cfg,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttrPair("data.truenas_webshare.upper", "id", "truenas_webshare.test", "id"),
				resource.TestCheckResourceAttr("data.truenas_webshare.upper", "name", strings.ToUpper(name)),
			),
		}},
	})
}

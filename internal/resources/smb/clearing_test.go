// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccSMBShare_setThenUnset sets flat legacy and top-level attributes, then
// removes them from the configuration. They used to keep their old values
// with an empty plan (B-113); now they reset to the server defaults.
func TestAccSMBShare_setThenUnset(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tffix3shtasks"))
	name := acctest.RandName("tffix3sh")
	c1 := smbShareConfig(ds, name, `
  comment       = "tffix3"
  hostsallow    = ["10.0.0.0/8"]
  hostsdeny     = ["192.168.99.0/24"]
  recyclebin    = true
  guestok       = true
  streams       = false
  durablehandle = false
  ro            = true
  abe           = true
  browsable     = false
`)
	c2 := smbShareConfig(ds, name, "")
	n := "truenas_smb_share.test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c1, Check: resource.TestCheckResourceAttr(n, "options.hostsallow.#", "1")},
			{
				Config: c2,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(n, "hostsallow.#", "0"),
					resource.TestCheckResourceAttr(n, "hostsdeny.#", "0"),
					resource.TestCheckResourceAttr(n, "options.hostsallow.#", "0"),
					resource.TestCheckResourceAttr(n, "recyclebin", "false"),
					resource.TestCheckResourceAttr(n, "guestok", "false"),
					resource.TestCheckResourceAttr(n, "streams", "true"),
					resource.TestCheckResourceAttr(n, "durablehandle", "true"),
					resource.TestCheckResourceAttr(n, "ro", "false"),
					resource.TestCheckResourceAttr(n, "abe", "false"),
					resource.TestCheckResourceAttr(n, "browsable", "true"),
					resource.TestCheckResourceAttr(n, "comment", ""),
				),
			},
			{Config: c2, ConfigPlanChecks: expectEmptyPlan()},
		},
	})
}

// TestAccSMBShare_dataSourceNameCase looks a share up by its name in upper
// case; SMB share names are case-insensitive, and the exact "=" filter
// reported "not found" (B-154).
func TestAccSMBShare_dataSourceNameCase(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tffix3shtasks"))
	name := acctest.RandName("tffix3sh")
	cfg := smbShareConfig(ds, name, "") + fmt.Sprintf(`
data "truenas_smb_share" "upper" {
  name       = %q
  depends_on = [truenas_smb_share.test]
}
`, strings.ToUpper(name))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: cfg,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttrPair("data.truenas_smb_share.upper", "id", "truenas_smb_share.test", "id"),
				resource.TestCheckResourceAttr("data.truenas_smb_share.upper", "name", strings.ToUpper(name)),
			),
		}},
	})
}

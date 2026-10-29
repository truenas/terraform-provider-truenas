// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nfs_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func nfsShareConfig(ds, body string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name = %q
}

resource "truenas_nfs_share" "test" {
  path = truenas_dataset.fixture.mountpoint
%s
}
`, ds, body)
}

func expectEmptyPlan() resource.ConfigPlanChecks {
	return resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
}

// TestAccNFSShare_bareIPNetwork: the server stores a bare address as /32
// (/128), so a bare IP failed with an inconsistent result (B-075).
func TestAccNFSShare_bareIPNetwork(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tffix3shtasks"))
	cfg := nfsShareConfig(ds, `  networks = ["10.1.2.3", "2001:db8::1"]`)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nfs_share.test", "networks.0", "10.1.2.3"),
					resource.TestCheckResourceAttr("truenas_nfs_share.test", "networks.1", "2001:db8::1"),
				),
			},
			{Config: cfg, ConfigPlanChecks: expectEmptyPlan()},
		},
	})
}

// TestAccNFSShare_importNonCanonicalNetwork: import stores the server form
// (10.77.0.0/16); a configured 10.77.3.4/16 planned an update (B-124).
func TestAccNFSShare_importNonCanonicalNetwork(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tffix3shtasks"))
	cfg := nfsShareConfig(ds, `
  networks = ["10.77.3.4/16", "10.66.0.0/16", "10.9.9.9"]
  enabled  = false
`)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ResourceName: "truenas_nfs_share.test", ImportState: true, ImportStateKind: resource.ImportBlockWithID},
		},
	})
}

// TestAccNFSShare_setThenUnset: removing optional attributes from the
// configuration left the old values on the share (B-114); they now reset to
// the server defaults.
func TestAccNFSShare_setThenUnset(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tffix3shtasks"))
	set := nfsShareConfig(ds, `
  comment       = "tffix3"
  networks      = ["10.0.0.0/8"]
  hosts         = ["192.0.2.5"]
  ro            = true
  maproot_user  = "root"
  maproot_group = "wheel"
  security      = ["SYS"]
`)
	unset := nfsShareConfig(ds, "")
	n := "truenas_nfs_share.test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: set, Check: resource.TestCheckResourceAttr(n, "maproot_user", "root")},
			{
				Config: unset,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(n, "networks.#", "0"),
					resource.TestCheckResourceAttr(n, "hosts.#", "0"),
					resource.TestCheckResourceAttr(n, "security.#", "0"),
					resource.TestCheckResourceAttr(n, "comment", ""),
					resource.TestCheckResourceAttr(n, "ro", "false"),
					resource.TestCheckResourceAttr(n, "maproot_user", ""),
					resource.TestCheckResourceAttr(n, "maproot_group", ""),
				),
			},
			{Config: unset, ConfigPlanChecks: expectEmptyPlan()},
			// The mapall pair on its own, then removed.
			{Config: nfsShareConfig(ds, `
  mapall_user  = "nobody"
  mapall_group = "nogroup"
`)},
			{
				Config: unset,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(n, "mapall_user", ""),
					resource.TestCheckResourceAttr(n, "mapall_group", ""),
				),
			},
		},
	})
}

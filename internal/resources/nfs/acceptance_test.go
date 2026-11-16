// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nfs_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNFSShare_basic creates a dataset fixture, shares its mountpoint over
// NFS, checks attributes, updates a mutable field (comment), imports the
// share by its integer ID, and verifies destruction.
func TestAccNFSShare_basic(t *testing.T) {
	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-nfs-ds"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNFSShareDestroyed(datasetName),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccNFSShareConfig(datasetName, "initial comment", []string{"127.0.0.1/32"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_nfs_share.test", "id"),
					resource.TestCheckResourceAttrPair("truenas_nfs_share.test", "path", "truenas_dataset.fixture", "mountpoint"),
					resource.TestCheckResourceAttr("truenas_nfs_share.test", "comment", "initial comment"),
					resource.TestCheckResourceAttr("truenas_nfs_share.test", "enabled", "true"),
					resource.TestCheckResourceAttr("truenas_nfs_share.test", "networks.#", "1"),
					resource.TestCheckResourceAttr("truenas_nfs_share.test", "networks.0", "127.0.0.1/32"),
				),
			},
			// Update comment and the networks list in place.
			{
				// The second network carries host bits (192.168.100.10/24);
				// TrueNAS stores it as its network address. The plan modifier
				// normalizes it so the apply is consistent (regression for #13).
				Config: acctest.ProviderConfig() + testAccNFSShareConfig(datasetName, "updated comment", []string{"127.0.0.1/32", "192.168.100.10/24"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nfs_share.test", "comment", "updated comment"),
					resource.TestCheckResourceAttr("truenas_nfs_share.test", "networks.#", "2"),
					// Semantic equality: the apply is consistent and state keeps the
					// user's form (192.168.100.10/24), equal to the stored .0/24.
					resource.TestCheckResourceAttr("truenas_nfs_share.test", "networks.1", "192.168.100.10/24"),
				),
			},
			// Import by integer ID (the resource's "id" attribute is the
			// share's numeric sharing.nfs ID, stringified).
			{
				ResourceName:      "truenas_nfs_share.test",
				ImportState:       true,
				ImportStateVerify: true,
				// networks is compared semantically at plan time, but
				// ImportStateVerify does a raw string compare: import returns the
				// stored network address (192.168.100.0/24) while state kept the
				// user's 192.168.100.10/24. They are equal; skip the raw check.
				ImportStateVerifyIgnore: []string{"networks"},
			},
		},
	})
}

// TestAccNFSShare_mapall exercises the mapall_user / mapall_group attributes:
// creates a share mapping all clients to root:root, verifies the values round
// trip, and imports.
func TestAccNFSShare_mapall(t *testing.T) {
	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-nfs-mapall"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNFSShareDestroyed(datasetName),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccNFSShareMapallConfig(datasetName, "root", "root", `["SYS"]`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nfs_share.test", "mapall_user", "root"),
					resource.TestCheckResourceAttr("truenas_nfs_share.test", "mapall_group", "root"),
					resource.TestCheckResourceAttr("truenas_nfs_share.test", "security.#", "1"),
					resource.TestCheckResourceAttr("truenas_nfs_share.test", "security.0", "SYS"),
					resource.TestCheckResourceAttrSet("truenas_nfs_share.test", "expose_snapshots"),
				),
			},
			// In-place update: grow the security list.
			{
				Config: acctest.ProviderConfig() + testAccNFSShareMapallConfig(datasetName, "root", "root", `["SYS", "KRB5"]`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nfs_share.test", "security.#", "2"),
					resource.TestCheckResourceAttr("truenas_nfs_share.test", "security.1", "KRB5"),
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

func testAccNFSShareMapallConfig(datasetName, mapallUser, mapallGroup, securityHCL string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name = %q
}

resource "truenas_nfs_share" "test" {
  path         = truenas_dataset.fixture.mountpoint
  mapall_user  = %q
  mapall_group = %q
  security     = %s
}
`, datasetName, mapallUser, mapallGroup, securityHCL)
}

func testAccNFSShareConfig(datasetName, comment string, networks []string) string {
	networksHCL := ""
	for i, n := range networks {
		if i > 0 {
			networksHCL += ", "
		}
		networksHCL += fmt.Sprintf("%q", n)
	}
	return fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name = %q
}

resource "truenas_nfs_share" "test" {
  path     = truenas_dataset.fixture.mountpoint
  comment  = %q
  networks = [%s]

  enabled          = true
  ro               = true
  hosts            = ["192.168.1.100"]
  maproot_user     = "root"
  maproot_group    = "root"
  expose_snapshots = false
}
`, datasetName, comment, networksHCL)
}

// TestAccNFSShare_reorderNoDiff verifies the pool #66 bug class fix for the
// NFS share allowlists: networks and hosts are order-insensitive sets, so
// re-applying the same elements in a different order than they were created
// (and than sharing.nfs returns them) must produce an empty plan, not a
// perpetual update diff.
func TestAccNFSShare_reorderNoDiff(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-nfs-ord"))
	create := nfsShareConfig(ds, `
  networks = ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"]
  hosts    = ["192.0.2.1", "192.0.2.2", "192.0.2.3"]
`)
	// Same three networks and hosts, each reordered: must be a no-op plan.
	reordered := nfsShareConfig(ds, `
  networks = ["192.168.0.0/16", "10.0.0.0/8", "172.16.0.0/12"]
  hosts    = ["192.0.2.3", "192.0.2.1", "192.0.2.2"]
`)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNFSShareDestroyed(ds),
		Steps: []resource.TestStep{
			{
				Config: create,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nfs_share.test", "networks.#", "3"),
					resource.TestCheckResourceAttr("truenas_nfs_share.test", "hosts.#", "3"),
				),
			},
			{
				Config:             reordered,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// nfsShareSummary is the subset of sharing.nfs.query fields this test
// package needs directly (outside of the provider's own resource code).
type nfsShareSummary struct {
	ID   int64  `json:"id"`
	Path string `json:"path"`
}

// testAccCheckNFSShareDestroyed queries sharing.nfs by the fixture dataset's
// mountpoint path since the share's ID is not known outside of Terraform
// state at CheckDestroy time.
func testAccCheckNFSShareDestroyed(datasetName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		path := "/mnt/" + datasetName
		raw, err := c.Call(context.Background(), "sharing.nfs.query", [][]any{{"path", "=", path}})
		if err != nil {
			return fmt.Errorf("error checking NFS share for %s: %v", path, err)
		}
		var results []nfsShareSummary
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing sharing.nfs.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("NFS share for %s still exists", path)
		}
		return nil
	}
}

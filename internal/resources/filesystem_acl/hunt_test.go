// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_acl_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntFilesystemAcl_allFields is an adversarial value round-trip: it sets
// every writable attribute of truenas_filesystem_acl to a non-default value in
// one config and proves each round-trips by (1) applying, (2) re-planning the
// identical config and asserting an empty plan, and (3) importing by path with
// ImportStateVerify.
//
// The ACL is set on a truenas_dataset fixture created in the same config with
// acltype = nfsv4 + aclmode = PASSTHROUGH (the combination that lets an NFS4 ACL
// be set on the path); truenas_filesystem_acl references the dataset's
// mountpoint, so it depends on the fixture implicitly. A rich three-entry NFS4
// ACL (owner@ FULL_CONTROL, group@ MODIFY, everyone@ READ with mixed INHERIT/
// NOINHERIT flags) exercises the entries attribute, and uid/gid/recursive/
// traverse are all set.
//
// recursive and traverse are write-only (apply-time instructions never stored in
// state), so, as in the acceptance import step, they need no ImportStateVerify
// ignore — they are simply absent from state on both sides.
func TestHuntFilesystemAcl_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)
	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfhunt-facl-ds"))
	cfg := acctest.ProviderConfig() + huntFilesystemAclConfig(datasetName)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_filesystem_acl.hunt", "path", "/mnt/"+datasetName),
					resource.TestCheckResourceAttr("truenas_filesystem_acl.hunt", "id", "/mnt/"+datasetName),
					resource.TestCheckResourceAttr("truenas_filesystem_acl.hunt", "acltype", "NFS4"),
					resource.TestCheckResourceAttr("truenas_filesystem_acl.hunt", "uid", "0"),
					resource.TestCheckResourceAttr("truenas_filesystem_acl.hunt", "gid", "0"),
					testAccCheckACL("/mnt/"+datasetName, 3, "owner@", "FULL_CONTROL"),
				),
			},
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:      "truenas_filesystem_acl.hunt",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// huntFilesystemAclConfig creates an NFS4 dataset fixture and sets a rich NFS4
// ACL plus uid/gid/recursive/traverse on its mountpoint.
func huntFilesystemAclConfig(datasetName string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name    = %q
  acltype = "nfsv4"
  aclmode = "PASSTHROUGH"
}

resource "truenas_filesystem_acl" "hunt" {
  path = truenas_dataset.fixture.mountpoint
  entries = jsonencode([
    {
      tag   = "owner@"
      type  = "ALLOW"
      perms = { BASIC = "FULL_CONTROL" }
      flags = { BASIC = "INHERIT" }
    },
    {
      tag   = "group@"
      type  = "ALLOW"
      perms = { BASIC = "MODIFY" }
      flags = { BASIC = "INHERIT" }
    },
    {
      tag   = "everyone@"
      type  = "ALLOW"
      perms = { BASIC = "READ" }
      flags = { BASIC = "NOINHERIT" }
    },
  ])

  uid       = 0
  gid       = 0
  recursive = true
  traverse  = true
}
`, datasetName)
}

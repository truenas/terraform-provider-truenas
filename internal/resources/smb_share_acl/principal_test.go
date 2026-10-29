// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_share_acl_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func shareACLConfig(ds, share, entries string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name       = %q
  share_type = "SMB"
}
resource "truenas_smb_share" "fixture" {
  path = truenas_dataset.fixture.mountpoint
  name = %q
}
resource "truenas_smb_share_acl" "test" {
  share_name = truenas_smb_share.fixture.name
  share_acl  = [%s]
}
`, ds, share, entries)
}

func expectEmptyPlan() resource.ConfigPlanChecks {
	return resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
}

// TestAccSMBShareACL_lowercaseSID: the server stores SIDs upper-case, and a
// lower-case SID gave a permanent diff (B-074). The configured spelling is now
// kept, and an import (server spelling) plans no change.
func TestAccSMBShareACL_lowercaseSID(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tffix3shtasks"))
	share := acctest.RandName("tffix3sh")
	cfg := shareACLConfig(ds, share, `{ ae_perm = "READ", ae_type = "ALLOWED", ae_who_sid = "s-1-1-0" }`)
	name := "truenas_smb_share_acl.test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, Check: resource.TestCheckResourceAttr(name, "share_acl.0.ae_who_sid", "s-1-1-0")},
			{Config: cfg, ConfigPlanChecks: expectEmptyPlan()},
			{Config: cfg, ResourceName: name, ImportState: true, ImportStateKind: resource.ImportBlockWithID},
		},
	})
}

// TestAccSMBShareACL_whoStrReadOnly: sharing.smb.setacl cannot resolve
// ae_who_str (every apply failed with "'NoneType' object is not subscriptable",
// B-073), so it is read-only and rejected at plan time.
func TestAccSMBShareACL_whoStrReadOnly(t *testing.T) {
	cfg := acctest.ProviderConfig() + `
resource "truenas_smb_share_acl" "test" {
  share_name = "tffix3shtasks-none"
  share_acl  = [{ ae_perm = "READ", ae_type = "ALLOWED", ae_who_str = "everyone@" }]
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, PlanOnly: true, ExpectError: regexp.MustCompile(`ae_who_str`)},
		},
	})
}

// TestAccSMBShareACL_whoIDImport: an entry configured by Unix ID imports with
// an empty plan (B-123; Read used to keep only the SID). A SID entry in the
// same ACL stores the resolved name and an unmapped (null) ID. An in-place
// perm change keeps the resolved fields of unchanged principals.
func TestAccSMBShareACL_whoIDImport(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tffix3shtasks"))
	share := acctest.RandName("tffix3sh")
	entries := func(perm string) string {
		return fmt.Sprintf(`
    { ae_perm = %q, ae_type = "ALLOWED", ae_who_id = { id_type = "GROUP", id = 545 } },
    { ae_perm = "READ", ae_type = "ALLOWED", ae_who_sid = "S-1-1-0" },
`, perm)
	}
	c1 := shareACLConfig(ds, share, entries("FULL"))
	c2 := shareACLConfig(ds, share, entries("CHANGE"))
	name := "truenas_smb_share_acl.test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: c1,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "share_acl.0.ae_who_id.id", "545"),
					resource.TestCheckResourceAttr(name, "share_acl.0.ae_who_id.id_type", "GROUP"),
					resource.TestCheckResourceAttrSet(name, "share_acl.0.ae_who_sid"),
					resource.TestCheckResourceAttr(name, "share_acl.1.ae_who_sid", "S-1-1-0"),
					resource.TestCheckResourceAttr(name, "share_acl.1.ae_who_str", "everyone@"),
					resource.TestCheckNoResourceAttr(name, "share_acl.1.ae_who_id"),
				),
			},
			{Config: c1, ConfigPlanChecks: expectEmptyPlan()},
			{Config: c1, ResourceName: name, ImportState: true, ImportStateKind: resource.ImportBlockWithID},
			{Config: c1, ResourceName: name, ImportState: true, ImportStateVerify: true},
			{
				Config: c2,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectKnownValue(name, tfjsonpath.New("share_acl").AtSliceIndex(0).AtMapKey("ae_who_sid"), knownvalue.NotNull()),
				}},
				Check: resource.TestCheckResourceAttr(name, "share_acl.0.ae_perm", "CHANGE"),
			},
			{Config: c2, ConfigPlanChecks: expectEmptyPlan()},
		},
	})
}

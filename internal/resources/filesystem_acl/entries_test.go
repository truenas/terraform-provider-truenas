// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_acl_test

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

func aclDS() string {
	return fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfacc-st3-acl"))
}

func aclDatasetCfg(ds string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "fx" {
  name    = %q
  acltype = "nfsv4"
  aclmode = "PASSTHROUGH"
}
`, ds)
}

func aclCfg(ds, entries, extra string) string {
	return aclDatasetCfg(ds) + fmt.Sprintf(`
resource "truenas_filesystem_acl" "test" {
  path    = truenas_dataset.fx.mountpoint
  entries = jsonencode(%s)
%s}
`, entries, extra)
}

func emptyPlan() resource.ConfigPlanChecks {
	return resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
}

const partialBagEntries = `[
    { tag = "owner@",    type = "ALLOW", perms = { BASIC = "FULL_CONTROL" }, flags = { BASIC = "INHERIT" } },
    { tag = "group@",    type = "ALLOW", perms = { BASIC = "MODIFY" },       flags = { BASIC = "INHERIT" } },
    { tag = "everyone@", type = "ALLOW", perms = { READ_DATA = true, EXECUTE = true }, flags = { FILE_INHERIT = true } },
  ]`

// A partial bag of booleans (the server stores the missing members as false)
// converges: the plan after create is empty. (B-092) An entries update keeps
// acltype, uid and gid known in the plan instead of (known after apply).
// (B-146)
func TestAccFilesystemAcl_partialPermsBag(t *testing.T) {
	ds := aclDS()
	changed := regexp.MustCompile(`BASIC = "MODIFY"`).ReplaceAllString(partialBagEntries, `BASIC = "READ"`)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: aclCfg(ds, partialBagEntries, "")},
			{Config: aclCfg(ds, partialBagEntries, ""), ConfigPlanChecks: emptyPlan()},
			{
				Config: aclCfg(ds, changed, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_filesystem_acl.test", plancheck.ResourceActionUpdate),
					plancheck.ExpectKnownValue("truenas_filesystem_acl.test", tfjsonpath.New("acltype"), knownvalue.StringExact("NFS4")),
					plancheck.ExpectKnownValue("truenas_filesystem_acl.test", tfjsonpath.New("uid"), knownvalue.NotNull()),
					plancheck.ExpectKnownValue("truenas_filesystem_acl.test", tfjsonpath.New("gid"), knownvalue.NotNull()),
				}},
			},
			{Config: aclCfg(ds, changed, ""), ConfigPlanChecks: emptyPlan()},
		},
	})
}

// A USER entry given by "who" compares equal to the id the server stores.
// (B-093)
func TestAccFilesystemAcl_whoInsteadOfID(t *testing.T) {
	ds := aclDS()
	entries := `[
    { tag = "owner@", type = "ALLOW", perms = { BASIC = "FULL_CONTROL" }, flags = { BASIC = "INHERIT" } },
    { tag = "group@", type = "ALLOW", perms = { BASIC = "MODIFY" },       flags = { BASIC = "INHERIT" } },
    { tag = "USER", who = "root", type = "ALLOW", perms = { BASIC = "READ" }, flags = { BASIC = "INHERIT" } },
  ]`
	byID := `[
    { tag = "owner@", type = "ALLOW", perms = { BASIC = "FULL_CONTROL" }, flags = { BASIC = "INHERIT" } },
    { tag = "group@", type = "ALLOW", perms = { BASIC = "MODIFY" },       flags = { BASIC = "INHERIT" } },
    { tag = "USER", id = 0, type = "ALLOW", perms = { BASIC = "READ" }, flags = { BASIC = "INHERIT" } },
  ]`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: aclCfg(ds, entries, "")},
			{Config: aclCfg(ds, entries, ""), ConfigPlanChecks: emptyPlan()},
			// The same ACL written by id instead of name plans nothing.
			{Config: aclCfg(ds, byID, ""), ConfigPlanChecks: emptyPlan()},
		},
	})
}

// recursive/traverse are write-only and partial bags compare equal, so an
// import against the identical configuration plans no change: it used to plan
// an update that re-ran a recursive setacl over the tree. (B-115, B-092)
func TestAccFilesystemAcl_importNoop(t *testing.T) {
	ds := aclDS()
	cfg := aclCfg(ds, partialBagEntries, "  recursive = true\n  traverse  = true\n")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, Check: resource.TestCheckNoResourceAttr("truenas_filesystem_acl.test", "recursive")},
			{Config: aclDatasetCfg(ds) + `
removed {
  from = truenas_filesystem_acl.test
  lifecycle {
    destroy = false
  }
}
`},
			{
				Config: cfg + `
import {
  to = truenas_filesystem_acl.test
  id = truenas_dataset.fx.mountpoint
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_filesystem_acl.test", plancheck.ResourceActionNoop),
				}},
			},
			{Config: cfg, ConfigPlanChecks: emptyPlan()},
		},
	})
}

// uid/gid = -1 is rejected at plan time (it read back as the real id).
func TestAccFilesystemAcl_uidMinusOneRejected(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: aclCfg(aclDS(), partialBagEntries, "  uid = -1\n"), PlanOnly: true, ExpectError: regexp.MustCompile(`uid`)},
		},
	})
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acl_template_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func tmplCfg(name, acl string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_acl_template" "test" {
  name    = %q
  acltype = "NFS4"
  acl     = jsonencode(%s)
}
`, name, acl)
}

func emptyPlan() resource.ConfigPlanChecks {
	return resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
}

// A partial bag of booleans (the server stores the missing members as false)
// converges. (B-092)
func TestAccAclTemplate_partialPermsBag(t *testing.T) {
	cfg := tmplCfg(acctest.RandName("tfacc-st3-tmpl"), `[
    { tag = "owner@",    type = "ALLOW", perms = { BASIC = "FULL_CONTROL" }, flags = { BASIC = "INHERIT" } },
    { tag = "everyone@", type = "ALLOW", perms = { READ_DATA = true, EXECUTE = true }, flags = { FILE_INHERIT = true } },
  ]`)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ConfigPlanChecks: emptyPlan()},
		},
	})
}

// A USER entry given by "who" creates (the provider sends the uid; the server
// crashes with KeyError: 'who' otherwise) and converges. (B-149)
func TestAccAclTemplate_whoEntry(t *testing.T) {
	cfg := tmplCfg(acctest.RandName("tfacc-st3-tmplw"), `[
    { tag = "owner@", type = "ALLOW", perms = { BASIC = "FULL_CONTROL" }, flags = { BASIC = "INHERIT" } },
    { tag = "USER", who = "root", type = "ALLOW", perms = { BASIC = "READ" }, flags = { BASIC = "INHERIT" } },
  ]`)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ConfigPlanChecks: emptyPlan()},
		},
	})
}

// An import against a configuration that spells the entries differently from
// the server (id = -1 / who = null, a partial bag) plans no change. (B-121)
func TestAccAclTemplate_importNoop(t *testing.T) {
	name := acctest.RandName("tfacc-st3-tmpli")
	cfg := tmplCfg(name, `[
    { tag = "owner@",    id = -1, who = null, type = "ALLOW", perms = { BASIC = "FULL_CONTROL" }, flags = { BASIC = "INHERIT" } },
    { tag = "everyone@", id = -1, who = null, type = "ALLOW", perms = { READ_DATA = true }, flags = { FILE_INHERIT = true } },
  ]`)
	lookup := fmt.Sprintf(`
data "truenas_acl_template" "found" {
  name = %q
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: acctest.ProviderConfig() + `
removed {
  from = truenas_acl_template.test
  lifecycle {
    destroy = false
  }
}
`},
			{
				Config: cfg + lookup + `
import {
  to = truenas_acl_template.test
  id = tostring(data.truenas_acl_template.found.id)
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_acl_template.test", plancheck.ResourceActionNoop),
				}},
			},
			{Config: cfg + lookup, ConfigPlanChecks: emptyPlan()},
		},
	})
}

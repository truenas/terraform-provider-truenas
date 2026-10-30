// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acl_template_test

// Adversarial bug-hunt test: set every API-accepted acl_template attribute to
// a valid non-default value (name, acltype NFS4, a non-empty comment, and a
// multi-entry ACL), apply, assert an empty re-plan, then import. The ACL JSON
// is written in the exact compact, alphabetical-key form that the provider's
// canonicalization produces from the server read-back, so the value is
// byte-identical across create -> refresh -> import (see the acceptance
// test's own note on this). The POSIX1E acltype path is not covered here:
// its read-back normalization (id sentinels, boolean-bag collapsing) makes a
// byte-identical config harder to pin and is better left to its own fixture.

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestHuntAclTemplate_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)

	name := acctest.RandName("tfhuntacltmpl")

	acl := `[{"flags":{"BASIC":"INHERIT"},"perms":{"BASIC":"FULL_CONTROL"},"tag":"owner@","type":"ALLOW"},` +
		`{"flags":{"BASIC":"INHERIT"},"perms":{"BASIC":"MODIFY"},"tag":"group@","type":"ALLOW"},` +
		`{"flags":{"BASIC":"NOINHERIT"},"perms":{"BASIC":"TRAVERSE"},"tag":"everyone@","type":"ALLOW"}]`

	cfg := acctest.ProviderConfig() + testHuntAclTemplateAllConfig(name, "tf hunt comment", acl)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAclTemplateDestroyed(name),
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:      "truenas_acl_template.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testHuntAclTemplateAllConfig(name, comment, acl string) string {
	return fmt.Sprintf(`
resource "truenas_acl_template" "test" {
  name    = %q
  acltype = "NFS4"
  comment = %q
  acl     = %q
}
`, name, comment, acl)
}

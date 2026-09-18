// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acl_template_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccAclTemplate_basic creates a user-defined NFS4 ACL template (entry
// shape probed live via filesystem.acltemplate.create/get_instance),
// checks its attributes, updates the comment and the acl entries in place,
// imports it by its numeric id, and verifies destruction. It never touches
// any of TrueNAS's 9 builtin templates (NFS4_OPEN, NFS4_RESTRICTED,
// NFS4_HOME, NFS4_DOMAIN_HOME, POSIX_OPEN, POSIX_RESTRICTED, POSIX_HOME,
// NFS4_ADMIN, POSIX_ADMIN — probed live via filesystem.acltemplate.query);
// CheckDestroy is scoped to this test's own generated name only.
func TestAccAclTemplate_basic(t *testing.T) {
	name := acctest.RandName("tf-acc-acltemplate")

	// These are written in the exact compact, alphabetical-key form that
	// canonicalACLJSON/aclEntriesNormalized produce from the server's
	// get_instance response (id/who dropped since they're null/-1 for
	// these special tags; json.Marshal of a Go map sorts keys) — this
	// keeps the value byte-identical across create -> refresh -> import,
	// which terraform-plugin-testing's post-apply/ImportStateVerify checks
	// require for a Required (non-Computed) attribute.
	initialACL := `[{"flags":{"BASIC":"INHERIT"},"perms":{"BASIC":"FULL_CONTROL"},"tag":"owner@","type":"ALLOW"},` +
		`{"flags":{"BASIC":"INHERIT"},"perms":{"BASIC":"MODIFY"},"tag":"group@","type":"ALLOW"},` +
		`{"flags":{"BASIC":"INHERIT"},"perms":{"BASIC":"READ"},"tag":"everyone@","type":"ALLOW"}]`
	updatedACL := `[{"flags":{"BASIC":"INHERIT"},"perms":{"BASIC":"FULL_CONTROL"},"tag":"owner@","type":"ALLOW"},` +
		`{"flags":{"BASIC":"INHERIT"},"perms":{"BASIC":"FULL_CONTROL"},"tag":"group@","type":"ALLOW"}]`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAclTemplateDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccAclTemplateConfig(name, "initial comment", initialACL),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_acl_template.test", "name", name),
					resource.TestCheckResourceAttr("truenas_acl_template.test", "acltype", "NFS4"),
					resource.TestCheckResourceAttr("truenas_acl_template.test", "comment", "initial comment"),
					resource.TestCheckResourceAttr("truenas_acl_template.test", "builtin", "false"),
					resource.TestCheckResourceAttrSet("truenas_acl_template.test", "id"),
					testAccCheckAclTemplateACLLen(name, 3),
				),
			},
			// Update in place: comment and acl entries.
			{
				Config: acctest.ProviderConfig() + testAccAclTemplateConfig(name, "updated comment", updatedACL),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_acl_template.test", "comment", "updated comment"),
					testAccCheckAclTemplateACLLen(name, 2),
				),
			},
			// Import by the template's numeric id.
			{
				ResourceName:      "truenas_acl_template.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccAclTemplateConfig(name, comment, acl string) string {
	return fmt.Sprintf(`
resource "truenas_acl_template" "test" {
  name    = %q
  acltype = "NFS4"
  comment = %q
  acl     = %q
}
`, name, comment, acl)
}

// aclTemplateSummary is the subset of filesystem.acltemplate.query fields
// this test package needs directly.
type aclTemplateSummary struct {
	ID  int64           `json:"id"`
	ACL json.RawMessage `json:"acl"`
}

// testAccCheckAclTemplateACLLen verifies (server-side, via
// filesystem.acltemplate.query) that the template's acl array has the
// expected number of entries.
func testAccCheckAclTemplateACLLen(name string, wantLen int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "filesystem.acltemplate.query", [][]any{{"name", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking acl template %s: %v", name, err)
		}
		var results []aclTemplateSummary
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing filesystem.acltemplate.query response: %v", err)
		}
		if len(results) != 1 {
			return fmt.Errorf("expected exactly 1 acl template named %s, got %d", name, len(results))
		}
		var entries []map[string]any
		if err := json.Unmarshal(results[0].ACL, &entries); err != nil {
			return fmt.Errorf("error parsing acl entries: %v", err)
		}
		if len(entries) != wantLen {
			return fmt.Errorf("acl template %s has %d acl entries, want %d", name, len(entries), wantLen)
		}
		return nil
	}
}

// testAccCheckAclTemplateDestroyed verifies the template no longer exists
// by its generated name. Scoped to this test's own name only — TrueNAS's
// 9 builtin templates are never touched by this resource and are not
// checked here.
func testAccCheckAclTemplateDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "filesystem.acltemplate.query", [][]any{{"name", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking acl template %s: %v", name, err)
		}
		var results []aclTemplateSummary
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing filesystem.acltemplate.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("acl template %s still exists", name)
		}
		return nil
	}
}

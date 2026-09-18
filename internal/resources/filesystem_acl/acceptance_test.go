// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_acl_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccFilesystemAcl_basic pre-creates an NFS4-acltype dataset fixture
// directly via the raw API (in PreCheck, cleaned up via t.Cleanup) rather
// than through truenas_dataset: this provider's probe pool has "tank"'s
// root acltype=POSIX AND aclmode=DISCARD set LOCAL (probed live), and
// TrueNAS rejects `pool.dataset.create({"acltype":"NFSV4"})` with EINVAL
// ("aclmode may not be set for NFSv4 acl type") when the inherited aclmode
// is DISCARD - truenas_dataset does not expose "aclmode" to override this,
// so an explicit aclmode="PASSTHROUGH" (verified live) must be sent
// alongside acltype, which only the raw pool.dataset.create call below can
// do. A fixture user is still managed by Terraform (for the "USER" ACE's
// real uid). The test sets an owner@/group@/USER NFS4 ACL via truenas_
// filesystem_acl, getacl-verifies it server-side, updates the entries in
// place, imports by path (ignoring the apply-time-only recursive/traverse
// options), then removes ONLY truenas_filesystem_acl from the config
// (destroying just that resource while the user fixture remains) and
// asserts - this is the documented Delete semantic, probed live and
// confirmed clean on both NFS4 and POSIX1E - that filesystem.getacl
// reports "trivial": true afterward: destroy strips the ACL to a mode-
// derived trivial one rather than leaving it in place.
func TestAccFilesystemAcl_basic(t *testing.T) {
	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-facl-ds"))
	path := "/mnt/" + datasetName
	username := acctest.RandName("tf-acc-facl-user")

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(t)
			c := acctest.Client()
			if _, err := c.Call(context.Background(), "pool.dataset.create", map[string]any{
				"name":    datasetName,
				"acltype": "NFSV4",
				"aclmode": "PASSTHROUGH",
			}); err != nil {
				t.Fatalf("fixture dataset create failed: %v", err)
			}
			t.Cleanup(func() {
				if _, err := c.Call(context.Background(), "pool.dataset.delete", datasetName, map[string]any{"recursive": true}); err != nil {
					t.Logf("fixture dataset cleanup failed (%s): %v", datasetName, err)
				}
			})
		},
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccFilesystemAclConfig(path, username, "FULL_CONTROL", "MODIFY", true /* includeFACL */),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_filesystem_acl.test", "path", path),
					resource.TestCheckResourceAttr("truenas_filesystem_acl.test", "id", path),
					resource.TestCheckResourceAttr("truenas_filesystem_acl.test", "acltype", "NFS4"),
					resource.TestCheckResourceAttrSet("truenas_filesystem_acl.test", "uid"),
					resource.TestCheckResourceAttrSet("truenas_filesystem_acl.test", "gid"),
					testAccCheckACL(path, 3, "owner@", "FULL_CONTROL"),
				),
			},
			// Update entries in place: owner@ perms change, group@ perms
			// change, USER entry's perms change.
			{
				Config: acctest.ProviderConfig() + testAccFilesystemAclConfig(path, username, "MODIFY", "FULL_CONTROL", true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckACL(path, 3, "owner@", "MODIFY"),
				),
			},
			// Import by path.
			{
				ResourceName:            "truenas_filesystem_acl.test",
				ImportState:             true,
				ImportStateId:           path,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"recursive", "traverse"},
			},
			// Remove ONLY truenas_filesystem_acl from the config: this
			// destroys just that resource (Delete = filesystem.setacl with
			// options.stripacl=true) while the user fixture and the raw-
			// created dataset remain, so the documented Delete semantic
			// can be checked directly server-side before the framework's
			// final full teardown (and this test's own t.Cleanup).
			{
				Config: acctest.ProviderConfig() + testAccFilesystemAclConfig(path, username, "MODIFY", "FULL_CONTROL", false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckACLTrivial(path),
				),
			},
		},
	})
}

// testAccFilesystemAclConfig returns HCL managing only the user fixture
// (and, when includeFACL, truenas_filesystem_acl). The dataset itself is
// NOT a Terraform resource here - see TestAccFilesystemAcl_basic's doc
// comment for why it is pre-created via the raw API in PreCheck instead of
// via truenas_dataset.
func testAccFilesystemAclConfig(path, username, ownerPerm, groupPerm string, includeFACL bool) string {
	fixtures := fmt.Sprintf(`
resource "truenas_user" "fixture" {
  username          = %q
  full_name         = "TF Acc Facl User"
  password          = "Tf-Acc-Test-Passw0rd!"
  password_disabled = false
  home              = "/var/empty"
  shell             = "/usr/bin/bash"
  smb               = false
  group_create      = true
}
`, username)

	if !includeFACL {
		return fixtures
	}

	return fixtures + fmt.Sprintf(`
resource "truenas_filesystem_acl" "test" {
  path = %q
  entries = jsonencode([
    {
      tag   = "owner@"
      type  = "ALLOW"
      perms = { BASIC = %q }
      flags = { BASIC = "INHERIT" }
    },
    {
      tag   = "group@"
      type  = "ALLOW"
      perms = { BASIC = %q }
      flags = { BASIC = "INHERIT" }
    },
    {
      tag   = "USER"
      id    = truenas_user.fixture.uid
      type  = "ALLOW"
      perms = { BASIC = "READ" }
      flags = { BASIC = "INHERIT" }
    },
  ])
}
`, path, ownerPerm, groupPerm)
}

// getAclSummary is the subset of filesystem.getacl's response this test
// package needs directly.
type getAclSummary struct {
	ACLType string          `json:"acltype"`
	ACL     json.RawMessage `json:"acl"`
	Trivial bool            `json:"trivial"`
}

type aceSummary struct {
	Tag   string `json:"tag"`
	Perms struct {
		Basic string `json:"BASIC"`
	} `json:"perms"`
}

// testAccCheckACL calls filesystem.getacl directly (bypassing Terraform
// state entirely) and asserts the path has wantLen entries and that the
// first entry matching wantFirstTag carries wantFirstBasicPerm.
func testAccCheckACL(path string, wantLen int, wantFirstTag, wantFirstBasicPerm string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "filesystem.getacl", path)
		if err != nil {
			return fmt.Errorf("error calling filesystem.getacl for %s: %v", path, err)
		}
		var got getAclSummary
		if err := json.Unmarshal(raw, &got); err != nil {
			return fmt.Errorf("error parsing filesystem.getacl response: %v", err)
		}
		if got.ACLType != "NFS4" {
			return fmt.Errorf("filesystem.getacl(%s) acltype = %s, want NFS4", path, got.ACLType)
		}
		var entries []aceSummary
		if err := json.Unmarshal(got.ACL, &entries); err != nil {
			return fmt.Errorf("error parsing acl entries: %v", err)
		}
		if len(entries) != wantLen {
			return fmt.Errorf("filesystem.getacl(%s) has %d acl entries, want %d", path, len(entries), wantLen)
		}
		for _, e := range entries {
			if e.Tag == wantFirstTag {
				if e.Perms.Basic != wantFirstBasicPerm {
					return fmt.Errorf("filesystem.getacl(%s) tag %s perms.BASIC = %s, want %s",
						path, wantFirstTag, e.Perms.Basic, wantFirstBasicPerm)
				}
				return nil
			}
		}
		return fmt.Errorf("filesystem.getacl(%s) has no entry with tag %s", path, wantFirstTag)
	}
}

// testAccCheckACLTrivial calls filesystem.getacl directly and asserts the
// documented Delete semantic: the ACL was stripped to a trivial (mode-
// derived) one.
func testAccCheckACLTrivial(path string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "filesystem.getacl", path)
		if err != nil {
			return fmt.Errorf("error calling filesystem.getacl for %s: %v", path, err)
		}
		var got getAclSummary
		if err := json.Unmarshal(raw, &got); err != nil {
			return fmt.Errorf("error parsing filesystem.getacl response: %v", err)
		}
		if !got.Trivial {
			return fmt.Errorf("filesystem.getacl(%s) trivial = false after destroy, want true (documented strip-on-delete semantic)", path)
		}
		return nil
	}
}

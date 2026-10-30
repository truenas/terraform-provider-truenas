// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_share_acl_test

// Adversarial bug-hunt tests (throwaway, not for commit).

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// Rename the share the ACL is attached to (share_name is RequiresReplace and
// references the share's name).
func TestHuntSMBACL_shareRename(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfhunt"))
	e := `{ ae_perm = "READ", ae_type = "ALLOWED", ae_who_sid = "S-1-1-0" }`
	c1 := shareACLConfig(ds, acctest.RandName("tfhunt"), e)
	c2 := shareACLConfig(ds, acctest.RandName("tfhunt"), e)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c1},
			{Config: c2},
			{Config: c2, ConfigPlanChecks: expectEmptyPlan()},
		},
	})
}

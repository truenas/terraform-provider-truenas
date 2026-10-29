// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// smbShareConfig returns a dataset fixture plus a truenas_smb_share on its
// mountpoint, with body spliced into the share block.
func smbShareConfig(ds, name, body string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name       = %q
  share_type = "SMB"
}

resource "truenas_smb_share" "test" {
  path = truenas_dataset.fixture.mountpoint
  name = %q
%s
}
`, ds, name, body)
}

func expectEmptyPlan() resource.ConfigPlanChecks {
	return resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
}

// TestAccSMBShare_flatAttrUpdate changes flat legacy attributes on an existing
// share while `options` is not configured. options is pinned from state and
// the payload must carry the whole options object, so the changed flat values
// have to be copied onto it; before the fix the stale pinned values were sent
// and the apply failed with an inconsistent result (B-011).
func TestAccSMBShare_flatAttrUpdate(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tffix"))
	name := acctest.RandName("tffix")
	c1 := smbShareConfig(ds, name, `
  guestok    = false
  streams    = true
  hostsallow = ["10.0.0.0/8"]
`)
	c2 := smbShareConfig(ds, name, `
  guestok    = true
  streams    = false
  hostsallow = ["192.168.0.0/16"]
`)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c1},
			{Config: c1, ConfigPlanChecks: expectEmptyPlan()},
			{
				Config: c2,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_smb_share.test", "guestok", "true"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "streams", "false"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "hostsallow.#", "1"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "hostsallow.0", "192.168.0.0/16"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "options.guestok", "true"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "options.hostsallow.0", "192.168.0.0/16"),
					resource.TestCheckResourceAttrSet("truenas_smb_share.test", "vuid"),
				),
			},
			{Config: c2, ConfigPlanChecks: expectEmptyPlan()},
		},
	})
}

// TestAccSMBShare_purposeChange switches an existing share LEGACY_SHARE ->
// DEFAULT_SHARE -> LEGACY_SHARE with options not configured. options and vuid
// are derived from purpose; before the fix both were pinned from the old
// purpose and the apply failed with inconsistent results (B-066). The switch
// back to LEGACY_SHARE also carries a flat attribute.
func TestAccSMBShare_purposeChange(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tffix"))
	name := acctest.RandName("tffix")
	c1 := smbShareConfig(ds, name, `  purpose = "LEGACY_SHARE"`)
	c2 := smbShareConfig(ds, name, `  purpose = "DEFAULT_SHARE"`)
	c3 := smbShareConfig(ds, name, `
  purpose = "LEGACY_SHARE"
  guestok = true
`)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c1},
			{
				Config: c2,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_smb_share.test", "purpose", "DEFAULT_SHARE"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "vuid", ""),
					resource.TestCheckNoResourceAttr("truenas_smb_share.test", "options.vuid"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "options.aapl_name_mangling", "false"),
				),
			},
			{Config: c2, ConfigPlanChecks: expectEmptyPlan()},
			{
				Config: c3,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_smb_share.test", "purpose", "LEGACY_SHARE"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "guestok", "true"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "options.guestok", "true"),
					resource.TestCheckResourceAttrSet("truenas_smb_share.test", "vuid"),
				),
			},
			{Config: c3, ConfigPlanChecks: expectEmptyPlan()},
		},
	})
}

// TestAccSMBShare_flatOnNonLegacyPurpose checks that a flat legacy attribute
// with a non-LEGACY purpose is a plan-time error. apiPayload only sends the
// flat attributes for LEGACY_SHARE, so before the fix the value was silently
// dropped and the apply failed with an inconsistent result (B-067).
func TestAccSMBShare_flatOnNonLegacyPurpose(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tffix"))
	name := acctest.RandName("tffix")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: smbShareConfig(ds, name, `
  purpose = "DEFAULT_SHARE"
  guestok = true
`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Attribute only valid for\s+LEGACY_SHARE`),
			},
			{
				Config: smbShareConfig(ds, name, `
  purpose    = "DEFAULT_SHARE"
  hostsallow = ["10.0.0.0/8"]
`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Set\s+options\.hostsallow\s+instead`),
			},
		},
	})
}

// TestAccSMBShare_flatAndOptionsConflict checks that setting a flat legacy
// attribute and its options twin is a plan-time error: options would win and
// the flat attribute would read back a different value.
func TestAccSMBShare_flatAndOptionsConflict(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tffix"))
	name := acctest.RandName("tffix")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: smbShareConfig(ds, name, `
  guestok = true
  options = { guestok = false }
`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Attribute set twice`),
			},
		},
	})
}

// TestAccSMBShare_flatWithPartialOptions changes a flat legacy attribute on a
// LEGACY_SHARE whose options block is configured but does not set that field.
// The unset options field is planned unknown, and the flat value must be sent.
func TestAccSMBShare_flatWithPartialOptions(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tffix"))
	name := acctest.RandName("tffix")
	c1 := smbShareConfig(ds, name, `
  guestok = false
  options = { recyclebin = true }
`)
	c2 := smbShareConfig(ds, name, `
  guestok = true
  options = { recyclebin = true }
`)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c1},
			{
				Config: c2,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_smb_share.test", "guestok", "true"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "options.guestok", "true"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "options.recyclebin", "true"),
				),
			},
			{Config: c2, ConfigPlanChecks: expectEmptyPlan()},
		},
	})
}

// TestAccSMBShare_unsetDefaults creates a share without enabled, browsable,
// ro, abe or comment. They must take the server defaults (enabled and
// browsable true); before the fix their Go zero values were sent and the
// share was created disabled and hidden. A later update that leaves them
// unset must keep them.
func TestAccSMBShare_unsetDefaults(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tffix"))
	name := acctest.RandName("tffix")
	c1 := smbShareConfig(ds, name, "")
	c2 := smbShareConfig(ds, name, `  guestok = true`)
	check := resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("truenas_smb_share.test", "enabled", "true"),
		resource.TestCheckResourceAttr("truenas_smb_share.test", "browsable", "true"),
		resource.TestCheckResourceAttr("truenas_smb_share.test", "ro", "false"),
		resource.TestCheckResourceAttr("truenas_smb_share.test", "abe", "false"),
		resource.TestCheckResourceAttr("truenas_smb_share.test", "comment", ""),
	)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c1, Check: check},
			{Config: c1, ConfigPlanChecks: expectEmptyPlan()},
			{Config: c2, Check: check},
			{Config: c2, ConfigPlanChecks: expectEmptyPlan()},
		},
	})
}

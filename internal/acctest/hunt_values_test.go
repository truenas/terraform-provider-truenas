// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

// Adversarial VALUES and lifecycle edges users hit:
//   - sizes the API rounds (unaligned zvol volsize) -> "inconsistent result"?
//   - zvol shrink -> clear error, or half-applied state?
//   - empty string vs the API's own default sentinel (dataset comments "")
//   - unicode / quotes in free-text fields
//   - a share name with a space
//   - object deleted behind Terraform's back -> plan must be a clean create

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func applyThenEmpty(t *testing.T, cfg string) {
	t.Helper()
	acctest.PreCheck(t)
	full := acctest.ProviderConfig() + cfg
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: full},
			{Config: full, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

// 33554433 bytes is not a multiple of any volblocksize; ZFS rounds it up.
func TestHuntValues_zvolUnalignedVolsize(t *testing.T) {
	applyThenEmpty(t, fmt.Sprintf(`
resource "truenas_zvol" "z" {
  name    = %q
  volsize = 33554433
  sparse  = true
}
`, acctest.TestPool()+"/"+acctest.RandName("tfhuntval")))
}

// Shrinking a zvol is refused by ZFS/the API. The provider must say so
// clearly at plan or apply — not apply half of the change.
func TestHuntValues_zvolShrink(t *testing.T) {
	acctest.PreCheck(t)
	n := acctest.TestPool() + "/" + acctest.RandName("tfhuntval")
	big := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "z" {
  name    = %q
  volsize = 67108864
  sparse  = true
}
`, n)
	small := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "z" {
  name    = %q
  volsize = 33554432
  sparse  = true
}
`, n)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: big},
			// whatever the provider does, it must be an explicit error that
			// mentions the size; accept either plan-time or apply-time.
			{Config: small, ExpectError: regexp.MustCompile(`(?i)volsize|shrink|smaller|size`)},
			// and the box must still be consistent with the big config
			{Config: big, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

// The API's default for comments is the sentinel "INHERIT"; an explicit ""
// must round-trip as "" (or be rejected), not flip to INHERIT and diff.
func TestHuntValues_datasetEmptyComment(t *testing.T) {
	applyThenEmpty(t, fmt.Sprintf(`
resource "truenas_dataset" "d" {
  name     = %q
  comments = ""
}
`, acctest.TestPool()+"/"+acctest.RandName("tfhuntval")))
}

func TestHuntValues_datasetUnicodeComment(t *testing.T) {
	applyThenEmpty(t, fmt.Sprintf(`
resource "truenas_dataset" "d" {
  name     = %q
  comments = "Zoë's \"quoted\" dataset — ünïcödé ✓ 日本語"
}
`, acctest.TestPool()+"/"+acctest.RandName("tfhuntval")))
}

func TestHuntValues_smbShareNameWithSpace(t *testing.T) {
	n := acctest.RandName("tfhuntval")
	applyThenEmpty(t, huntDS(n)+fmt.Sprintf(`
resource "truenas_smb_share" "s" {
  name    = "%s share"
  path    = truenas_dataset.fx.mountpoint
  comment = "has a space"
}
`, n))
}

func TestHuntValues_userFullNameQuotes(t *testing.T) {
	applyThenEmpty(t, fmt.Sprintf(`
resource "truenas_user" "u" {
  username          = %q
  full_name         = "O'Brien, \"Bill\" <bill@example.com>"
  group_create      = true
  password_disabled = true
  smb               = false
}
`, acctest.RandName("tfhuntval")))
}

// deletedOutOfBand: create, delete via API, then plan must be a create (not
// an error), and apply must recreate; final plan empty.
func deletedOutOfBand(t *testing.T, addr, cfg string, del func(s *terraform.State) error) {
	t.Helper()
	acctest.PreCheck(t)
	full := acctest.ProviderConfig() + cfg
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: full},
			{Config: full, Check: del, ExpectNonEmptyPlan: true},
			{Config: full, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
				plancheck.ExpectResourceAction(addr, plancheck.ResourceActionCreate),
			}}},
			{Config: full, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

func TestHuntValues_datasetDeletedOutOfBand(t *testing.T) {
	n := acctest.TestPool() + "/" + acctest.RandName("tfhuntval")
	deletedOutOfBand(t, "truenas_dataset.d", fmt.Sprintf(`
resource "truenas_dataset" "d" {
  name = %q
}
`, n), func(*terraform.State) error {
		_, err := acctest.Client().Call(context.Background(), "pool.dataset.delete", n)
		return err
	})
}

func TestHuntValues_nfsShareDeletedOutOfBand(t *testing.T) {
	n := acctest.RandName("tfhuntval")
	deletedOutOfBand(t, "truenas_nfs_share.s", huntDS(n)+`
resource "truenas_nfs_share" "s" {
  path = truenas_dataset.fx.mountpoint
}
`, func(s *terraform.State) error {
		var id int64
		fmt.Sscanf(idOf(s, "truenas_nfs_share.s"), "%d", &id)
		_, err := acctest.Client().Call(context.Background(), "sharing.nfs.delete", id)
		return err
	})
}

func TestHuntValues_userDeletedOutOfBand(t *testing.T) {
	n := acctest.RandName("tfhuntval")
	deletedOutOfBand(t, "truenas_user.u", fmt.Sprintf(`
resource "truenas_user" "u" {
  username          = %q
  full_name         = "gone"
  group_create      = true
  password_disabled = true
  smb               = false
}
`, n), func(s *terraform.State) error {
		var id int64
		fmt.Sscanf(idOf(s, "truenas_user.u"), "%d", &id)
		_, err := acctest.Client().Call(context.Background(), "user.delete", id, map[string]any{"delete_group": true})
		return err
	})
}

func TestHuntValues_cronjobDeletedOutOfBand(t *testing.T) {
	desc := acctest.RandName("tfhuntval")
	deletedOutOfBand(t, "truenas_cronjob.c", fmt.Sprintf(`
resource "truenas_cronjob" "c" {
  command     = "echo gone"
  user        = "root"
  description = %q
  schedule = {
    minute = "0"
    hour   = "1"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}
`, desc), func(s *terraform.State) error {
		var id int64
		fmt.Sscanf(idOf(s, "truenas_cronjob.c"), "%d", &id)
		_, err := acctest.Client().Call(context.Background(), "cronjob.delete", id)
		return err
	})
}

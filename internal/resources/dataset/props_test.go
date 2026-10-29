// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset_test

import (
	"fmt"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func dsName(p string) string {
	return fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfacc-st3-"+p))
}

// refreservation = 0 reads back 0 (the API reports parsed = null for a LOCAL
// 0). (B-090)
func TestAccDataset_refreservationZero(t *testing.T) {
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name           = %q
  refreservation = 0
  reservation    = 0
}
`, dsName("rr0"))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, Check: resource.TestCheckResourceAttr("truenas_dataset.test", "refreservation", "0")},
			{Config: cfg, ConfigPlanChecks: emptyPlan()},
		},
	})
}

// recordsize is case-insensitive: "1m" creates on every release (it is sent
// as "1M") and state keeps the configured spelling, so the plan stays empty;
// a byte count is a plan error. (B-126)
func TestAccDataset_recordsizeSpelling(t *testing.T) {
	name := dsName("rs")
	cfg := func(rs string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name       = %q
  recordsize = %q
}
`, name, rs)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg("1048576"), PlanOnly: true, ExpectError: regexp.MustCompile(`recordsize`)},
			{Config: cfg("1m"), Check: func(s *terraform.State) error {
				inst := acctest.GetInstance(t, "pool.dataset", name)
				acctest.AssertServerValue(t, inst, "recordsize.value", "1M")
				return resource.TestCheckResourceAttr("truenas_dataset.test", "recordsize", "1m")(s)
			}},
			{Config: cfg("1m"), ConfigPlanChecks: emptyPlan()},
			// A spelling-only change plans nothing.
			{Config: cfg("1M"), ConfigPlanChecks: emptyPlan()},
		},
	})
}

// On TrueNAS 26.0+ a non-default encryption_algorithm is a plan error instead
// of being ignored (26.0) or dropped (27.0+) and failing the apply. (B-056)
func TestAccDataset_encryptionAlgorithmFixed(t *testing.T) {
	acctest.PreCheck(t)
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("encryption_algorithm is still honored below TrueNAS 26.0")
	}
	name := dsName("ea")
	cfg := func(alg string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name                    = %q
  encryption              = true
  inherit_encryption      = false
  encryption_generate_key = true
  encryption_algorithm    = %q
}
`, name, alg)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg("AES-128-CCM"), PlanOnly: true, ExpectError: regexp.MustCompile(`encryption_algorithm cannot be chosen`)},
			{Config: cfg("AES-256-GCM"), Check: resource.TestCheckResourceAttr("truenas_dataset.test", "encryption_algorithm", "AES-256-GCM")},
			{Config: cfg("AES-256-GCM"), ConfigPlanChecks: emptyPlan()},
		},
	})
}

// encryption set with inherit_encryption unset under an encrypted parent:
// false fails with the server's clear error instead of creating an encrypted
// child (inconsistent result); true creates the child as its own encryption
// root instead of failing. (B-042)
func TestAccDataset_encryptionUnderEncryptedParent(t *testing.T) {
	parent := dsName("encp")
	cfg := func(enc bool) string {
		return encParentAndChild(parent, fmt.Sprintf("  encryption              = %t\n  encryption_generate_key = %t\n", enc, enc))
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg(false), ExpectError: regexp.MustCompile(`unencrypted dataset within an encrypted\s+dataset`)},
			{Config: cfg(true), Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("truenas_dataset.child", "encrypted", "true"),
				resource.TestCheckResourceAttr("truenas_dataset.child", "inherit_encryption", "false"),
			)},
			{Config: cfg(true), ConfigPlanChecks: emptyPlan()},
		},
	})
}

// acltype = "posix" (or "off") without aclmode creates on every release: 26.0
// rejected it unless aclmode was DISCARD, which 28.0 sets by itself. (B-148)
func TestAccDataset_posixWithoutAclmode(t *testing.T) {
	for _, at := range []string{"posix", "off"} {
		t.Run(at, func(t *testing.T) {
			cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name    = %q
  acltype = %q
}
`, dsName("px"), at)
			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { acctest.PreCheck(t) },
				ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{Config: cfg, Check: resource.TestCheckResourceAttr("truenas_dataset.test", "aclmode", "DISCARD")},
					{Config: cfg, ConfigPlanChecks: emptyPlan()},
				},
			})
		})
	}
}

// A dataset created without comments reads back "" on every release (26.0
// writes the literal "INHERIT" as its description). (B-147)
func TestAccDataset_commentsUnsetIsEmpty(t *testing.T) {
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name = %q
}
data "truenas_dataset" "test" {
  name = truenas_dataset.test.name
}
`, dsName("cm"))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("truenas_dataset.test", "comments", ""),
				resource.TestCheckResourceAttr("data.truenas_dataset.test", "comments", ""),
			)},
			{Config: cfg, ConfigPlanChecks: emptyPlan()},
		},
	})
}

// A thick VOLUME grown in place lets ZFS grow its refreservation; an
// unrelated update sends nothing else. (B-038 for truenas_dataset)
func TestAccDataset_volumeGrowFollowsRefreservation(t *testing.T) {
	name := dsName("vg")
	cfg := func(size int64) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name    = %q
  type    = "VOLUME"
  volsize = %d
}
`, name, size)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg(1073741824)},
			{
				Config: cfg(2147483648),
				Check: func(s *terraform.State) error {
					inst := acctest.GetInstance(t, "pool.dataset", name)
					rr, _ := acctest.ServerPath(inst, "refreservation.rawvalue")
					r, _ := strconv.ParseInt(fmt.Sprint(rr), 10, 64)
					if r < 2147483648 {
						return fmt.Errorf("thick volume grown to 2147483648 but refreservation stayed %d", r)
					}
					return resource.TestCheckResourceAttr("truenas_dataset.test", "refreservation", strconv.FormatInt(r, 10))(s)
				},
			},
			{Config: cfg(2147483648), ConfigPlanChecks: emptyPlan()},
		},
	})
}

// gzip levels 2-8 are accepted from TrueNAS 28.0. (B-157)
func TestAccDataset_compressionGzipLevel(t *testing.T) {
	acctest.PreCheck(t)
	if !acctest.ServerVersionAtLeast(t, 28, 0) {
		t.Skip("GZIP-2 to GZIP-8 are accepted from TrueNAS 28.0")
	}
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name        = %q
  compression = "GZIP-5"
}
`, dsName("gz"))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ConfigPlanChecks: emptyPlan()},
		},
	})
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol_test

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func zvName(p string) string {
	return fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfacc-st3-"+p))
}

// serverInt reads an integer ZFS property's rawvalue from get_instance.
func serverInt(t *testing.T, name, prop string) int64 {
	t.Helper()
	inst := acctest.GetInstance(t, "pool.dataset", name)
	raw, _ := acctest.ServerPath(inst, prop+".rawvalue")
	n, err := strconv.ParseInt(fmt.Sprint(raw), 10, 64)
	if err != nil {
		t.Fatalf("%s %s.rawvalue = %v: %v", name, prop, raw, err)
	}
	return n
}

// An unrelated update must not write the absent comments as a LOCAL empty
// description (the #62 fix ported to zvol). (B-039)
func TestAccZvol_commentsNotWrittenOnUnrelatedUpdate(t *testing.T) {
	name := zvName("zc")
	cfg := func(sync string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "test" {
  name    = %q
  volsize = 67108864
  sync    = %q
}
`, name, sync)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg("STANDARD"), Check: resource.TestCheckResourceAttr("truenas_zvol.test", "comments", "")},
			{
				Config: cfg("ALWAYS"),
				Check: func(_ *terraform.State) error {
					inst := acctest.GetInstance(t, "pool.dataset", name)
					if c, ok := acctest.ServerPath(inst, "user_properties.comments.value"); ok && c != "INHERIT" {
						return fmt.Errorf("unrelated sync update wrote the comments user property: %#v", c)
					}
					return nil
				},
			},
			{Config: cfg("ALWAYS"), ConfigPlanChecks: emptyPlanZ()},
		},
	})
}

// Growing a thick zvol with refreservation unset lets ZFS grow the
// refreservation with it; the stale value is no longer resent. (B-038)
func TestAccZvol_growFollowsRefreservation(t *testing.T) {
	name := zvName("zg")
	cfg := func(size int64) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "test" {
  name    = %q
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
					r, v := serverInt(t, name, "refreservation"), serverInt(t, name, "volsize")
					if r < v {
						return fmt.Errorf("thick zvol grown to volsize=%d but refreservation stayed %d", v, r)
					}
					return resource.TestCheckResourceAttr("truenas_zvol.test", "refreservation", strconv.FormatInt(r, 10))(s)
				},
			},
			{Config: cfg(2147483648), ConfigPlanChecks: emptyPlanZ()},
		},
	})
}

// A configured refreservation is kept when volsize grows (it is sent with the
// new volsize, so ZFS's automatic adjustment does not override it).
func TestAccZvol_growKeepsConfiguredRefreservation(t *testing.T) {
	name := zvName("zgk")
	cfg := func(size int64) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "test" {
  name           = %q
  volsize        = %d
  refreservation = 536870912
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
				Check: func(_ *terraform.State) error {
					if r := serverInt(t, name, "refreservation"); r != 536870912 {
						return fmt.Errorf("configured refreservation 536870912 became %d after the grow", r)
					}
					return nil
				},
			},
			{Config: cfg(2147483648), ConfigPlanChecks: emptyPlanZ()},
		},
	})
}

// refreservation = 0 / reservation = 0 read back 0 (the API reports parsed =
// null for a LOCAL 0). (B-090)
func TestAccZvol_reservationsZero(t *testing.T) {
	name := zvName("zr0")
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "test" {
  name           = %q
  volsize        = 67108864
  refreservation = 0
  reservation    = 0
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("truenas_zvol.test", "refreservation", "0"),
				resource.TestCheckResourceAttr("truenas_zvol.test", "reservation", "0"),
				resource.TestCheckResourceAttr("truenas_zvol.test", "sparse", "true"),
			)},
			{Config: cfg, ConfigPlanChecks: emptyPlanZ()},
		},
	})
}

// Import reads sparse from the refreservation, so an imported sparse zvol with
// sparse = true in config plans no change. (B-109)
func TestAccZvol_importSparse(t *testing.T) {
	for _, sparse := range []bool{true, false} {
		t.Run(strconv.FormatBool(sparse), func(t *testing.T) {
			name := zvName("zsp")
			cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "test" {
  name    = %q
  volsize = 67108864
  sparse  = %t
}
`, name, sparse)
			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { acctest.PreCheck(t) },
				ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{Config: cfg},
					{Config: acctest.ProviderConfig() + "\nremoved {\n  from = truenas_zvol.test\n  lifecycle {\n    destroy = false\n  }\n}\n"},
					{
						Config: cfg + fmt.Sprintf("\nimport {\n  to = truenas_zvol.test\n  id = %q\n}\n", name),
						ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
							plancheck.ExpectResourceAction("truenas_zvol.test", plancheck.ResourceActionNoop),
						}},
					},
				},
			})
		})
	}
}

// Changing sparse on an existing zvol is a plan error instead of an update
// that changes nothing on the server. (B-109)
func TestAccZvol_sparseChangeIsPlanError(t *testing.T) {
	name := zvName("zst")
	cfg := func(sp string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "test" {
  name    = %q
  volsize = 67108864
%s}
`, name, sp)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg("  sparse = true\n")},
			{Config: cfg("  sparse = false\n"), ExpectError: regexp.MustCompile(`sparse cannot be changed on an existing\s+zvol`)},
			// Removing sparse from the configuration keeps the recorded value.
			{Config: cfg(""), ConfigPlanChecks: emptyPlanZ()},
		},
	})
}

// encryption = false under an encrypted parent with inherit_encryption unset
// fails with the server's clear error instead of creating an encrypted zvol
// (inconsistent result); encryption = true there creates its own encryption
// root. (B-042)
func TestAccZvol_encryptionUnderEncryptedParent(t *testing.T) {
	parent := zvName("encp")
	cfg := func(enc bool) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "parent" {
  name                    = %q
  encryption              = true
  inherit_encryption      = false
  encryption_generate_key = true
}
resource "truenas_zvol" "child" {
  name                    = "${truenas_dataset.parent.name}/z"
  volsize                 = 67108864
  encryption              = %t
  encryption_generate_key = %t
}
`, parent, enc, enc)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg(false), ExpectError: regexp.MustCompile(`unencrypted dataset within an encrypted\s+dataset`)},
			{Config: cfg(true), Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("truenas_zvol.child", "encrypted", "true"),
				resource.TestCheckResourceAttr("truenas_zvol.child", "inherit_encryption", "false"),
			)},
			{Config: cfg(true), ConfigPlanChecks: emptyPlanZ()},
		},
	})
}

// On TrueNAS 26.0+ a non-default encryption_algorithm is a plan error instead
// of being ignored (26.0) or dropped (27.0+) and failing the apply. (B-056)
func TestAccZvol_encryptionAlgorithmFixed(t *testing.T) {
	acctest.PreCheck(t)
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("encryption_algorithm is still honored below TrueNAS 26.0")
	}
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "test" {
  name                    = %q
  volsize                 = 67108864
  encryption              = true
  inherit_encryption      = false
  encryption_generate_key = true
  encryption_algorithm    = "AES-128-CCM"
}
`, zvName("zea"))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, PlanOnly: true, ExpectError: regexp.MustCompile(`encryption_algorithm cannot be chosen`)},
		},
	})
}

// A zvol whose name has a space gets its /dev/zvol node under "+" (B-160): the
// create no longer waits the full device timeout and warns.
func TestAccZvol_nameWithSpace(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfacc-st3 zsp"))
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "test" {
  name    = %q
  volsize = 67108864
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, Check: func(_ *terraform.State) error {
				_, err := acctest.Client().CallRead(t.Context(), "filesystem.stat", "/dev/zvol/"+strings.ReplaceAll(name, " ", "+"))
				return err
			}},
		},
	})
}

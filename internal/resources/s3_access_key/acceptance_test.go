// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package s3_access_key_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
	"github.com/truenas/terraform-provider-truenas/internal/resources/s3_config"
)

func s3Check(t *testing.T) {
	t.Helper()
	acctest.PreCheck(t)
	if _, err := acctest.Client().CallRead(context.Background(), "s3.config"); err != nil {
		if s3_config.IsUnavailable(err) {
			t.Skip("s3.config does not exist on this box: the S3 API requires TrueNAS 27.0 or later")
		}
		t.Fatalf("s3.config: %v", err)
	}
}

// fixture is a local user whose group holds SHARING_S3_WRITE (needed for
// manage_buckets) plus the key under test.
func fixture(user, group, keyName string, enabled, manageBuckets bool, expires, extra string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_group" "g" {
  name = %[2]q
}
resource "truenas_privilege" "p" {
  name         = %[2]q
  local_groups = [truenas_group.g.gid]
  roles        = ["SHARING_S3_WRITE"]
  web_shell    = false
}
resource "truenas_user" "u" {
  username     = %[1]q
  full_name    = "S3 key test"
  group_create = true
  groups       = [truenas_group.g.id]
  password     = "Tf-Acc-S3-Passw0rd!"
  smb          = false
  shell        = "/usr/sbin/nologin"
  home         = "/var/empty"
  depends_on   = [truenas_privilege.p]
}
resource "truenas_s3_access_key" "k" {
  name           = %[3]q
  username       = truenas_user.u.username
  enabled        = %[4]t
  manage_buckets = %[5]t
%[6]s%[7]s}
`, user, group, keyName, enabled, manageBuckets, expires, extra)
}

func stateAttr(s *terraform.State, addr, attr string) string {
	rs, ok := s.RootModule().Resources[addr]
	if !ok {
		return ""
	}
	return rs.Primary.Attributes[attr]
}

// TestAccS3AccessKey_generated: generated access key and secret, enabled /
// manage_buckets / expires_at updates, rotation (new secret, same access key),
// import (secret not imported), empty plan after.
func TestAccS3AccessKey_generated(t *testing.T) {
	s3Check(t)
	user := acctest.RandName("tfaccs3u")
	group := acctest.RandName("tfaccs3g")
	keyName := acctest.RandName("tfaccs3k")
	var accessKey, secret string
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fixture(user, group, keyName, true, true, "  expires_at = \"2035-01-01T00:00:00Z\"\n", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_s3_access_key.k", "id"),
					resource.TestMatchResourceAttr("truenas_s3_access_key.k", "access_key", regexp.MustCompile(`^[A-Z0-9]{16,128}$`)),
					resource.TestCheckResourceAttrSet("truenas_s3_access_key.k", "secret"),
					resource.TestCheckResourceAttr("truenas_s3_access_key.k", "username", user),
					resource.TestCheckResourceAttr("truenas_s3_access_key.k", "enabled", "true"),
					resource.TestCheckResourceAttr("truenas_s3_access_key.k", "manage_buckets", "true"),
					resource.TestCheckResourceAttr("truenas_s3_access_key.k", "expires_at", "2035-01-01T00:00:00Z"),
					resource.TestCheckResourceAttr("truenas_s3_access_key.k", "local", "true"),
					resource.TestCheckResourceAttr("truenas_s3_access_key.k", "status", "ENABLED"),
					resource.TestCheckResourceAttrSet("truenas_s3_access_key.k", "created_at"),
					resource.TestCheckResourceAttrSet("truenas_s3_access_key.k", "user_identifier"),
					resource.TestCheckNoResourceAttr("truenas_s3_access_key.k", "last_used_at"),
					func(s *terraform.State) error {
						accessKey, secret = stateAttr(s, "truenas_s3_access_key.k", "access_key"), stateAttr(s, "truenas_s3_access_key.k", "secret")
						return nil
					},
				),
			},
			{Config: fixture(user, group, keyName, true, true, "  expires_at = \"2035-01-01T00:00:00Z\"\n", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			// Disable, drop manage_buckets and the expiry, rename: in place.
			{
				Config: fixture(user, group, keyName+"-2", false, false, "", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_s3_access_key.k", plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_s3_access_key.k", "name", keyName+"-2"),
					resource.TestCheckResourceAttr("truenas_s3_access_key.k", "enabled", "false"),
					resource.TestCheckResourceAttr("truenas_s3_access_key.k", "manage_buckets", "false"),
					resource.TestCheckNoResourceAttr("truenas_s3_access_key.k", "expires_at"),
					resource.TestCheckResourceAttr("truenas_s3_access_key.k", "status", "DISABLED"),
					func(s *terraform.State) error {
						if a := stateAttr(s, "truenas_s3_access_key.k", "access_key"); a != accessKey {
							return fmt.Errorf("access_key changed on update: %s -> %s", accessKey, a)
						}
						if sec := stateAttr(s, "truenas_s3_access_key.k", "secret"); sec != secret {
							return fmt.Errorf("secret changed on a plain update")
						}
						return nil
					},
				),
			},
			// Rotate: same access key, new secret.
			{
				Config: fixture(user, group, keyName+"-2", false, false, "", "  rotate_version = 1\n"),
				Check: func(s *terraform.State) error {
					if a := stateAttr(s, "truenas_s3_access_key.k", "access_key"); a != accessKey {
						return fmt.Errorf("access_key changed on rotate: %s -> %s", accessKey, a)
					}
					sec := stateAttr(s, "truenas_s3_access_key.k", "secret")
					if sec == "" || sec == secret {
						return fmt.Errorf("rotate did not produce a new secret")
					}
					return nil
				},
			},
			{Config: fixture(user, group, keyName+"-2", false, false, "", "  rotate_version = 1\n"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				Config:            fixture(user, group, keyName+"-2", false, false, "", "  rotate_version = 1\n"),
				ResourceName:      "truenas_s3_access_key.k",
				ImportState:       true,
				ImportStateVerify: true,
				// The secret is never read back; the version triggers are configuration-only.
				ImportStateVerifyIgnore: []string{"secret", "rotate_version"},
			},
		},
	})
}

// TestAccS3AccessKey_suppliedSecret: access key id and secret from the
// configuration; the secret is write-only and never lands in state.
func TestAccS3AccessKey_suppliedSecret(t *testing.T) {
	s3Check(t)
	user := acctest.RandName("tfaccs3u")
	group := acctest.RandName("tfaccs3g")
	keyName := acctest.RandName("tfaccs3k")
	ak := "TFACCS3KEY0000000001"
	extra := fmt.Sprintf("  access_key        = %q\n  secret_wo         = \"tf-acc-supplied-secret-%s\"\n  secret_wo_version = 1\n", ak, acctest.RandName("s"))
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fixture(user, group, keyName, true, false, "", extra),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_s3_access_key.k", "access_key", ak),
					resource.TestCheckNoResourceAttr("truenas_s3_access_key.k", "secret"),
					resource.TestCheckNoResourceAttr("truenas_s3_access_key.k", "secret_wo"),
				),
			},
			{Config: fixture(user, group, keyName, true, false, "", extra),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			// A new secret_wo_version recreates the key (the API cannot set a chosen secret on an existing key).
			{
				Config: fixture(user, group, keyName, true, false, "", fmt.Sprintf("  access_key        = %q\n  secret_wo         = \"tf-acc-supplied-secret-v2-%s\"\n  secret_wo_version = 2\n", ak, acctest.RandName("s"))),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_s3_access_key.k", plancheck.ResourceActionReplace),
				}},
				Check: resource.TestCheckResourceAttr("truenas_s3_access_key.k", "access_key", ak),
			},
			{
				Config: acctest.ProviderConfig() + fmt.Sprintf(`
data "truenas_s3_access_key" "k" {
  name = %q
}
`, keyName) + fixture(user, group, keyName, true, false, "", fmt.Sprintf("  access_key        = %q\n  secret_wo         = \"tf-acc-supplied-secret-v2-%s\"\n  secret_wo_version = 2\n", ak, acctest.RandName("s")))[len(acctest.ProviderConfig()):],
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_s3_access_key.k", "access_key", ak),
					resource.TestCheckResourceAttr("data.truenas_s3_access_key.k", "username", user),
				),
			},
		},
	})
}

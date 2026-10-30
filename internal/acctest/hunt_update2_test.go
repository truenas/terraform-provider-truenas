// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

// Update-path hunt, batch 2: zvol (grow only — shrink is its own question),
// group, iscsi extent/target, alert_service, api_key, keychain keypair.

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// huntUpdateOneWay: A → B (no replace) → minimal, each followed by an empty
// re-plan. For resources where going back to A is not legal (volsize shrink).
func huntUpdateOneWay(t *testing.T, addr, cfgA, cfgB, cfgMin string) {
	t.Helper()
	acctest.PreCheck(t)
	p := acctest.ProviderConfig()
	empty := resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
	noReplace := resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{acctest.ExpectNoReplace(addr)}}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: p + cfgA},
			{Config: p + cfgA, ConfigPlanChecks: empty},
			{Config: p + cfgB, ConfigPlanChecks: noReplace},
			{Config: p + cfgB, ConfigPlanChecks: empty},
			{Config: p + cfgMin, ConfigPlanChecks: noReplace},
			{Config: p + cfgMin, ConfigPlanChecks: empty},
		},
	})
}

func TestHuntUpdate_zvolGrow(t *testing.T) {
	n := acctest.TestPool() + "/" + acctest.RandName("tfhuntupdz")
	a := fmt.Sprintf(`
resource "truenas_zvol" "z" {
  name        = %q
  volsize     = 33554432
  sparse      = true
  compression = "lz4"
  sync        = "STANDARD"
  comments    = "hunt A"
  readonly    = "OFF"
}
`, n)
	b := fmt.Sprintf(`
resource "truenas_zvol" "z" {
  name        = %q
  volsize     = 67108864
  sparse      = true
  compression = "zstd"
  sync        = "ALWAYS"
  comments    = "hunt B"
  readonly    = "ON"
}
`, n)
	m := fmt.Sprintf(`
resource "truenas_zvol" "z" {
  name    = %q
  volsize = 67108864
}
`, n)
	huntUpdateOneWay(t, "truenas_zvol.z", a, b, m)
}

func TestHuntUpdate_group(t *testing.T) {
	n := acctest.RandName("tfhuntupdg")
	a := fmt.Sprintf(`
resource "truenas_group" "g" {
  name          = %q
  smb           = true
  sudo_commands = ["/usr/bin/id"]
}
`, n)
	b := fmt.Sprintf(`
resource "truenas_group" "g" {
  name                   = %q
  smb                    = false
  sudo_commands          = ["/usr/bin/id", "/usr/bin/whoami"]
  sudo_commands_nopasswd = ["/usr/bin/uptime"]
}
`, n)
	m := fmt.Sprintf(`
resource "truenas_group" "g" {
  name = %q
}
`, n)
	huntUpdateRoundTrip(t, "truenas_group.g", a, b, m)
}

func TestHuntUpdate_iscsiExtent(t *testing.T) {
	n := acctest.RandName("tfhuntupde")
	z := fmt.Sprintf(`
resource "truenas_zvol" "z" {
  name    = %q
  volsize = 33554432
  sparse  = true
}
`, acctest.TestPool()+"/"+n)
	a := z + fmt.Sprintf(`
resource "truenas_iscsi_extent" "e" {
  name    = %q
  type    = "DISK"
  disk    = "zvol/${truenas_zvol.z.name}"
  comment = "hunt A"
  ro      = false
  enabled = true
  rpm     = "SSD"
}
`, n)
	b := z + fmt.Sprintf(`
resource "truenas_iscsi_extent" "e" {
  name         = %q
  type         = "DISK"
  disk         = "zvol/${truenas_zvol.z.name}"
  comment      = "hunt B"
  ro           = true
  enabled      = false
  rpm          = "7200"
  insecure_tpc = false
  xen          = true
}
`, n)
	m := z + fmt.Sprintf(`
resource "truenas_iscsi_extent" "e" {
  name = %q
  type = "DISK"
  disk = "zvol/${truenas_zvol.z.name}"
}
`, n)
	huntUpdateRoundTrip(t, "truenas_iscsi_extent.e", a, b, m)
}

func TestHuntUpdate_iscsiTarget(t *testing.T) {
	n := acctest.RandName("tfhuntupdt")
	a := fmt.Sprintf(`
resource "truenas_iscsi_target" "t" {
  name  = %q
  alias = "hunt A"
}
`, n)
	b := fmt.Sprintf(`
resource "truenas_iscsi_target" "t" {
  name  = %q
  alias = "hunt B"
}
`, n)
	m := fmt.Sprintf(`
resource "truenas_iscsi_target" "t" {
  name = %q
}
`, n)
	huntUpdateRoundTrip(t, "truenas_iscsi_target.t", a, b, m)
}

func TestHuntUpdate_alertService(t *testing.T) {
	n := acctest.RandName("tfhuntupda")
	a := fmt.Sprintf(`
resource "truenas_alert_service" "a" {
  name       = %q
  level      = "WARNING"
  enabled    = true
  attributes = jsonencode({ type = "Slack", url = "https://hooks.slack.invalid/a" })
}
`, n)
	b := fmt.Sprintf(`
resource "truenas_alert_service" "a" {
  name       = %q
  level      = "CRITICAL"
  enabled    = false
  attributes = jsonencode({ type = "Slack", url = "https://hooks.slack.invalid/b" })
}
`, n)
	m := fmt.Sprintf(`
resource "truenas_alert_service" "a" {
  name       = %q
  level      = "WARNING"
  attributes = jsonencode({ type = "Slack", url = "https://hooks.slack.invalid/a" })
}
`, n)
	huntUpdateRoundTrip(t, "truenas_alert_service.a", a, b, m)
}

func TestHuntUpdate_apiKey(t *testing.T) {
	n := acctest.RandName("tfhuntupdk")
	a := fmt.Sprintf(`
resource "truenas_api_key" "k" {
  name       = %q
  username   = "root"
  expires_at = "2035-06-15T12:30:00Z"
}
`, n)
	b := fmt.Sprintf(`
resource "truenas_api_key" "k" {
  name       = "%s-renamed"
  username   = "root"
  expires_at = "2036-01-01T00:00:00Z"
}
`, n)
	m := fmt.Sprintf(`
resource "truenas_api_key" "k" {
  name     = %q
  username = "root"
}
`, n)
	huntUpdateRoundTrip(t, "truenas_api_key.k", a, b, m)
}

func TestHuntUpdate_keychainKeypairRename(t *testing.T) {
	n := acctest.RandName("tfhuntupdkp")
	a := fmt.Sprintf(`
resource "truenas_keychain_ssh_keypair" "k" {
  name     = %q
  generate = true
}
`, n)
	b := fmt.Sprintf(`
resource "truenas_keychain_ssh_keypair" "k" {
  name     = "%s-renamed"
  generate = true
}
`, n)
	huntUpdateRoundTrip(t, "truenas_keychain_ssh_keypair.k", a, b, a)
}

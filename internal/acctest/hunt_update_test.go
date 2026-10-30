// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

// Adversarial bug hunt: the UPDATE path, which is where users find bugs and
// where create-shaped tests never look (create-only fields rejected by
// X.update, derived Computed values going stale, Optional+Computed attributes
// that silently keep the old server value when dropped from config).
//
// For each resource: apply A, flip every mutable field to B (must be an
// in-place update, never a replace), flip back to A, then drop every optional
// attribute (the "minimal" config). After every apply the re-plan must be empty.

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func huntUpdateRoundTrip(t *testing.T, addr, cfgA, cfgB, cfgMin string) {
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
			{Config: p + cfgA, ConfigPlanChecks: noReplace},
			{Config: p + cfgA, ConfigPlanChecks: empty},
			{Config: p + cfgMin, ConfigPlanChecks: noReplace},
			{Config: p + cfgMin, ConfigPlanChecks: empty},
		},
	})
}

func TestHuntUpdate_dataset(t *testing.T) {
	n := acctest.TestPool() + "/" + acctest.RandName("tfhuntupds")
	a := fmt.Sprintf(`
resource "truenas_dataset" "d" {
  name        = %q
  compression = "lz4"
  atime       = "ON"
  sync        = "STANDARD"
  comments    = "hunt A"
  readonly    = "OFF"
  exec        = "ON"
  snapdir     = "HIDDEN"
  copies      = 1
  recordsize  = "128K"
}
`, n)
	b := fmt.Sprintf(`
resource "truenas_dataset" "d" {
  name        = %q
  compression = "zstd"
  atime       = "OFF"
  sync        = "ALWAYS"
  comments    = "hunt B"
  readonly    = "ON"
  exec        = "OFF"
  snapdir     = "VISIBLE"
  copies      = 2
  recordsize  = "1M"
}
`, n)
	m := fmt.Sprintf(`
resource "truenas_dataset" "d" {
  name = %q
}
`, n)
	huntUpdateRoundTrip(t, "truenas_dataset.d", a, b, m)
}

func TestHuntUpdate_user(t *testing.T) {
	n := acctest.RandName("tfhuntupdu")
	a := fmt.Sprintf(`
resource "truenas_user" "u" {
  username      = %q
  full_name     = "hunt A"
  group_create  = true
  email         = "a@example.com"
  shell         = "/usr/bin/bash"
  password      = "TfHunt-Upd-Pw-1"
  smb           = true
  locked        = false
  sudo_commands = ["/usr/bin/id"]
}
`, n)
	b := fmt.Sprintf(`
resource "truenas_user" "u" {
  username               = %q
  full_name              = "hunt B"
  group_create           = true
  email                  = "b@example.com"
  shell                  = "/usr/sbin/nologin"
  password               = "TfHunt-Upd-Pw-1"
  smb                    = false
  locked                 = true
  sudo_commands          = ["/usr/bin/id", "/usr/bin/whoami"]
  sudo_commands_nopasswd = ["/usr/bin/uptime"]
}
`, n)
	m := fmt.Sprintf(`
resource "truenas_user" "u" {
  username     = %q
  full_name    = "hunt A"
  group_create = true
  password     = "TfHunt-Upd-Pw-1"
}
`, n)
	huntUpdateRoundTrip(t, "truenas_user.u", a, b, m)
}

func TestHuntUpdate_smbShare(t *testing.T) {
	n := acctest.RandName("tfhuntupdsmb")
	ds := huntDS(n)
	a := ds + fmt.Sprintf(`
resource "truenas_smb_share" "s" {
  name       = %q
  path       = truenas_dataset.fx.mountpoint
  comment    = "hunt A"
  ro         = false
  browsable  = true
  guestok    = false
  enabled    = true
  abe        = false
  recyclebin = false
}
`, n)
	b := ds + fmt.Sprintf(`
resource "truenas_smb_share" "s" {
  name       = %q
  path       = truenas_dataset.fx.mountpoint
  comment    = "hunt B"
  ro         = true
  browsable  = false
  guestok    = true
  enabled    = false
  abe        = true
  recyclebin = true
}
`, n)
	m := ds + fmt.Sprintf(`
resource "truenas_smb_share" "s" {
  name = %q
  path = truenas_dataset.fx.mountpoint
}
`, n)
	huntUpdateRoundTrip(t, "truenas_smb_share.s", a, b, m)
}

func TestHuntUpdate_nfsShare(t *testing.T) {
	n := acctest.RandName("tfhuntupdnfs")
	ds := huntDS(n)
	a := ds + `
resource "truenas_nfs_share" "s" {
  path          = truenas_dataset.fx.mountpoint
  comment       = "hunt A"
  ro            = false
  enabled       = true
  maproot_user  = "root"
  maproot_group = "wheel"
  networks      = ["192.0.2.0/24"]
}
`
	b := ds + `
resource "truenas_nfs_share" "s" {
  path          = truenas_dataset.fx.mountpoint
  comment       = "hunt B"
  ro            = true
  enabled       = false
  maproot_user  = "nobody"
  maproot_group = "nogroup"
  networks      = ["198.51.100.0/24", "203.0.113.0/24"]
  hosts         = ["192.0.2.10"]
}
`
	m := ds + `
resource "truenas_nfs_share" "s" {
  path = truenas_dataset.fx.mountpoint
}
`
	huntUpdateRoundTrip(t, "truenas_nfs_share.s", a, b, m)
}

func TestHuntUpdate_periodicSnapshot(t *testing.T) {
	n := acctest.RandName("tfhuntupdps")
	ds := huntDS(n)
	a := ds + `
resource "truenas_periodic_snapshot_task" "p" {
  dataset        = truenas_dataset.fx.name
  recursive      = false
  lifetime_value = 1
  lifetime_unit  = "WEEK"
  naming_schema  = "auto-%Y-%m-%d_%H-%M"
  enabled        = true
  allow_empty    = true
  schedule = {
    minute = "0"
    hour   = "0"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}
`
	b := ds + `
resource "truenas_periodic_snapshot_task" "p" {
  dataset        = truenas_dataset.fx.name
  recursive      = true
  lifetime_value = 2
  lifetime_unit  = "MONTH"
  naming_schema  = "hunt-%Y%m%d-%H%M"
  enabled        = false
  allow_empty    = false
  schedule = {
    minute = "15"
    hour   = "3"
    dom    = "1"
    month  = "*"
    dow    = "0"
  }
}
`
	m := ds + `
resource "truenas_periodic_snapshot_task" "p" {
  dataset        = truenas_dataset.fx.name
  lifetime_value = 1
  lifetime_unit  = "WEEK"
  naming_schema  = "auto-%Y-%m-%d_%H-%M"
  schedule = {
    minute = "0"
    hour   = "0"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}
`
	huntUpdateRoundTrip(t, "truenas_periodic_snapshot_task.p", a, b, m)
}

func TestHuntUpdate_cronjob(t *testing.T) {
	a := `
resource "truenas_cronjob" "c" {
  command     = "echo hunt-a"
  user        = "root"
  description = "hunt A"
  enabled     = true
  stdout      = true
  stderr      = false
  schedule = {
    minute = "0"
    hour   = "1"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}
`
	b := `
resource "truenas_cronjob" "c" {
  command     = "echo hunt-b"
  user        = "root"
  description = "hunt B"
  enabled     = false
  stdout      = false
  stderr      = true
  schedule = {
    minute = "30"
    hour   = "2"
    dom    = "1"
    month  = "6"
    dow    = "1"
  }
}
`
	m := `
resource "truenas_cronjob" "c" {
  command = "echo hunt-a"
  user    = "root"
  schedule = {
    minute = "0"
    hour   = "1"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}
`
	huntUpdateRoundTrip(t, "truenas_cronjob.c", a, b, m)
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

// Adversarial bug hunt: DATA SOURCE value parity. The read path (data
// sources) lags the write path (resources). Create an object with the
// resource, read it back through the same-named data source in the same
// config, and require every shared attribute to be equal. A mismatch is a
// read-path bug a user hits the moment they reference an existing object.

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func parity(res, ds string, attrs ...string) resource.TestCheckFunc {
	var checks []resource.TestCheckFunc
	for _, a := range attrs {
		checks = append(checks, resource.TestCheckResourceAttrPair(res, a, ds, a))
	}
	return resource.ComposeAggregateTestCheckFunc(checks...)
}

func huntParity(t *testing.T, cfg string, check resource.TestCheckFunc) {
	t.Helper()
	acctest.PreCheck(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps:                    []resource.TestStep{{Config: acctest.ProviderConfig() + cfg, Check: check}},
	})
}

func TestHuntDSParity_dataset(t *testing.T) {
	n := acctest.TestPool() + "/" + acctest.RandName("tfhuntpar")
	huntParity(t, fmt.Sprintf(`
resource "truenas_dataset" "d" {
  name        = %q
  compression = "zstd"
  atime       = "OFF"
  sync        = "ALWAYS"
  comments    = "parity"
  recordsize  = "1M"
  readonly    = "ON"
  snapdir     = "VISIBLE"
  copies      = 2
  acltype     = "nfsv4"
  aclmode     = "RESTRICTED"
}
data "truenas_dataset" "d" {
  name       = truenas_dataset.d.name
  depends_on = [truenas_dataset.d]
}
`, n), parity("truenas_dataset.d", "data.truenas_dataset.d",
		"id", "name", "pool", "type", "mountpoint", "compression", "atime", "sync", "comments",
		"recordsize", "readonly", "snapdir", "copies", "acltype", "aclmode", "encrypted"))
}

func TestHuntDSParity_user(t *testing.T) {
	n := acctest.RandName("tfhuntpar")
	huntParity(t, fmt.Sprintf(`
resource "truenas_user" "u" {
  username      = %q
  full_name     = "parity"
  group_create  = true
  password      = "TfHunt-Par-Pw-1"
  email         = "parity@example.com"
  shell         = "/usr/bin/bash"
  smb           = true
  sudo_commands = ["/usr/bin/id"]
}
data "truenas_user" "u" {
  username   = truenas_user.u.username
  depends_on = [truenas_user.u]
}
`, n), parity("truenas_user.u", "data.truenas_user.u",
		"id", "username", "uid", "full_name", "email", "shell", "home", "smb", "locked",
		"password_disabled", "sudo_commands", "group", "builtin"))
}

func TestHuntDSParity_group(t *testing.T) {
	n := acctest.RandName("tfhuntpar")
	huntParity(t, fmt.Sprintf(`
resource "truenas_group" "g" {
  name          = %q
  smb           = true
  sudo_commands = ["/usr/bin/id"]
}
data "truenas_group" "g" {
  name       = truenas_group.g.name
  depends_on = [truenas_group.g]
}
`, n), parity("truenas_group.g", "data.truenas_group.g", "id", "name", "gid", "smb", "sudo_commands", "builtin"))
}

func TestHuntDSParity_smbShare(t *testing.T) {
	n := acctest.RandName("tfhuntpar")
	huntParity(t, huntDS(n)+fmt.Sprintf(`
resource "truenas_smb_share" "s" {
  name      = %q
  path      = truenas_dataset.fx.mountpoint
  comment   = "parity"
  ro        = true
  browsable = false
  enabled   = false
}
data "truenas_smb_share" "s" {
  name       = truenas_smb_share.s.name
  depends_on = [truenas_smb_share.s]
}
`, n), parity("truenas_smb_share.s", "data.truenas_smb_share.s",
		"id", "name", "path", "comment", "ro", "browsable", "enabled", "purpose", "guestok", "abe", "recyclebin"))
}

func TestHuntDSParity_nfsShare(t *testing.T) {
	n := acctest.RandName("tfhuntpar")
	huntParity(t, huntDS(n)+`
resource "truenas_nfs_share" "s" {
  path          = truenas_dataset.fx.mountpoint
  comment       = "parity"
  ro            = true
  enabled       = false
  maproot_user  = "root"
  maproot_group = "wheel"
  networks      = ["192.0.2.0/24"]
  hosts         = ["192.0.2.10"]
}
data "truenas_nfs_share" "s" {
  id         = truenas_nfs_share.s.id
  depends_on = [truenas_nfs_share.s]
}
`, parity("truenas_nfs_share.s", "data.truenas_nfs_share.s",
		"id", "path", "comment", "ro", "enabled", "maproot_user", "maproot_group", "networks", "hosts", "security"))
}

func TestHuntDSParity_cronjob(t *testing.T) {
	desc := acctest.RandName("tfhuntpar")
	huntParity(t, fmt.Sprintf(`
resource "truenas_cronjob" "c" {
  command     = "echo parity"
  user        = "root"
  description = %q
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
data "truenas_cronjob" "c" {
  description = truenas_cronjob.c.description
  depends_on  = [truenas_cronjob.c]
}
`, desc), parity("truenas_cronjob.c", "data.truenas_cronjob.c",
		"id", "command", "user", "description", "enabled", "stdout", "stderr",
		"schedule.minute", "schedule.hour", "schedule.dom", "schedule.month", "schedule.dow"))
}

func TestHuntDSParity_zvol(t *testing.T) {
	n := acctest.TestPool() + "/" + acctest.RandName("tfhuntpar")
	huntParity(t, fmt.Sprintf(`
resource "truenas_zvol" "z" {
  name         = %q
  volsize      = 33554432
  volblocksize = 32768
  sparse       = true
  compression  = "zstd"
  sync         = "ALWAYS"
  comments     = "parity"
}
data "truenas_zvol" "z" {
  name       = truenas_zvol.z.name
  depends_on = [truenas_zvol.z]
}
`, n), parity("truenas_zvol.z", "data.truenas_zvol.z",
		"id", "name", "volsize", "volblocksize", "compression", "sync", "comments", "readonly"))
}

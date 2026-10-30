// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

// Adversarial bug hunt: import objects a user CUSTOMIZED in the UI, with the
// minimal HCL a user writes to adopt them (name/path only). The API-default
// probes in hunt_external_import_test.go pass because the server defaults
// already matched; here every optional field is deliberately non-default on
// the box. Two acceptable outcomes exist for an unset attribute — keep the
// server value (classic Optional+Computed) or reset it to the default
// (1.6.0's documented share semantics) — but a plan that silently rewrites a
// user's UI settings on adoption is a trap, and resources disagreeing with
// each other is a finding either way.

import (
	"fmt"
	"testing"

	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestHuntExternalCustom_smbShare(t *testing.T) {
	acctest.PreCheck(t)
	ds := acctest.TestPool() + "/" + acctest.RandName("tfhuntcsmb")
	apiCreate(t, "pool.dataset.create", map[string]any{"name": ds}, "")
	t.Cleanup(func() { apiDelete("pool.dataset.delete", ds) })
	share := acctest.RandName("tfhuntcsmb")
	// 26.0+ keeps the legacy flags under options{} behind a purpose.
	id := apiCreate(t, "sharing.smb.create", map[string]any{
		"name": share, "path": "/mnt/" + ds,
		"comment": "set in UI", "browsable": false, "enabled": false,
		"readonly": true, "access_based_share_enumeration": true,
		"purpose": "LEGACY_SHARE",
		"options": map[string]any{"guestok": true, "recyclebin": true},
	}, "")
	t.Cleanup(func() { apiDelete("sharing.smb.delete", mustInt(t, id)) })
	huntImportExternal(t, "truenas_smb_share.s", fmt.Sprintf(`
resource "truenas_smb_share" "s" {
  name = %q
  path = %q
}
`, share, "/mnt/"+ds), id)
}

func TestHuntExternalCustom_nfsShare(t *testing.T) {
	acctest.PreCheck(t)
	ds := acctest.TestPool() + "/" + acctest.RandName("tfhuntcnfs")
	apiCreate(t, "pool.dataset.create", map[string]any{"name": ds}, "")
	t.Cleanup(func() { apiDelete("pool.dataset.delete", ds) })
	id := apiCreate(t, "sharing.nfs.create", map[string]any{
		"path": "/mnt/" + ds, "comment": "set in UI", "ro": true, "enabled": false,
		"maproot_user": "root", "maproot_group": "wheel",
		"networks": []string{"192.0.2.0/24"}, "hosts": []string{"192.0.2.10"},
	}, "")
	t.Cleanup(func() { apiDelete("sharing.nfs.delete", mustInt(t, id)) })
	huntImportExternal(t, "truenas_nfs_share.s", fmt.Sprintf(`
resource "truenas_nfs_share" "s" {
  path = %q
}
`, "/mnt/"+ds), id)
}

func TestHuntExternalCustom_dataset(t *testing.T) {
	acctest.PreCheck(t)
	name := acctest.TestPool() + "/" + acctest.RandName("tfhuntcds")
	apiCreate(t, "pool.dataset.create", map[string]any{
		"name": name, "compression": "ZSTD", "atime": "OFF", "sync": "DISABLED",
		"comments": "set in UI", "recordsize": "1M", "readonly": "ON", "snapdir": "VISIBLE", "copies": 2,
	}, "")
	t.Cleanup(func() { apiDelete("pool.dataset.delete", name) })
	huntImportExternal(t, "truenas_dataset.d", fmt.Sprintf(`
resource "truenas_dataset" "d" {
  name = %q
}
`, name), name)
}

func TestHuntExternalCustom_user(t *testing.T) {
	acctest.PreCheck(t)
	name := acctest.RandName("tfhuntcu")
	id := apiCreate(t, "user.create", map[string]any{
		"username": name, "full_name": "set in UI", "group_create": true, "password": "TfHunt-Cust-Pw-1",
		"email": "ui@example.com", "shell": "/usr/sbin/nologin", "locked": true, "smb": false,
		"sudo_commands": []string{"/usr/bin/id"}, "sudo_commands_nopasswd": []string{"/usr/bin/uptime"},
	}, "")
	t.Cleanup(func() { apiDelete("user.delete", mustInt(t, id), map[string]any{"delete_group": true}) })
	huntImportExternal(t, "truenas_user.u", fmt.Sprintf(`
resource "truenas_user" "u" {
  username  = %q
  full_name = "set in UI"
}
`, name), id)
}

func TestHuntExternalCustom_cronjob(t *testing.T) {
	acctest.PreCheck(t)
	desc := acctest.RandName("tfhuntccron")
	id := apiCreate(t, "cronjob.create", map[string]any{
		"command": "echo " + desc, "user": "root", "description": desc, "enabled": false,
		"stdout": false, "stderr": true,
		"schedule": map[string]any{"minute": "30", "hour": "2", "dom": "1", "month": "6", "dow": "1"},
	}, "")
	t.Cleanup(func() { apiDelete("cronjob.delete", mustInt(t, id)) })
	huntImportExternal(t, "truenas_cronjob.c", fmt.Sprintf(`
resource "truenas_cronjob" "c" {
  command     = %q
  user        = "root"
  description = %q
  schedule = {
    minute = "30"
    hour   = "2"
    dom    = "1"
    month  = "6"
    dow    = "1"
  }
}
`, "echo "+desc, desc), id)
}

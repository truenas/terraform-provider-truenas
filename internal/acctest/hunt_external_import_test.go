// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

// Adversarial bug hunt: import objects that Terraform did NOT create.
//
// Every earlier import test imported an object the provider itself had just
// created, so the server state already matched the provider's idea of the
// defaults — a tautology. Users adopt objects made in the UI/API, where the
// server filled in every default. Create such an object through the API with
// the minimum payload, import it with the minimal HCL a user would write, and
// require an empty plan. A non-empty plan here is the Optional+Computed /
// not-read-back class of bug (GH #32) in the exact shape users hit it.

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// apiCreate calls method with params and returns the "id" of the result as a
// string (int or string ids), or the value under idKey when given.
func apiCreate(t *testing.T, method string, params any, idKey string) string {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), method, params)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		// some creates return a bare id/name
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return s
		}
		var n float64
		if json.Unmarshal(raw, &n) == nil {
			return fmt.Sprintf("%d", int64(n))
		}
		t.Fatalf("%s: cannot parse result %s", method, raw)
	}
	v := m["id"]
	if idKey != "" {
		v = m[idKey]
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return fmt.Sprintf("%d", int64(x))
	}
	t.Fatalf("%s: no usable id in %s", method, raw)
	return ""
}

// apiDelete is best-effort cleanup for an object the test's destroy may have
// already removed.
func apiDelete(method string, params ...any) {
	_, _ = acctest.Client().Call(context.Background(), method, params...)
}

// huntImportExternal imports the API-created object at importID into addr
// using cfg, then requires an empty plan. Terraform destroys it at the end.
func huntImportExternal(t *testing.T, addr, cfg, importID string) {
	t.Helper()
	full := acctest.ProviderConfig() + cfg
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:             full,
				ResourceName:       addr,
				ImportState:        true,
				ImportStatePersist: true,
				ImportStateId:      importID,
			},
			{
				Config:           full,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{logPlanDiff{t}, plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

// A dataset made with nothing but a name.
func TestHuntExternal_datasetDefaults(t *testing.T) {
	acctest.PreCheck(t)
	name := acctest.TestPool() + "/" + acctest.RandName("tfhuntext")
	apiCreate(t, "pool.dataset.create", map[string]any{"name": name}, "")
	t.Cleanup(func() { apiDelete("pool.dataset.delete", name) })
	huntImportExternal(t, "truenas_dataset.d", fmt.Sprintf(`
resource "truenas_dataset" "d" {
  name = %q
}
`, name), name)
}

// A group made with nothing but a name.
func TestHuntExternal_groupDefaults(t *testing.T) {
	acctest.PreCheck(t)
	name := acctest.RandName("tfhuntextg")
	id := apiCreate(t, "group.create", map[string]any{"name": name}, "")
	t.Cleanup(func() { apiDelete("group.delete", mustInt(t, id)) })
	huntImportExternal(t, "truenas_group.g", fmt.Sprintf(`
resource "truenas_group" "g" {
  name = %q
}
`, name), id)
}

// A user made the way the UI's minimal form does: username, full name, a
// password, its own primary group, and the API default smb = true. The HCL a
// user adopting it would write (no password: they don't manage it in TF).
func TestHuntExternal_userDefaults(t *testing.T) {
	acctest.PreCheck(t)
	name := acctest.RandName("tfhuntextu")
	id := apiCreate(t, "user.create", map[string]any{
		"username": name, "full_name": "tfhunt external", "group_create": true, "password": "TfHunt-Ext-Pw-1",
	}, "")
	t.Cleanup(func() { apiDelete("user.delete", mustInt(t, id), map[string]any{"delete_group": true}) })
	huntImportExternal(t, "truenas_user.u", fmt.Sprintf(`
resource "truenas_user" "u" {
  username  = %q
  full_name = "tfhunt external"
}
`, name), id)
}

// An NFS share made with nothing but a path.
func TestHuntExternal_nfsDefaults(t *testing.T) {
	acctest.PreCheck(t)
	ds := acctest.TestPool() + "/" + acctest.RandName("tfhuntextnfs")
	apiCreate(t, "pool.dataset.create", map[string]any{"name": ds}, "")
	t.Cleanup(func() { apiDelete("pool.dataset.delete", ds) })
	id := apiCreate(t, "sharing.nfs.create", map[string]any{"path": "/mnt/" + ds}, "")
	t.Cleanup(func() { apiDelete("sharing.nfs.delete", mustInt(t, id)) })
	huntImportExternal(t, "truenas_nfs_share.s", fmt.Sprintf(`
resource "truenas_nfs_share" "s" {
  path = %q
}
`, "/mnt/"+ds), id)
}

// An SMB share made with nothing but a name and a path.
func TestHuntExternal_smbDefaults(t *testing.T) {
	acctest.PreCheck(t)
	ds := acctest.TestPool() + "/" + acctest.RandName("tfhuntextsmb")
	apiCreate(t, "pool.dataset.create", map[string]any{"name": ds}, "")
	t.Cleanup(func() { apiDelete("pool.dataset.delete", ds) })
	share := acctest.RandName("tfhuntextsmb")
	id := apiCreate(t, "sharing.smb.create", map[string]any{"name": share, "path": "/mnt/" + ds}, "")
	t.Cleanup(func() { apiDelete("sharing.smb.delete", mustInt(t, id)) })
	huntImportExternal(t, "truenas_smb_share.s", fmt.Sprintf(`
resource "truenas_smb_share" "s" {
  name = %q
  path = %q
}
`, share, "/mnt/"+ds), id)
}

// A cron job made with the API minimum: command, user, schedule.
func TestHuntExternal_cronjobDefaults(t *testing.T) {
	acctest.PreCheck(t)
	desc := acctest.RandName("tfhuntextcron")
	sched := map[string]any{"minute": "0", "hour": "1", "dom": "*", "month": "*", "dow": "*"}
	id := apiCreate(t, "cronjob.create", map[string]any{
		"command": "echo " + desc, "user": "root", "description": desc, "schedule": sched,
	}, "")
	t.Cleanup(func() { apiDelete("cronjob.delete", mustInt(t, id)) })
	huntImportExternal(t, "truenas_cronjob.c", fmt.Sprintf(`
resource "truenas_cronjob" "c" {
  command     = %q
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
`, "echo "+desc, desc), id)
}

// A periodic snapshot task made with the API minimum.
func TestHuntExternal_periodicSnapshotDefaults(t *testing.T) {
	acctest.PreCheck(t)
	ds := acctest.TestPool() + "/" + acctest.RandName("tfhuntextps")
	apiCreate(t, "pool.dataset.create", map[string]any{"name": ds}, "")
	t.Cleanup(func() { apiDelete("pool.dataset.delete", ds) })
	sched := map[string]any{"minute": "0", "hour": "0", "dom": "*", "month": "*", "dow": "*"}
	id := apiCreate(t, "pool.snapshottask.create", map[string]any{
		"dataset": ds, "recursive": false, "lifetime_value": 1, "lifetime_unit": "WEEK",
		"naming_schema": "auto-%Y-%m-%d_%H-%M", "schedule": sched,
	}, "")
	t.Cleanup(func() { apiDelete("pool.snapshottask.delete", mustInt(t, id)) })
	huntImportExternal(t, "truenas_periodic_snapshot_task.p", fmt.Sprintf(`
resource "truenas_periodic_snapshot_task" "p" {
  dataset        = %q
  lifetime_value = 1
  lifetime_unit  = "WEEK"
  naming_schema  = "auto-%%Y-%%m-%%d_%%H-%%M"
  schedule = {
    minute = "0"
    hour   = "0"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}
`, ds), id)
}

// An iSCSI target made with nothing but a name (no portal/initiator groups).
func TestHuntExternal_iscsiTargetDefaults(t *testing.T) {
	acctest.PreCheck(t)
	name := acctest.RandName("tfhuntexttgt")
	id := apiCreate(t, "iscsi.target.create", map[string]any{"name": name}, "")
	t.Cleanup(func() { apiDelete("iscsi.target.delete", mustInt(t, id)) })
	huntImportExternal(t, "truenas_iscsi_target.t", fmt.Sprintf(`
resource "truenas_iscsi_target" "t" {
  name = %q
}
`, name), id)
}

func mustInt(t *testing.T, s string) int64 {
	t.Helper()
	var n int64
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		t.Fatalf("id %q is not an integer", s)
	}
	return n
}

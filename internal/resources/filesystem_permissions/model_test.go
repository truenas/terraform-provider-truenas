// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_permissions

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

// TestPermModeString verifies the full st_mode -> permission-bits-only
// octal string conversion, using the exact value observed probing
// filesystem.stat live after filesystem.setperm mode="0750" on a
// directory (0o40750 = decimal 16872).
func TestPermModeString(t *testing.T) {
	cases := []struct {
		stMode int64
		want   string
	}{
		{16872, "0750"}, // directory, 0750 (probed live)
		{16877, "0755"}, // directory, 0755 (probed live, default new dataset dir)
		{0o100644, "0644"},
		{0o40777, "0777"},
	}
	for _, c := range cases {
		if got := permModeString(c.stMode); got != c.want {
			t.Errorf("permModeString(%d) = %q, want %q", c.stMode, got, c.want)
		}
	}
}

func TestSetpermPayload_FullySet(t *testing.T) {
	m := &FilesystemPermissionsModel{
		Path:      types.StringValue("/mnt/tank/tf-acc-fsperm"),
		Mode:      normalize.NewOctalModeValue("0750"),
		UID:       types.Int64Value(1000),
		GID:       types.Int64Value(1000),
		Recursive: types.BoolValue(true),
		Traverse:  types.BoolValue(false),
	}

	p := m.setpermPayload(nil)

	if p["path"] != "/mnt/tank/tf-acc-fsperm" {
		t.Errorf("payload[path] = %v", p["path"])
	}
	if p["mode"] != "0750" {
		t.Errorf("payload[mode] = %v, want 0750", p["mode"])
	}
	if p["uid"] != int64(1000) {
		t.Errorf("payload[uid] = %v, want 1000", p["uid"])
	}
	if p["gid"] != int64(1000) {
		t.Errorf("payload[gid] = %v, want 1000", p["gid"])
	}
	opts, ok := p["options"].(map[string]any)
	if !ok {
		t.Fatalf("payload[options] is %T, want map[string]any", p["options"])
	}
	if opts["recursive"] != true {
		t.Errorf("options[recursive] = %v, want true", opts["recursive"])
	}
	if opts["traverse"] != false {
		t.Errorf("options[traverse] = %v, want false", opts["traverse"])
	}
}

// TestSetpermPayload_OmitsUnsetModeUidGid verifies mode/uid/gid are
// omitted entirely (not sent as explicit null) when unset, so
// filesystem.setperm's "leave unchanged" default applies rather than the
// provider clobbering an out-of-band value.
func TestSetpermPayload_OmitsUnsetModeUidGid(t *testing.T) {
	m := &FilesystemPermissionsModel{
		Path:      types.StringValue("/mnt/tank/tf-acc-fsperm"),
		Mode:      normalize.NewOctalModeNull(),
		UID:       types.Int64Null(),
		GID:       types.Int64Null(),
		Recursive: types.BoolNull(),
		Traverse:  types.BoolNull(),
	}

	p := m.setpermPayload(nil)

	if _, ok := p["mode"]; ok {
		t.Errorf("payload should omit mode when unset, got %v", p["mode"])
	}
	if _, ok := p["uid"]; ok {
		t.Errorf("payload should omit uid when unset, got %v", p["uid"])
	}
	if _, ok := p["gid"]; ok {
		t.Errorf("payload should omit gid when unset, got %v", p["gid"])
	}
	// options is still sent (with false defaults) even when recursive/
	// traverse are null in the model.
	opts, ok := p["options"].(map[string]any)
	if !ok {
		t.Fatalf("payload[options] is %T, want map[string]any", p["options"])
	}
	if opts["recursive"] != false || opts["traverse"] != false {
		t.Errorf("options = %v, want both false when unset", opts)
	}
}

func TestHasWrite(t *testing.T) {
	cases := []struct {
		name string
		m    *FilesystemPermissionsModel
		want bool
	}{
		{"all unset", &FilesystemPermissionsModel{Mode: normalize.NewOctalModeNull(), UID: types.Int64Null(), GID: types.Int64Null()}, false},
		{"mode set", &FilesystemPermissionsModel{Mode: normalize.NewOctalModeValue("0750"), UID: types.Int64Null(), GID: types.Int64Null()}, true},
		{"uid set", &FilesystemPermissionsModel{Mode: normalize.NewOctalModeNull(), UID: types.Int64Value(1000), GID: types.Int64Null()}, true},
		{"gid set", &FilesystemPermissionsModel{Mode: normalize.NewOctalModeNull(), UID: types.Int64Null(), GID: types.Int64Value(1000)}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.m.hasWrite(nil); got != c.want {
				t.Errorf("hasWrite() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestResponseToModel(t *testing.T) {
	api := &fsStatAPI{Mode: 16872, UID: 1000, GID: 1000}
	var m FilesystemPermissionsModel
	diags := responseToModel(api, "/mnt/tank/tf-acc-fsperm", &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostics errors: %v", diags)
	}

	if m.ID.ValueString() != "/mnt/tank/tf-acc-fsperm" {
		t.Errorf("ID = %v", m.ID)
	}
	if m.Path.ValueString() != "/mnt/tank/tf-acc-fsperm" {
		t.Errorf("Path = %v", m.Path)
	}
	if m.Mode.ValueString() != "0750" {
		t.Errorf("Mode = %v, want 0750", m.Mode)
	}
	if m.UID.ValueInt64() != 1000 {
		t.Errorf("UID = %v, want 1000", m.UID)
	}
	if m.GID.ValueInt64() != 1000 {
		t.Errorf("GID = %v, want 1000", m.GID)
	}
	// responseToModel must never touch Recursive/Traverse (apply-time-only,
	// not readable from the wire).
	if !m.Recursive.IsNull() {
		t.Errorf("Recursive = %v, want untouched (null)", m.Recursive)
	}
	if !m.Traverse.IsNull() {
		t.Errorf("Traverse = %v, want untouched (null)", m.Traverse)
	}
}

func TestResponseToDataSourceModel(t *testing.T) {
	api := &fsStatAPI{Mode: 16877, UID: 0, GID: 0}
	var m FilesystemPermissionsDataSourceModel
	diags := responseToDataSourceModel(api, "/mnt/tank", &m)
	if diags.HasError() {
		t.Fatalf("responseToDataSourceModel returned diagnostics errors: %v", diags)
	}
	if m.Mode.ValueString() != "0755" {
		t.Errorf("Mode = %v, want 0755", m.Mode)
	}
}

func TestDeleteWarningDiagnostics(t *testing.T) {
	diags := deleteWarningDiagnostics("/mnt/tank/tf-acc-fsperm")
	if diags.HasError() {
		t.Fatalf("deleteWarningDiagnostics returned an error-level diagnostic: %v", diags)
	}
	if len(diags) != 1 {
		t.Fatalf("expected exactly 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Severity().String() != "Warning" {
		t.Errorf("severity = %v, want Warning", diags[0].Severity())
	}
}

// TestSetpermPayload_SendsOnlyWhatChanged guards the issue #67 class:
// unconfigured or unchanged mode/uid/gid are carried into every update plan
// from state, and sending them back made a recursive setperm apply each one
// to every file under the path (a mode-only change re-owned the tree; an
// owner-only change gave every file the directory's mode).
func TestSetpermPayload_SendsOnlyWhatChanged(t *testing.T) {
	prior := &FilesystemPermissionsModel{
		Mode: normalize.NewOctalModeValue("0755"), UID: types.Int64Value(0), GID: types.Int64Value(0),
	}
	mk := func(mode string, uid, gid int64) *FilesystemPermissionsModel {
		return &FilesystemPermissionsModel{
			Path: types.StringValue("/mnt/tank/x"), Mode: normalize.NewOctalModeValue(mode),
			UID: types.Int64Value(uid), GID: types.Int64Value(gid), Recursive: types.BoolValue(true),
		}
	}
	keys := func(p map[string]any) string {
		out := ""
		for _, k := range []string{"mode", "uid", "gid"} {
			if _, ok := p[k]; ok {
				out += k + " "
			}
		}
		return out
	}
	cases := []struct {
		name string
		m    *FilesystemPermissionsModel
		want string
	}{
		{"nothing changed", mk("0755", 0, 0), ""},
		{"same mode, other spelling", mk("755", 0, 0), ""},
		{"mode only", mk("0750", 0, 0), "mode "},
		{"uid only", mk("0755", 1000, 0), "uid "},
		{"gid only", mk("0755", 0, 1000), "gid "},
		{"all", mk("0700", 1000, 1000), "mode uid gid "},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := keys(c.m.setpermPayload(prior)); got != c.want {
				t.Errorf("payload carries %q, want %q", got, c.want)
			}
			if got, want := c.m.hasWrite(prior), c.want != ""; got != want {
				t.Errorf("hasWrite = %v, want %v", got, want)
			}
		})
	}
	// Create (no prior): every configured value is sent.
	if got := keys(mk("0750", 1000, 1000).setpermPayload(nil)); got != "mode uid gid " {
		t.Errorf("create payload carries %q", got)
	}
}

// TestWriteCall: setperm needs a mode, so an owner-only change is a chown.
func TestWriteCall(t *testing.T) {
	prior := &FilesystemPermissionsModel{
		Mode: normalize.NewOctalModeValue("0755"), UID: types.Int64Value(0), GID: types.Int64Value(0),
	}
	mk := func(mode string, uid, gid int64) *FilesystemPermissionsModel {
		return &FilesystemPermissionsModel{Path: types.StringValue("/mnt/tank/x"), Mode: normalize.NewOctalModeValue(mode),
			UID: types.Int64Value(uid), GID: types.Int64Value(gid), Recursive: types.BoolValue(true)}
	}
	for name, c := range map[string]struct {
		m    *FilesystemPermissionsModel
		want string
	}{
		"nothing":      {mk("0755", 0, 0), ""},
		"mode":         {mk("0750", 0, 0), "filesystem.setperm"},
		"mode and uid": {mk("0750", 1, 0), "filesystem.setperm"},
		"uid only":     {mk("0755", 1, 0), "filesystem.chown"},
		"gid only":     {mk("0755", 0, 1), "filesystem.chown"},
	} {
		method, p := c.m.writeCall(prior)
		if method != c.want {
			t.Errorf("%s: method %q, want %q", name, method, c.want)
		}
		if method == "filesystem.chown" {
			if _, ok := p["mode"]; ok {
				t.Errorf("%s: chown payload carries a mode", name)
			}
			if opts, _ := p["options"].(map[string]any); opts["recursive"] != true {
				t.Errorf("%s: chown payload lost options.recursive", name)
			}
		}
	}
	// Create with only an owner configured (mode unknown): chown, not a
	// setperm the server would reject.
	m := &FilesystemPermissionsModel{Path: types.StringValue("/x"), Mode: normalize.NewOctalModeUnknown(),
		UID: types.Int64Value(1), GID: types.Int64Unknown()}
	if method, _ := m.writeCall(nil); method != "filesystem.chown" {
		t.Errorf("owner-only create: method %q, want filesystem.chown", method)
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_acl

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

const nfs4OwnerGroupACL = `[{"flags":{"BASIC":"INHERIT"},"perms":{"BASIC":"FULL_CONTROL"},"tag":"owner@","type":"ALLOW"},` +
	`{"flags":{"BASIC":"INHERIT"},"perms":{"BASIC":"MODIFY"},"tag":"group@","type":"ALLOW"}]`

const posix1eUserMaskACL = `[{"default":false,"id":1000,"perms":{"EXECUTE":false,"READ":true,"WRITE":false},"tag":"USER"},` +
	`{"default":false,"perms":{"EXECUTE":true,"READ":true,"WRITE":true},"tag":"MASK"}]`

func TestSetaclPayload_FullySet(t *testing.T) {
	m := &FilesystemAclModel{
		Path:      types.StringValue("/mnt/tank/tf-acc"),
		Entries:   types.StringValue(nfs4OwnerGroupACL),
		UID:       types.Int64Value(1000),
		GID:       types.Int64Value(1000),
		Recursive: types.BoolValue(true),
		Traverse:  types.BoolValue(true),
	}

	p := m.setaclPayload()

	if p["path"] != "/mnt/tank/tf-acc" {
		t.Errorf("payload[path] = %v, want /mnt/tank/tf-acc", p["path"])
	}
	rawDacl, ok := p["dacl"].(json.RawMessage)
	if !ok {
		t.Fatalf("payload[dacl] is %T, want json.RawMessage", p["dacl"])
	}
	if string(rawDacl) != nfs4OwnerGroupACL {
		t.Errorf("payload[dacl] = %s, want %s", rawDacl, nfs4OwnerGroupACL)
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
	if opts["traverse"] != true {
		t.Errorf("options[traverse] = %v, want true", opts["traverse"])
	}
	if opts["stripacl"] != false {
		t.Errorf("options[stripacl] = %v, want false (Create/Update never strips)", opts["stripacl"])
	}
}

func TestSetaclPayload_UIDGIDOmittedWhenUnset(t *testing.T) {
	m := &FilesystemAclModel{
		Path:    types.StringValue("/mnt/tank/tf-acc"),
		Entries: types.StringValue(nfs4OwnerGroupACL),
		UID:     types.Int64Null(),
		GID:     types.Int64Null(),
	}

	p := m.setaclPayload()

	if _, ok := p["uid"]; ok {
		t.Errorf("payload should omit uid when unset, got %v", p["uid"])
	}
	if _, ok := p["gid"]; ok {
		t.Errorf("payload should omit gid when unset, got %v", p["gid"])
	}
	opts := p["options"].(map[string]any)
	if opts["recursive"] != false {
		t.Errorf("options[recursive] = %v, want false when unset", opts["recursive"])
	}
	if opts["traverse"] != false {
		t.Errorf("options[traverse] = %v, want false when unset", opts["traverse"])
	}
}

// TestDeleteACLPayload verifies the exact stripacl=true payload shape
// (probed live, confirmed a clean strip on both NFS4 and POSIX1E).
func TestDeleteACLPayload(t *testing.T) {
	p := deleteACLPayload("/mnt/tank/tf-acc")

	if p["path"] != "/mnt/tank/tf-acc" {
		t.Errorf("payload[path] = %v, want /mnt/tank/tf-acc", p["path"])
	}
	dacl, ok := p["dacl"].([]any)
	if !ok || len(dacl) != 0 {
		t.Errorf("payload[dacl] = %v (%T), want empty []any", p["dacl"], p["dacl"])
	}
	opts, ok := p["options"].(map[string]any)
	if !ok {
		t.Fatalf("payload[options] is %T, want map[string]any", p["options"])
	}
	if opts["stripacl"] != true {
		t.Errorf("options[stripacl] = %v, want true", opts["stripacl"])
	}
	for _, k := range []string{"uid", "gid", "recursive", "traverse"} {
		if _, ok := opts[k]; ok {
			t.Errorf("delete payload options should not set %q", k)
		}
		if _, ok := p[k]; ok {
			t.Errorf("delete payload should not set top-level %q", k)
		}
	}
}

// TestEntriesNormalized_StripsIDSentinels verifies both observed "no id"
// sentinels (null and -1, probed live via filesystem.getacl) are dropped,
// and a real USER/GROUP id (e.g. 1000) is preserved.
func TestEntriesNormalized_StripsIDSentinels(t *testing.T) {
	raw := json.RawMessage(`[
		{"tag":"owner@","type":"ALLOW","perms":{"BASIC":"FULL_CONTROL"},"flags":{"BASIC":"INHERIT"},"id":null,"who":null},
		{"tag":"owner@","type":"ALLOW","perms":{"BASIC":"FULL_CONTROL"},"flags":{"BASIC":"INHERIT"},"id":-1,"who":null},
		{"tag":"USER","type":"ALLOW","perms":{"BASIC":"READ"},"flags":{"BASIC":"INHERIT"},"id":1000,"who":null}
	]`)

	entries, err := entriesNormalized(raw)
	if err != nil {
		t.Fatalf("entriesNormalized: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}
	for i, e := range entries[:2] {
		if _, ok := e["id"]; ok {
			t.Errorf("entry %d: id should be stripped, got %v", i, e["id"])
		}
		if _, ok := e["who"]; ok {
			t.Errorf("entry %d: who should be stripped, got %v", i, e["who"])
		}
	}
	if entries[2]["id"] != float64(1000) {
		t.Errorf("entry 2: id = %v, want 1000 (real uid must be preserved)", entries[2]["id"])
	}
}

// TestEntriesDrifted_NullVsMinusOneNotDrift verifies the two id sentinels
// don't trigger drift against each other, matching the exact quirk probed
// live on filesystem.getacl.
func TestEntriesDrifted_NullVsMinusOneNotDrift(t *testing.T) {
	stateEntries := `[{"tag":"owner@","type":"ALLOW","perms":{"BASIC":"FULL_CONTROL"},"flags":{"BASIC":"INHERIT"}}]`
	apiACL := json.RawMessage(`[{"tag":"owner@","type":"ALLOW","perms":{"BASIC":"FULL_CONTROL"},"flags":{"BASIC":"INHERIT"},"id":-1,"who":null}]`)

	drifted, diags := entriesDrifted(stateEntries, apiACL)
	if diags.HasError() {
		t.Fatalf("entriesDrifted returned diagnostics errors: %v", diags)
	}
	if drifted {
		t.Error("entriesDrifted = true, want false: id:-1/who:null vs. omitted should be equivalent")
	}
}

// TestEntriesDrifted_RealChangeIsDrift verifies an actual permission change
// is still detected as drift.
func TestEntriesDrifted_RealChangeIsDrift(t *testing.T) {
	stateEntries := `[{"tag":"owner@","type":"ALLOW","perms":{"BASIC":"FULL_CONTROL"},"flags":{"BASIC":"INHERIT"}}]`
	apiACL := json.RawMessage(`[{"tag":"owner@","type":"ALLOW","perms":{"BASIC":"MODIFY"},"flags":{"BASIC":"INHERIT"},"id":-1,"who":null}]`)

	drifted, diags := entriesDrifted(stateEntries, apiACL)
	if diags.HasError() {
		t.Fatalf("entriesDrifted returned diagnostics errors: %v", diags)
	}
	if !drifted {
		t.Error("entriesDrifted = false, want true: FULL_CONTROL vs MODIFY is a real change")
	}
}

// TestEntriesNormalized_PosixMaskRequired is a documentation-level unit
// test (POSIX1E is not live-testable on this provider's probe tank/NFS4
// fixture path per the task brief - see schema.go) covering the wire
// quirk discovered live: a POSIX1E ACL with a named USER/GROUP entry
// requires a MASK entry. entriesNormalized itself doesn't enforce this
// (server-side validation, like acl_template's tradeoff), but must still
// round-trip POSIX1E's "default" field and id/who sentinels correctly.
func TestEntriesNormalized_PosixMaskRequired(t *testing.T) {
	entries, err := entriesNormalized(json.RawMessage(posix1eUserMaskACL))
	if err != nil {
		t.Fatalf("entriesNormalized: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0]["tag"] != "USER" {
		t.Errorf("entries[0][tag] = %v, want USER", entries[0]["tag"])
	}
	if entries[0]["id"] != float64(1000) {
		t.Errorf("entries[0][id] = %v, want 1000", entries[0]["id"])
	}
	if entries[0]["default"] != false {
		t.Errorf("entries[0][default] = %v, want false", entries[0]["default"])
	}
	if entries[1]["tag"] != "MASK" {
		t.Errorf("entries[1][tag] = %v, want MASK", entries[1]["tag"])
	}
}

func TestCanonicalEntriesJSON_InvalidJSON(t *testing.T) {
	_, diags := canonicalEntriesJSON(json.RawMessage(`not json`))
	if !diags.HasError() {
		t.Error("canonicalEntriesJSON should return an error diagnostic for invalid JSON")
	}
}

func TestResponseToModel(t *testing.T) {
	api := &fsGetAclAPI{
		Path:    "/mnt/tank/tf-acc",
		UID:     1000,
		GID:     1000,
		ACLType: "NFS4",
		ACL:     json.RawMessage(nfs4OwnerGroupACL),
	}
	var m FilesystemAclModel
	diags := responseToModel(api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostics errors: %v", diags)
	}

	if m.ID.ValueString() != "/mnt/tank/tf-acc" {
		t.Errorf("ID = %v, want /mnt/tank/tf-acc", m.ID)
	}
	if m.Path.ValueString() != "/mnt/tank/tf-acc" {
		t.Errorf("Path = %v, want /mnt/tank/tf-acc", m.Path)
	}
	if m.ACLType.ValueString() != "NFS4" {
		t.Errorf("ACLType = %v, want NFS4", m.ACLType)
	}
	if m.UID.ValueInt64() != 1000 {
		t.Errorf("UID = %v, want 1000", m.UID)
	}
	if m.GID.ValueInt64() != 1000 {
		t.Errorf("GID = %v, want 1000", m.GID)
	}
	// responseToModel must not touch Entries (write-what-you-said).
	if !m.Entries.IsNull() {
		t.Errorf("Entries = %v, want untouched (null)", m.Entries)
	}
}

func TestResponseToDataSourceModel(t *testing.T) {
	api := &fsGetAclAPI{
		Path:    "/mnt/tank/tf-acc",
		UID:     0,
		GID:     0,
		ACLType: "POSIX1E",
		ACL:     json.RawMessage(posix1eUserMaskACL),
	}
	var m FilesystemAclDataSourceModel
	diags := responseToDataSourceModel(api, &m)
	if diags.HasError() {
		t.Fatalf("responseToDataSourceModel returned diagnostics errors: %v", diags)
	}
	if m.Entries.IsNull() {
		t.Error("Entries should be set directly from the API response for the datasource")
	}
	if m.ACLType.ValueString() != "POSIX1E" {
		t.Errorf("ACLType = %v, want POSIX1E", m.ACLType)
	}
}

func TestDeleteWarningDiagnostics(t *testing.T) {
	diags := deleteWarningDiagnostics("/mnt/tank/tf-acc")
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1", len(diags))
	}
	if diags[0].Severity().String() != "Warning" {
		t.Errorf("severity = %v, want Warning", diags[0].Severity())
	}
}

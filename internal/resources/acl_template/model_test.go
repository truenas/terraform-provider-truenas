// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acl_template

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

const nfs4OwnerOnlyACL = `[{"tag":"owner@","type":"ALLOW","perms":{"BASIC":"FULL_CONTROL"},"flags":{"BASIC":"INHERIT"}}]`

func TestApiPayload_FullySet(t *testing.T) {
	m := &AclTemplateModel{
		Name:    types.StringValue("tf-acl-template"),
		ACLType: types.StringValue("NFS4"),
		ACL:     types.StringValue(nfs4OwnerOnlyACL),
		Comment: types.StringValue("a comment"),
	}

	p, diags := m.apiPayload()
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}

	if p["name"] != "tf-acl-template" {
		t.Errorf("payload[name] = %v, want tf-acl-template", p["name"])
	}
	if p["acltype"] != "NFS4" {
		t.Errorf("payload[acltype] = %v, want NFS4", p["acltype"])
	}
	if p["comment"] != "a comment" {
		t.Errorf("payload[comment] = %v, want %q", p["comment"], "a comment")
	}
	rawACL, ok := p["acl"].(json.RawMessage)
	if !ok {
		t.Fatalf("payload[acl] is %T, want json.RawMessage", p["acl"])
	}
	if string(rawACL) != nfs4OwnerOnlyACL {
		t.Errorf("payload[acl] = %s, want %s", rawACL, nfs4OwnerOnlyACL)
	}
}

func TestApiPayload_CommentOmittedWhenUnset(t *testing.T) {
	m := &AclTemplateModel{
		Name:    types.StringValue("tf-acl-template"),
		ACLType: types.StringValue("NFS4"),
		ACL:     types.StringValue(nfs4OwnerOnlyACL),
		Comment: types.StringNull(),
	}

	p, diags := m.apiPayload()
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}
	if _, ok := p["comment"]; ok {
		t.Errorf("payload should omit comment when unset, got %v", p["comment"])
	}
}

// TestAclEntriesNormalized_StripsIDSentinels verifies both observed "no id"
// sentinels (null and -1, probed live) are dropped, and a real USER/GROUP
// id (e.g. 544) is preserved.
func TestAclEntriesNormalized_StripsIDSentinels(t *testing.T) {
	raw := json.RawMessage(`[
		{"tag":"owner@","type":"ALLOW","perms":{"BASIC":"FULL_CONTROL"},"flags":{"BASIC":"INHERIT"},"id":null,"who":null},
		{"tag":"owner@","type":"ALLOW","perms":{"BASIC":"FULL_CONTROL"},"flags":{"BASIC":"INHERIT"},"id":-1,"who":null},
		{"tag":"GROUP","type":"ALLOW","perms":{"BASIC":"FULL_CONTROL"},"flags":{"BASIC":"INHERIT"},"id":544,"who":null}
	]`)

	entries, err := aclEntriesNormalized(raw)
	if err != nil {
		t.Fatalf("aclEntriesNormalized: %v", err)
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
	if entries[2]["id"] != float64(544) {
		t.Errorf("entry 2: id = %v, want 544 (real GID must be preserved)", entries[2]["id"])
	}
}

// TestAclDrifted_NullVsMinusOneNotDrift verifies the two id sentinels don't
// trigger drift against each other (this is the exact null-vs-null-become-
// -1 quirk observed probing filesystem.acltemplate.create/get_instance
// live).
func TestAclDrifted_NullVsMinusOneNotDrift(t *testing.T) {
	stateACL := `[{"tag":"owner@","type":"ALLOW","perms":{"BASIC":"FULL_CONTROL"},"flags":{"BASIC":"INHERIT"}}]`
	apiACL := json.RawMessage(`[{"tag":"owner@","type":"ALLOW","perms":{"BASIC":"FULL_CONTROL"},"flags":{"BASIC":"INHERIT"},"id":-1,"who":null}]`)

	drifted, diags := aclDrifted(stateACL, apiACL)
	if diags.HasError() {
		t.Fatalf("aclDrifted returned diagnostics errors: %v", diags)
	}
	if drifted {
		t.Error("aclDrifted = true, want false: id:-1/who:null vs. omitted should be equivalent")
	}
}

// TestAclDrifted_RealChangeIsDrift verifies an actual permission change is
// still detected as drift.
func TestAclDrifted_RealChangeIsDrift(t *testing.T) {
	stateACL := `[{"tag":"owner@","type":"ALLOW","perms":{"BASIC":"FULL_CONTROL"},"flags":{"BASIC":"INHERIT"}}]`
	apiACL := json.RawMessage(`[{"tag":"owner@","type":"ALLOW","perms":{"BASIC":"MODIFY"},"flags":{"BASIC":"INHERIT"},"id":-1,"who":null}]`)

	drifted, diags := aclDrifted(stateACL, apiACL)
	if diags.HasError() {
		t.Fatalf("aclDrifted returned diagnostics errors: %v", diags)
	}
	if !drifted {
		t.Error("aclDrifted = false, want true: FULL_CONTROL vs MODIFY is a real change")
	}
}

func TestResponseToModel(t *testing.T) {
	api := &aclTemplateAPI{
		ID:      10,
		Builtin: false,
		Name:    "tf-acl-template",
		ACLType: "NFS4",
		ACL:     json.RawMessage(nfs4OwnerOnlyACL),
		Comment: "tf probe",
	}
	var m AclTemplateModel
	diags := responseToModel(api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostics errors: %v", diags)
	}

	if m.ID.ValueInt64() != 10 {
		t.Errorf("ID = %v, want 10", m.ID)
	}
	if m.Name.ValueString() != "tf-acl-template" {
		t.Errorf("Name = %v, want tf-acl-template", m.Name)
	}
	if m.ACLType.ValueString() != "NFS4" {
		t.Errorf("ACLType = %v, want NFS4", m.ACLType)
	}
	if m.Comment.ValueString() != "tf probe" {
		t.Errorf("Comment = %v, want %q", m.Comment, "tf probe")
	}
	if m.Builtin.ValueBool() != false {
		t.Errorf("Builtin = %v, want false", m.Builtin)
	}
	// responseToModel must not touch ACL (write-what-you-said).
	if !m.ACL.IsNull() {
		t.Errorf("ACL = %v, want untouched (null)", m.ACL)
	}
}

func TestResponseToDataSourceModel(t *testing.T) {
	api := &aclTemplateAPI{
		ID:      1,
		Builtin: true,
		Name:    "NFS4_OPEN",
		ACLType: "NFS4",
		ACL:     json.RawMessage(nfs4OwnerOnlyACL),
		Comment: "builtin template",
	}
	var m AclTemplateDataSourceModel
	diags := responseToDataSourceModel(api, &m)
	if diags.HasError() {
		t.Fatalf("responseToDataSourceModel returned diagnostics errors: %v", diags)
	}
	if m.ACL.IsNull() {
		t.Error("ACL should be set directly from the API response for the datasource")
	}
	if m.Builtin.ValueBool() != true {
		t.Errorf("Builtin = %v, want true", m.Builtin)
	}
}

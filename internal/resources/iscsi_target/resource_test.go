// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_target

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// TestISCSITargetSchema verifies that the resource schema has the expected
// attributes with the correct types.
func TestISCSITargetSchema(t *testing.T) {
	s := resourceSchema()

	// id must be Int64Attribute and Computed.
	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	idInt64, ok := idAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.Int64Attribute", idAttr)
	}
	if !idInt64.IsComputed() {
		t.Error("'id' should be Computed")
	}
	if idInt64.IsOptional() || idInt64.IsRequired() {
		t.Error("'id' should be Computed-only (not Optional/Required)")
	}

	// name must be StringAttribute and Required.
	nameAttr, ok := s.Attributes["name"]
	if !ok {
		t.Fatal("schema missing 'name' attribute")
	}
	nameStr, ok := nameAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'name' attribute is %T, want schema.StringAttribute", nameAttr)
	}
	if !nameStr.IsRequired() {
		t.Error("'name' should be Required")
	}

	// groups must be ListNestedAttribute.
	groupsAttr, ok := s.Attributes["groups"]
	if !ok {
		t.Fatal("schema missing 'groups' attribute")
	}
	if _, ok := groupsAttr.(schema.ListNestedAttribute); !ok {
		t.Errorf("'groups' attribute is %T, want schema.ListNestedAttribute", groupsAttr)
	}

	// auth_networks must be ListAttribute.
	anAttr, ok := s.Attributes["auth_networks"]
	if !ok {
		t.Fatal("schema missing 'auth_networks' attribute")
	}
	if _, ok := anAttr.(schema.ListAttribute); !ok {
		t.Errorf("'auth_networks' attribute is %T, want schema.ListAttribute", anAttr)
	}

	// rel_tgt_id must be Computed-only.
	relAttr, ok := s.Attributes["rel_tgt_id"]
	if !ok {
		t.Fatal("schema missing 'rel_tgt_id' attribute")
	}
	relInt64, ok := relAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'rel_tgt_id' attribute is %T, want schema.Int64Attribute", relAttr)
	}
	if !relInt64.IsComputed() {
		t.Error("'rel_tgt_id' should be Computed")
	}
	if relInt64.IsOptional() || relInt64.IsRequired() {
		t.Error("'rel_tgt_id' should be Computed-only (not Optional/Required)")
	}
}

// TestISCSITargetApiPayload verifies that apiPayload produces a map with the
// expected keys and correct values, including groups encoding.
func TestISCSITargetApiPayload(t *testing.T) {
	ctx := context.Background()

	groupObj, diags := types.ObjectValue(groupsAttrTypes, map[string]attr.Value{
		"portal":     types.Int64Value(1),
		"initiator":  types.Int64Value(2),
		"auth":       types.Int64Value(0),
		"authmethod": types.StringValue("CHAP"),
	})
	if diags.HasError() {
		t.Fatalf("ObjectValue failed: %v", diags)
	}

	groupList, diags := types.ListValue(
		types.ObjectType{AttrTypes: groupsAttrTypes},
		[]attr.Value{groupObj},
	)
	if diags.HasError() {
		t.Fatalf("ListValue failed: %v", diags)
	}

	authNetworks, diags := types.ListValueFrom(ctx, types.StringType, []string{"10.0.0.0/8"})
	if diags.HasError() {
		t.Fatalf("ListValueFrom (auth_networks) failed: %v", diags)
	}

	m := ISCSITargetModel{
		ID:           types.Int64Value(0),
		Name:         types.StringValue("iqn.2023-01.com.example:target1"),
		Alias:        types.StringValue("my-target"),
		Mode:         types.StringValue("ISCSI"),
		Groups:       groupList,
		AuthNetworks: authNetworks,
		RelTgtID:     types.Int64Value(0),
	}

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	expectedKeys := []string{"name", "alias", "mode", "groups", "auth_networks"}
	if len(payload) != len(expectedKeys) {
		t.Errorf("payload has %d keys, want %d", len(payload), len(expectedKeys))
	}
	for _, k := range expectedKeys {
		if _, ok := payload[k]; !ok {
			t.Errorf("payload missing key %q", k)
		}
	}

	// id and rel_tgt_id must NOT appear in payload.
	for _, forbidden := range []string{"id", "rel_tgt_id"} {
		if _, ok := payload[forbidden]; ok {
			t.Errorf("payload should not contain key %q", forbidden)
		}
	}

	if payload["name"] != "iqn.2023-01.com.example:target1" {
		t.Errorf("payload[name] = %v, want iqn.2023-01.com.example:target1", payload["name"])
	}
	if payload["mode"] != "ISCSI" {
		t.Errorf("payload[mode] = %v, want ISCSI", payload["mode"])
	}

	// Verify groups encoding.
	groups, ok := payload["groups"].([]map[string]any)
	if !ok {
		t.Fatalf("payload[groups] is %T, want []map[string]any", payload["groups"])
	}
	if len(groups) != 1 {
		t.Fatalf("payload[groups] has %d items, want 1", len(groups))
	}
	g := groups[0]
	if g["portal"] != int64(1) {
		t.Errorf("groups[0][portal] = %v, want 1", g["portal"])
	}
	if g["initiator"] != int64(2) {
		t.Errorf("groups[0][initiator] = %v, want 2", g["initiator"])
	}
	// auth is 0, so it should be nil.
	if g["auth"] != nil {
		t.Errorf("groups[0][auth] = %v, want nil (because auth==0)", g["auth"])
	}
	if g["authmethod"] != "CHAP" {
		t.Errorf("groups[0][authmethod] = %v, want CHAP", g["authmethod"])
	}

	// Verify auth_networks.
	an, ok := payload["auth_networks"].([]string)
	if !ok {
		t.Fatalf("payload[auth_networks] is %T, want []string", payload["auth_networks"])
	}
	if len(an) != 1 || an[0] != "10.0.0.0/8" {
		t.Errorf("payload[auth_networks] = %v, want [10.0.0.0/8]", an)
	}
}

// TestISCSITargetApiPayload_NullLists verifies that null groups/auth_networks
// default to empty slices (not nil).
func TestISCSITargetApiPayload_NullLists(t *testing.T) {
	ctx := context.Background()

	m := ISCSITargetModel{
		ID:           types.Int64Value(0),
		Name:         types.StringValue("target"),
		Alias:        types.StringValue(""),
		Mode:         types.StringValue("ISCSI"),
		Groups:       types.ListNull(types.ObjectType{AttrTypes: groupsAttrTypes}),
		AuthNetworks: types.ListNull(types.StringType),
		RelTgtID:     types.Int64Value(0),
	}

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	groups, ok := payload["groups"].([]map[string]any)
	if !ok {
		t.Fatalf("payload[groups] is %T, want []map[string]any", payload["groups"])
	}
	if groups == nil {
		t.Error("payload[groups] must not be nil, want empty slice")
	}
	if len(groups) != 0 {
		t.Errorf("payload[groups] has %d items, want 0", len(groups))
	}

	an, ok := payload["auth_networks"].([]string)
	if !ok {
		t.Fatalf("payload[auth_networks] is %T, want []string", payload["auth_networks"])
	}
	if an == nil {
		t.Error("payload[auth_networks] must not be nil, want empty slice")
	}
}

// TestISCSITargetResponseToModel verifies that responseToModel populates all
// fields correctly, including nil pointer handling for alias/initiator/auth.
func TestISCSITargetResponseToModel(t *testing.T) {
	ctx := context.Background()

	initiator := int64(3)
	api := &targetAPI{
		ID:    42,
		Name:  "iqn.2023-01.com.example:target1",
		Alias: nil, // should become ""
		Mode:  "ISCSI",
		Groups: []targetGroupAPI{
			{Portal: 1, Initiator: &initiator, Auth: nil, AuthMethod: "CHAP"},
		},
		AuthNetworks: []string{"192.168.0.0/16"},
		RelTgtID:     7,
	}

	var m ISCSITargetModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if m.ID.ValueInt64() != 42 {
		t.Errorf("ID = %v, want 42", m.ID.ValueInt64())
	}
	if m.Name.ValueString() != "iqn.2023-01.com.example:target1" {
		t.Errorf("Name = %v", m.Name.ValueString())
	}
	if m.Alias.ValueString() != "" {
		t.Errorf("Alias = %q, want empty string for nil alias", m.Alias.ValueString())
	}
	if m.Mode.ValueString() != "ISCSI" {
		t.Errorf("Mode = %v, want ISCSI", m.Mode.ValueString())
	}
	if m.RelTgtID.ValueInt64() != 7 {
		t.Errorf("RelTgtID = %v, want 7", m.RelTgtID.ValueInt64())
	}

	var groupItems []TargetGroupModel
	if diags := m.Groups.ElementsAs(ctx, &groupItems, false); diags.HasError() {
		t.Fatalf("Groups.ElementsAs failed: %v", diags)
	}
	if len(groupItems) != 1 {
		t.Fatalf("Groups has %d items, want 1", len(groupItems))
	}
	g := groupItems[0]
	if g.Portal.ValueInt64() != 1 {
		t.Errorf("Groups[0].Portal = %v, want 1", g.Portal.ValueInt64())
	}
	if g.Initiator.ValueInt64() != 3 {
		t.Errorf("Groups[0].Initiator = %v, want 3", g.Initiator.ValueInt64())
	}
	if g.Auth.ValueInt64() != 0 {
		t.Errorf("Groups[0].Auth = %v, want 0 (nil pointer)", g.Auth.ValueInt64())
	}
	if g.AuthMethod.ValueString() != "CHAP" {
		t.Errorf("Groups[0].AuthMethod = %v, want CHAP", g.AuthMethod.ValueString())
	}

	var an []string
	if diags := m.AuthNetworks.ElementsAs(ctx, &an, false); diags.HasError() {
		t.Fatalf("AuthNetworks.ElementsAs failed: %v", diags)
	}
	if len(an) != 1 || an[0] != "192.168.0.0/16" {
		t.Errorf("AuthNetworks = %v, want [192.168.0.0/16]", an)
	}
}

// TestISCSITargetApiPayload_IscsiParameters verifies the nested
// iscsi_parameters block maps to the API's {"QueuedCommands": N} shape when
// set, and is omitted when null.
func TestISCSITargetApiPayload_IscsiParameters(t *testing.T) {
	ctx := context.Background()

	base := func(params types.Object) *ISCSITargetModel {
		return &ISCSITargetModel{
			Name:            types.StringValue("tgt"),
			Alias:           types.StringNull(),
			Mode:            types.StringNull(),
			Groups:          types.ListNull(types.ObjectType{AttrTypes: groupsAttrTypes}),
			AuthNetworks:    types.ListNull(types.StringType),
			IscsiParameters: params,
		}
	}

	obj, d := types.ObjectValueFrom(ctx, iscsiParamsAttrTypes, IscsiParametersModel{QueuedCommands: types.Int64Value(128)})
	if d.HasError() {
		t.Fatalf("build object: %v", d)
	}
	p, pd := base(obj).apiPayload(ctx)
	if pd.HasError() {
		t.Fatalf("apiPayload: %v", pd)
	}
	inner, ok := p["iscsi_parameters"].(map[string]any)
	if !ok {
		t.Fatalf("iscsi_parameters is %T, want map[string]any", p["iscsi_parameters"])
	}
	if inner["QueuedCommands"] != int64(128) {
		t.Errorf("QueuedCommands = %v, want 128", inner["QueuedCommands"])
	}

	// null object → key omitted entirely.
	p, _ = base(types.ObjectNull(iscsiParamsAttrTypes)).apiPayload(ctx)
	if _, ok := p["iscsi_parameters"]; ok {
		t.Errorf("null iscsi_parameters should be omitted, got %v", p["iscsi_parameters"])
	}
}

// TestISCSITargetResponseToModel_IscsiParameters verifies the nested block
// decodes: nil API value → null object; populated → object with queued_commands.
func TestISCSITargetResponseToModel_IscsiParameters(t *testing.T) {
	ctx := context.Background()

	var m ISCSITargetModel
	apiNil := &targetAPI{ID: 1, Name: "t", Mode: "ISCSI"}
	if d := responseToModel(ctx, apiNil, &m); d.HasError() {
		t.Fatalf("responseToModel nil: %v", d)
	}
	if !m.IscsiParameters.IsNull() {
		t.Error("iscsi_parameters should be null when API omits it")
	}

	qc := int64(32)
	apiSet := &targetAPI{ID: 2, Name: "t2", Mode: "ISCSI"}
	apiSet.IscsiParameters = &struct {
		QueuedCommands *int64 `json:"QueuedCommands"`
	}{QueuedCommands: &qc}
	var m2 ISCSITargetModel
	if d := responseToModel(ctx, apiSet, &m2); d.HasError() {
		t.Fatalf("responseToModel set: %v", d)
	}
	if m2.IscsiParameters.IsNull() {
		t.Fatal("iscsi_parameters should be set")
	}
	var params IscsiParametersModel
	if d := m2.IscsiParameters.As(ctx, &params, basetypes.ObjectAsOptions{}); d.HasError() {
		t.Fatalf("As: %v", d)
	}
	if params.QueuedCommands.ValueInt64() != 32 {
		t.Errorf("queued_commands = %d, want 32", params.QueuedCommands.ValueInt64())
	}
}

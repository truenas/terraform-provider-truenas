// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_portal

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestISCSIPortalSchema verifies that the resource schema has the expected
// attributes with the correct types.
func TestISCSIPortalSchema(t *testing.T) {
	s := resourceSchema()

	// id must be Int64Attribute and Computed-only.
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

	// comment must be Optional+Computed.
	commentAttr, ok := s.Attributes["comment"]
	if !ok {
		t.Fatal("schema missing 'comment' attribute")
	}
	commentStr, ok := commentAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'comment' attribute is %T, want schema.StringAttribute", commentAttr)
	}
	if !commentStr.IsOptional() || !commentStr.IsComputed() {
		t.Error("'comment' should be Optional and Computed")
	}

	// listen must be Required ListNestedAttribute.
	listenAttr, ok := s.Attributes["listen"]
	if !ok {
		t.Fatal("schema missing 'listen' attribute")
	}
	listenNested, ok := listenAttr.(schema.ListNestedAttribute)
	if !ok {
		t.Fatalf("'listen' attribute is %T, want schema.ListNestedAttribute", listenAttr)
	}
	if !listenNested.IsRequired() {
		t.Error("'listen' should be Required")
	}

	ipAttr, ok := listenNested.NestedObject.Attributes["ip"]
	if !ok {
		t.Fatal("'listen' nested object missing 'ip' attribute")
	}
	ipStr, ok := ipAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'listen.ip' attribute is %T, want schema.StringAttribute", ipAttr)
	}
	if !ipStr.IsRequired() {
		t.Error("'listen.ip' should be Required")
	}

	portAttr, ok := listenNested.NestedObject.Attributes["port"]
	if !ok {
		t.Fatal("'listen' nested object missing 'port' attribute")
	}
	portInt64, ok := portAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'listen.port' attribute is %T, want schema.Int64Attribute", portAttr)
	}
	if !portInt64.IsOptional() || !portInt64.IsComputed() {
		t.Error("'listen.port' should be Optional and Computed")
	}

	// tag must be Computed-only.
	tagAttr, ok := s.Attributes["tag"]
	if !ok {
		t.Fatal("schema missing 'tag' attribute")
	}
	tagInt64, ok := tagAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'tag' attribute is %T, want schema.Int64Attribute", tagAttr)
	}
	if !tagInt64.IsComputed() {
		t.Error("'tag' should be Computed")
	}
	if tagInt64.IsOptional() || tagInt64.IsRequired() {
		t.Error("'tag' should be Computed-only (not Optional/Required)")
	}
}

// buildListenList is a helper that constructs a types.List of ListenModel
// from raw ip/port pairs. A nil *int64 port produces an unknown port value;
// a non-nil port produces a known value.
func buildListenList(t *testing.T, entries []struct {
	ip   string
	port *int64
}) types.List {
	t.Helper()
	ctx := context.Background()

	objs := make([]attr.Value, 0, len(entries))
	for _, e := range entries {
		var portVal types.Int64
		if e.port == nil {
			portVal = types.Int64Unknown()
		} else {
			portVal = types.Int64Value(*e.port)
		}
		obj, diags := types.ObjectValue(listenAttrTypes, map[string]attr.Value{
			"ip":   types.StringValue(e.ip),
			"port": portVal,
		})
		if diags.HasError() {
			t.Fatalf("ObjectValue failed: %v", diags)
		}
		objs = append(objs, obj)
	}

	list, diags := types.ListValue(types.ObjectType{AttrTypes: listenAttrTypes}, objs)
	if diags.HasError() {
		t.Fatalf("ListValue failed: %v", diags)
	}
	_ = ctx
	return list
}

// TestISCSIPortalApiPayload_ListenShape verifies that apiPayload produces the
// expected listen payload shape: ip always present, port NEVER present
// (TrueNAS 26.0 removed per-listen port from create/update; the global iSCSI
// listen_port governs), regardless of whether port is known or unknown in
// the model.
func TestISCSIPortalApiPayload_ListenShape(t *testing.T) {
	ctx := context.Background()

	port := int64(3260)
	listenList := buildListenList(t, []struct {
		ip   string
		port *int64
	}{
		{ip: "0.0.0.0", port: &port},
		{ip: "192.168.1.1", port: nil}, // unknown port
	})

	m := ISCSIPortalModel{
		ID:      types.Int64Value(0),
		Comment: types.StringNull(),
		Listen:  listenList,
		Tag:     types.Int64Value(0),
	}

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	listen, ok := payload["listen"].([]map[string]any)
	if !ok {
		t.Fatalf("payload[listen] is %T, want []map[string]any", payload["listen"])
	}
	if len(listen) != 2 {
		t.Fatalf("payload[listen] has %d items, want 2", len(listen))
	}

	// First item: ip present, port never sent even though it was known/set.
	if listen[0]["ip"] != "0.0.0.0" {
		t.Errorf("listen[0][ip] = %v, want 0.0.0.0", listen[0]["ip"])
	}
	if _, ok := listen[0]["port"]; ok {
		t.Errorf("listen[0] should not contain key 'port' (TrueNAS 26.0 rejects it), got %v", listen[0]["port"])
	}

	// Second item: ip present, port omitted (unknown, and never sent anyway).
	if listen[1]["ip"] != "192.168.1.1" {
		t.Errorf("listen[1][ip] = %v, want 192.168.1.1", listen[1]["ip"])
	}
	if _, ok := listen[1]["port"]; ok {
		t.Errorf("listen[1] should not contain key 'port', got %v", listen[1]["port"])
	}
}

// TestISCSIPortalApiPayload_CommentGuarded verifies that comment is omitted
// from the payload when unset (null or unknown), and included when set.
func TestISCSIPortalApiPayload_CommentGuarded(t *testing.T) {
	ctx := context.Background()

	listenList := buildListenList(t, []struct {
		ip   string
		port *int64
	}{})

	// Case 1: comment is null -> omitted.
	mNull := ISCSIPortalModel{
		ID:      types.Int64Value(0),
		Comment: types.StringNull(),
		Listen:  listenList,
		Tag:     types.Int64Value(0),
	}
	payload, diags := mNull.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}
	if _, ok := payload["comment"]; ok {
		t.Errorf("payload should not contain key 'comment' when null, got %v", payload["comment"])
	}

	// Case 2: comment is unknown -> omitted.
	mUnknown := ISCSIPortalModel{
		ID:      types.Int64Value(0),
		Comment: types.StringUnknown(),
		Listen:  listenList,
		Tag:     types.Int64Value(0),
	}
	payload, diags = mUnknown.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}
	if _, ok := payload["comment"]; ok {
		t.Errorf("payload should not contain key 'comment' when unknown, got %v", payload["comment"])
	}

	// Case 3: comment is set -> included.
	mSet := ISCSIPortalModel{
		ID:      types.Int64Value(0),
		Comment: types.StringValue("portal1"),
		Listen:  listenList,
		Tag:     types.Int64Value(0),
	}
	payload, diags = mSet.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}
	if payload["comment"] != "portal1" {
		t.Errorf("payload[comment] = %v, want portal1", payload["comment"])
	}
}

// TestISCSIPortalApiPayload_NilListen verifies that a null listen list
// defaults to an empty slice (not nil) in the payload.
func TestISCSIPortalApiPayload_NilListen(t *testing.T) {
	ctx := context.Background()

	m := ISCSIPortalModel{
		ID:      types.Int64Value(0),
		Comment: types.StringNull(),
		Listen:  types.ListNull(types.ObjectType{AttrTypes: listenAttrTypes}),
		Tag:     types.Int64Value(0),
	}

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	listen, ok := payload["listen"].([]map[string]any)
	if !ok {
		t.Fatalf("payload[listen] is %T, want []map[string]any", payload["listen"])
	}
	if listen == nil {
		t.Error("payload[listen] must not be nil, want empty slice")
	}
	if len(listen) != 0 {
		t.Errorf("payload[listen] has %d items, want 0", len(listen))
	}
}

// TestISCSIPortalResponseToModel verifies that responseToModel populates all
// fields correctly, including the nested listen list mapping.
func TestISCSIPortalResponseToModel(t *testing.T) {
	ctx := context.Background()

	api := &portalAPI{
		ID:      1,
		Comment: "portal1",
		Listen: []listenAPI{
			{IP: "0.0.0.0", Port: 3260},
		},
		Tag: 1,
	}

	var m ISCSIPortalModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if m.ID.ValueInt64() != 1 {
		t.Errorf("ID = %v, want 1", m.ID.ValueInt64())
	}
	if m.Comment.ValueString() != "portal1" {
		t.Errorf("Comment = %v, want portal1", m.Comment.ValueString())
	}
	if m.Tag.ValueInt64() != 1 {
		t.Errorf("Tag = %v, want 1", m.Tag.ValueInt64())
	}

	var listenItems []ListenModel
	if diags := m.Listen.ElementsAs(ctx, &listenItems, false); diags.HasError() {
		t.Fatalf("Listen.ElementsAs failed: %v", diags)
	}
	if len(listenItems) != 1 {
		t.Fatalf("Listen has %d items, want 1", len(listenItems))
	}
	if listenItems[0].IP.ValueString() != "0.0.0.0" {
		t.Errorf("Listen[0].IP = %v, want 0.0.0.0", listenItems[0].IP.ValueString())
	}
	if listenItems[0].Port.ValueInt64() != 3260 {
		t.Errorf("Listen[0].Port = %v, want 3260", listenItems[0].Port.ValueInt64())
	}
}

// TestISCSIPortalResponseToModel_NilListen verifies that a nil Listen slice
// in the API response maps to an empty (not null) Terraform list.
func TestISCSIPortalResponseToModel_NilListen(t *testing.T) {
	ctx := context.Background()

	api := &portalAPI{
		ID:      2,
		Comment: "",
		Listen:  nil,
		Tag:     2,
	}

	var m ISCSIPortalModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if m.Listen.IsNull() {
		t.Error("Listen should not be null when API returns nil listen, want empty list")
	}

	var listenItems []ListenModel
	if diags := m.Listen.ElementsAs(ctx, &listenItems, false); diags.HasError() {
		t.Fatalf("Listen.ElementsAs failed: %v", diags)
	}
	if len(listenItems) != 0 {
		t.Errorf("Listen has %d items, want 0", len(listenItems))
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nfs_config

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// int64Ptr is a small test helper for building *int64 API values.
func int64Ptr(v int64) *int64 { return &v }

// baseModel returns a fully-populated NFSConfigModel with known (non-null,
// non-unknown) values for every writable field, used as a starting point by
// tests that only want to vary one field at a time.
func baseModel(ctx context.Context, t *testing.T) *NFSConfigModel {
	t.Helper()

	protocols, diags := types.ListValueFrom(ctx, types.StringType, []string{"NFSV3", "NFSV4"})
	if diags.HasError() {
		t.Fatalf("unexpected error building list: %v", diags)
	}
	bindip, diags := types.ListValueFrom(ctx, types.StringType, []string{"192.0.2.10"})
	if diags.HasError() {
		t.Fatalf("unexpected error building list: %v", diags)
	}

	return &NFSConfigModel{
		Servers:         types.Int64Value(8),
		AllowNonroot:    types.BoolValue(false),
		Protocols:       protocols,
		V4Domain:        types.StringValue(""),
		V4Krb:           types.BoolValue(false),
		BindIP:          bindip,
		MountdPort:      types.Int64Value(618),
		RPCStatdPort:    types.Int64Value(662),
		RPCLockdPort:    types.Int64Value(32803),
		MountdLog:       types.BoolValue(false),
		StatdLockdLog:   types.BoolValue(false),
		UserdManageGids: types.BoolValue(false),
		RDMA:            types.BoolValue(false),
		ManagedNFSD:     types.BoolValue(true),
		V4KrbEnabled:    types.BoolValue(false),
		KeytabHasNFSSPN: types.BoolValue(false),
	}
}

// TestNFSConfigSchema_IDIsComputed verifies that "id" is a Computed-only
// StringAttribute with UseStateForUnknown, since it's a fixed singleton value
// never supplied by the user.
func TestNFSConfigSchema_IDIsComputed(t *testing.T) {
	s := resourceSchema()

	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	idStr, ok := idAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.StringAttribute", idAttr)
	}
	if !idStr.IsComputed() {
		t.Error("'id' should be Computed")
	}
	if idStr.IsRequired() || idStr.IsOptional() {
		t.Error("'id' should be Computed-only (not Required/Optional)")
	}
	if len(idStr.PlanModifiers) == 0 {
		t.Error("'id' should have plan modifiers (UseStateForUnknown)")
	}
}

// TestNFSConfigSchema_ComputedOnlyTrioNeverOptional verifies that
// managed_nfsd, v4_krb_enabled, and keytab_has_nfs_spn are Computed-only
// (never Optional) and carry UseStateForUnknown plan modifiers.
func TestNFSConfigSchema_ComputedOnlyTrioNeverOptional(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"managed_nfsd", "v4_krb_enabled", "keytab_has_nfs_spn"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		boolAttr, ok := attr.(schema.BoolAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.BoolAttribute", name, attr)
		}
		if !boolAttr.IsComputed() {
			t.Errorf("%q should be Computed", name)
		}
		if boolAttr.IsOptional() || boolAttr.IsRequired() {
			t.Errorf("%q should be Computed-only (not Optional/Required)", name)
		}
		if len(boolAttr.PlanModifiers) == 0 {
			t.Errorf("%q should have plan modifiers (UseStateForUnknown)", name)
		}
	}
}

// TestNFSConfigSchema_AllOtherFieldsOptionalComputed verifies that every
// writable field is Optional+Computed with a plan modifier, matching the "all
// writable fields Optional+Computed + UseStateForUnknown" contract in the
// task brief.
func TestNFSConfigSchema_AllOtherFieldsOptionalComputed(t *testing.T) {
	s := resourceSchema()

	stringFields := []string{"v4_domain"}
	for _, name := range stringFields {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		strAttr, ok := attr.(schema.StringAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.StringAttribute", name, attr)
		}
		if !strAttr.IsOptional() || !strAttr.IsComputed() {
			t.Errorf("%q should be Optional+Computed", name)
		}
		if len(strAttr.PlanModifiers) == 0 {
			t.Errorf("%q should have plan modifiers (UseStateForUnknown)", name)
		}
	}

	boolFields := []string{
		"allow_nonroot", "v4_krb", "mountd_log", "statd_lockd_log",
		"userd_manage_gids", "rdma",
	}
	for _, name := range boolFields {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		boolAttr, ok := attr.(schema.BoolAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.BoolAttribute", name, attr)
		}
		if !boolAttr.IsOptional() || !boolAttr.IsComputed() {
			t.Errorf("%q should be Optional+Computed", name)
		}
		if len(boolAttr.PlanModifiers) == 0 {
			t.Errorf("%q should have plan modifiers (UseStateForUnknown)", name)
		}
	}

	int64Fields := []string{"servers", "mountd_port", "rpcstatd_port", "rpclockd_port"}
	for _, name := range int64Fields {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		intAttr, ok := attr.(schema.Int64Attribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.Int64Attribute", name, attr)
		}
		if !intAttr.IsOptional() || !intAttr.IsComputed() {
			t.Errorf("%q should be Optional+Computed", name)
		}
		if len(intAttr.PlanModifiers) == 0 {
			t.Errorf("%q should have plan modifiers (UseStateForUnknown)", name)
		}
	}

	listFields := []string{"protocols", "bindip"}
	for _, name := range listFields {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		listAttr, ok := attr.(schema.ListAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.ListAttribute", name, attr)
		}
		if !listAttr.IsOptional() || !listAttr.IsComputed() {
			t.Errorf("%q should be Optional+Computed", name)
		}
		if len(listAttr.PlanModifiers) == 0 {
			t.Errorf("%q should have plan modifiers (UseStateForUnknown)", name)
		}
		if listAttr.ElementType != types.StringType {
			t.Errorf("%q ElementType = %v, want types.StringType", name, listAttr.ElementType)
		}
	}
}

// TestResponseToModel_ListsNilBecomeEmptyList verifies that nil list fields
// from the API map to empty (non-null) lists in the model, for both list
// fields.
func TestResponseToModel_ListsNilBecomeEmptyList(t *testing.T) {
	api := &nfsConfigAPI{
		ID:        1,
		Protocols: nil,
		BindIP:    nil,
	}

	m := &NFSConfigModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	for name, l := range map[string]types.List{
		"protocols": m.Protocols,
		"bindip":    m.BindIP,
	} {
		if l.IsNull() {
			t.Errorf("%s should not be null when API returns nil, want empty list", name)
		}
		var out []string
		d := l.ElementsAs(context.Background(), &out, false)
		if d.HasError() {
			t.Fatalf("unexpected error extracting %s elements: %v", name, d)
		}
		if len(out) != 0 {
			t.Errorf("%s = %v, want empty slice", name, out)
		}
	}

	if m.ID.ValueString() != nfsConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), nfsConfigResourceID)
	}
}

// TestResponseToModel_ListsSet verifies that non-nil list fields from the API
// are carried through as-is.
func TestResponseToModel_ListsSet(t *testing.T) {
	api := &nfsConfigAPI{
		ID:        1,
		Protocols: []string{"NFSV3", "NFSV4"},
		BindIP:    []string{"192.0.2.10"},
	}

	m := &NFSConfigModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	var protocols []string
	if d := m.Protocols.ElementsAs(context.Background(), &protocols, false); d.HasError() {
		t.Fatalf("unexpected error: %v", d)
	}
	if len(protocols) != 2 || protocols[0] != "NFSV3" || protocols[1] != "NFSV4" {
		t.Errorf("Protocols = %v, want [NFSV3 NFSV4]", protocols)
	}

	var bindip []string
	if d := m.BindIP.ElementsAs(context.Background(), &bindip, false); d.HasError() {
		t.Fatalf("unexpected error: %v", d)
	}
	if len(bindip) != 1 || bindip[0] != "192.0.2.10" {
		t.Errorf("BindIP = %v, want [192.0.2.10]", bindip)
	}
}

// TestResponseToModel_NullableIntsAPINilBecomesZero verifies that a nil API
// pointer for each of the four nullable int fields maps to 0 in the model,
// for both the resource and datasource models.
func TestResponseToModel_NullableIntsAPINilBecomesZero(t *testing.T) {
	api := &nfsConfigAPI{
		ID:           1,
		Servers:      nil,
		MountdPort:   nil,
		RPCStatdPort: nil,
		RPCLockdPort: nil,
	}

	m := &NFSConfigModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.Servers.ValueInt64() != 0 {
		t.Errorf("Servers = %d, want 0", m.Servers.ValueInt64())
	}
	if m.MountdPort.ValueInt64() != 0 {
		t.Errorf("MountdPort = %d, want 0", m.MountdPort.ValueInt64())
	}
	if m.RPCStatdPort.ValueInt64() != 0 {
		t.Errorf("RPCStatdPort = %d, want 0", m.RPCStatdPort.ValueInt64())
	}
	if m.RPCLockdPort.ValueInt64() != 0 {
		t.Errorf("RPCLockdPort = %d, want 0", m.RPCLockdPort.ValueInt64())
	}

	dm := &NFSConfigDataSourceModel{}
	diags = responseToDataSourceModel(context.Background(), api, dm)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if dm.Servers.ValueInt64() != 0 || dm.MountdPort.ValueInt64() != 0 ||
		dm.RPCStatdPort.ValueInt64() != 0 || dm.RPCLockdPort.ValueInt64() != 0 {
		t.Error("datasource model nullable int fields should be 0 when API returns nil")
	}
}

// TestResponseToModel_NullableIntsSet verifies that non-nil API values for
// each of the four nullable int fields are carried through as-is.
func TestResponseToModel_NullableIntsSet(t *testing.T) {
	api := &nfsConfigAPI{
		ID:           1,
		Servers:      int64Ptr(8),
		MountdPort:   int64Ptr(618),
		RPCStatdPort: int64Ptr(662),
		RPCLockdPort: int64Ptr(32803),
	}

	m := &NFSConfigModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.Servers.ValueInt64() != 8 {
		t.Errorf("Servers = %d, want 8", m.Servers.ValueInt64())
	}
	if m.MountdPort.ValueInt64() != 618 {
		t.Errorf("MountdPort = %d, want 618", m.MountdPort.ValueInt64())
	}
	if m.RPCStatdPort.ValueInt64() != 662 {
		t.Errorf("RPCStatdPort = %d, want 662", m.RPCStatdPort.ValueInt64())
	}
	if m.RPCLockdPort.ValueInt64() != 32803 {
		t.Errorf("RPCLockdPort = %d, want 32803", m.RPCLockdPort.ValueInt64())
	}
}

// TestResponseToDataSourceModel_ListsNilBecomeEmptyList mirrors the same
// nil-to-empty-list handling for the datasource model.
func TestResponseToDataSourceModel_ListsNilBecomeEmptyList(t *testing.T) {
	api := &nfsConfigAPI{
		ID:        1,
		Protocols: nil,
		BindIP:    nil,
	}

	m := &NFSConfigDataSourceModel{}
	diags := responseToDataSourceModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.Protocols.IsNull() || m.BindIP.IsNull() {
		t.Error("list fields should not be null when API returns nil, want empty lists")
	}
}

// TestUpdatePayload_ComputedOnlyTrioNeverInPayload verifies that
// managed_nfsd, v4_krb_enabled, and keytab_has_nfs_spn are never included in
// the update payload, even though the model carries known values for them:
// they are Computed-only and TrueNAS does not accept them as nfs.update
// arguments.
func TestUpdatePayload_ComputedOnlyTrioNeverInPayload(t *testing.T) {
	m := baseModel(context.Background(), t)
	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	for _, k := range []string{"managed_nfsd", "v4_krb_enabled", "keytab_has_nfs_spn"} {
		if _, ok := p[k]; ok {
			t.Errorf("%q should never be present in the update payload", k)
		}
	}
}

// TestUpdatePayload_OnlyKnownFieldsSent verifies that updatePayload omits any
// field whose model value is null or unknown.
func TestUpdatePayload_OnlyKnownFieldsSent(t *testing.T) {
	m := &NFSConfigModel{
		Servers:         types.Int64Null(),
		AllowNonroot:    types.BoolNull(),
		Protocols:       types.ListNull(types.StringType),
		V4Domain:        types.StringUnknown(),
		V4Krb:           types.BoolNull(),
		BindIP:          types.ListUnknown(types.StringType),
		MountdPort:      types.Int64Null(),
		RPCStatdPort:    types.Int64Unknown(),
		RPCLockdPort:    types.Int64Null(),
		MountdLog:       types.BoolNull(),
		StatdLockdLog:   types.BoolNull(),
		UserdManageGids: types.BoolNull(),
		RDMA:            types.BoolValue(true),
		ManagedNFSD:     types.BoolValue(true),
		V4KrbEnabled:    types.BoolValue(false),
		KeytabHasNFSSPN: types.BoolValue(false),
	}

	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	omitted := []string{
		"servers", "allow_nonroot", "protocols", "v4_domain", "v4_krb",
		"bindip", "mountd_port", "rpcstatd_port", "rpclockd_port",
		"mountd_log", "statd_lockd_log", "userd_manage_gids",
		"managed_nfsd", "v4_krb_enabled", "keytab_has_nfs_spn",
	}
	for _, k := range omitted {
		if _, ok := p[k]; ok {
			t.Errorf("expected %q to be omitted (null/unknown)", k)
		}
	}
	if v, ok := p["rdma"]; !ok || v != true {
		t.Errorf("expected 'rdma' = true, got %v (present=%v)", v, ok)
	}
}

// TestUpdatePayload_AllKnownFieldsSent verifies that updatePayload includes
// every guarded writable field when its model value is known, and that the
// computed-only trio is excluded even though the model carries known values
// for them.
func TestUpdatePayload_AllKnownFieldsSent(t *testing.T) {
	ctx := context.Background()
	m := baseModel(ctx, t)

	p, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	want := map[string]any{
		"allow_nonroot":     false,
		"v4_domain":         "",
		"v4_krb":            false,
		"mountd_log":        false,
		"statd_lockd_log":   false,
		"userd_manage_gids": false,
		"rdma":              false,
		"servers":           int64(8),
		"mountd_port":       int64(618),
		"rpcstatd_port":     int64(662),
		"rpclockd_port":     int64(32803),
	}
	for k, v := range want {
		if p[k] != v {
			t.Errorf("payload[%q] = %v, want %v", k, p[k], v)
		}
	}

	for name, wantVal := range map[string][]string{
		"protocols": {"NFSV3", "NFSV4"},
		"bindip":    {"192.0.2.10"},
	} {
		got, ok := p[name].([]string)
		if !ok {
			t.Fatalf("payload[%s] is %T, want []string", name, p[name])
		}
		if !reflect.DeepEqual(got, wantVal) {
			t.Errorf("payload[%s] = %v, want %v", name, got, wantVal)
		}
	}

	for _, k := range []string{"managed_nfsd", "v4_krb_enabled", "keytab_has_nfs_spn"} {
		if _, ok := p[k]; ok {
			t.Errorf("%q should never be present in the update payload", k)
		}
	}

	// want (11) + 2 lists, computed-only trio excluded.
	if len(p) != len(want)+2 {
		t.Errorf("payload has %d keys (%v), want %d", len(p), p, len(want)+2)
	}
}

// TestUpdatePayload_ListsKnownEmptyBecomeEmptySlice verifies that a known but
// empty list is sent as an empty slice, not nil/null, for both list fields.
func TestUpdatePayload_ListsKnownEmptyBecomeEmptySlice(t *testing.T) {
	ctx := context.Background()
	emptyList, diags := types.ListValueFrom(ctx, types.StringType, []string{})
	if diags.HasError() {
		t.Fatalf("unexpected error building empty list: %v", diags)
	}

	m := &NFSConfigModel{
		Servers:         types.Int64Null(),
		AllowNonroot:    types.BoolNull(),
		Protocols:       emptyList,
		V4Domain:        types.StringNull(),
		V4Krb:           types.BoolNull(),
		BindIP:          emptyList,
		MountdPort:      types.Int64Null(),
		RPCStatdPort:    types.Int64Null(),
		RPCLockdPort:    types.Int64Null(),
		MountdLog:       types.BoolNull(),
		StatdLockdLog:   types.BoolNull(),
		UserdManageGids: types.BoolNull(),
		RDMA:            types.BoolNull(),
	}

	p, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	for _, name := range []string{"protocols", "bindip"} {
		got, ok := p[name].([]string)
		if !ok {
			t.Fatalf("payload[%s] is %T, want []string", name, p[name])
		}
		if len(got) != 0 {
			t.Errorf("payload[%s] = %v, want empty slice", name, got)
		}
	}
}

// TestUpdatePayload_NullableIntThreeWay verifies the three-way nullable
// handling for each of servers, mountd_port, rpcstatd_port, and
// rpclockd_port: omitted when null/unknown, sent as nil when explicitly set
// to 0, and sent as the value otherwise.
func TestUpdatePayload_NullableIntThreeWay(t *testing.T) {
	ctx := context.Background()

	fields := []struct {
		name string
		set  func(m *NFSConfigModel, v types.Int64)
	}{
		{"servers", func(m *NFSConfigModel, v types.Int64) { m.Servers = v }},
		{"mountd_port", func(m *NFSConfigModel, v types.Int64) { m.MountdPort = v }},
		{"rpcstatd_port", func(m *NFSConfigModel, v types.Int64) { m.RPCStatdPort = v }},
		{"rpclockd_port", func(m *NFSConfigModel, v types.Int64) { m.RPCLockdPort = v }},
	}

	for _, f := range fields {
		t.Run(f.name, func(t *testing.T) {
			// Null -> omitted.
			m := baseModel(ctx, t)
			f.set(m, types.Int64Null())
			p, diags := m.updatePayload(ctx)
			if diags.HasError() {
				t.Fatalf("unexpected error: %v", diags)
			}
			if _, ok := p[f.name]; ok {
				t.Errorf("expected %q to be omitted when null", f.name)
			}

			// Unknown -> omitted.
			m2 := baseModel(ctx, t)
			f.set(m2, types.Int64Unknown())
			p2, diags := m2.updatePayload(ctx)
			if diags.HasError() {
				t.Fatalf("unexpected error: %v", diags)
			}
			if _, ok := p2[f.name]; ok {
				t.Errorf("expected %q to be omitted when unknown", f.name)
			}

			// Explicit 0 -> sent as nil.
			m3 := baseModel(ctx, t)
			f.set(m3, types.Int64Value(0))
			p3, diags := m3.updatePayload(ctx)
			if diags.HasError() {
				t.Fatalf("unexpected error: %v", diags)
			}
			if v, ok := p3[f.name]; !ok {
				t.Errorf("expected %q to be present (nil) when explicitly set to 0", f.name)
			} else if v != nil {
				t.Errorf("expected %q = nil when explicitly set to 0, got %v", f.name, v)
			}

			// Non-zero value -> sent as-is.
			m4 := baseModel(ctx, t)
			f.set(m4, types.Int64Value(42))
			p4, diags := m4.updatePayload(ctx)
			if diags.HasError() {
				t.Fatalf("unexpected error: %v", diags)
			}
			if v, ok := p4[f.name]; !ok || v != int64(42) {
				t.Errorf("expected %q = 42, got %v (present=%v)", f.name, v, ok)
			}
		})
	}
}

// planStateFromModel builds a tfsdk.Plan/tfsdk.State pair from a model using
// the resource schema, for ModifyPlan tests that need real (non-nil) plan
// and state data rather than the zero-value tftypes.Value.
func planStateFromModel(ctx context.Context, t *testing.T, s schema.Schema, m *NFSConfigModel) tftypes.Value {
	t.Helper()
	var p tfsdk.Plan
	p.Schema = s
	diags := p.Set(ctx, m)
	if diags.HasError() {
		t.Fatalf("unexpected error building plan/state: %v", diags)
	}
	return p.Raw
}

// nullValue builds a null tftypes.Value for the given schema, mirroring
// what Terraform sends for Plan on destroy (or, hypothetically, State on
// create).
func nullValue(ctx context.Context, s schema.Schema) tftypes.Value {
	return tftypes.NewValue(s.Type().TerraformType(ctx), nil)
}

// TestModifyPlan_UpdateMarksServerMutableFieldsUnknown verifies the fix for
// "managed_nfsd: was cty.True, but now cty.False": on an update (both plan
// and state non-null), ModifyPlan must mark managed_nfsd, v4_krb_enabled,
// and keytab_has_nfs_spn unknown, so a server-side flip of one of these
// booleans (e.g. triggered by a v4_domain change) doesn't produce an
// inconsistent-result-after-apply error against the prior state's value.
func TestModifyPlan_UpdateMarksServerMutableFieldsUnknown(t *testing.T) {
	ctx := context.Background()
	s := resourceSchema()

	state := baseModel(ctx, t)
	state.ID = types.StringValue(nfsConfigResourceID)
	state.ManagedNFSD = types.BoolValue(true)

	plan := baseModel(ctx, t)
	plan.ID = types.StringValue(nfsConfigResourceID)
	plan.V4Domain = types.StringValue("new.example") // the field actually being changed
	// UseStateForUnknown carries the prior state's value into the plan for
	// the computed-only trio, which is exactly the value ModifyPlan must
	// override.
	plan.ManagedNFSD = types.BoolValue(true)

	req := resource.ModifyPlanRequest{
		Plan:  tfsdk.Plan{Schema: s, Raw: planStateFromModel(ctx, t, s, plan)},
		State: tfsdk.State{Schema: s, Raw: planStateFromModel(ctx, t, s, state)},
	}
	resp := &resource.ModifyPlanResponse{Plan: req.Plan}

	r := &NFSConfigResource{}
	r.ModifyPlan(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics)
	}

	var got NFSConfigModel
	diags := resp.Plan.Get(ctx, &got)
	if diags.HasError() {
		t.Fatalf("unexpected error reading back plan: %v", diags)
	}

	if !got.ManagedNFSD.IsUnknown() {
		t.Errorf("managed_nfsd = %v, want unknown", got.ManagedNFSD)
	}
	if !got.V4KrbEnabled.IsUnknown() {
		t.Errorf("v4_krb_enabled = %v, want unknown", got.V4KrbEnabled)
	}
	if !got.KeytabHasNFSSPN.IsUnknown() {
		t.Errorf("keytab_has_nfs_spn = %v, want unknown", got.KeytabHasNFSSPN)
	}
	// The field actually being changed must be untouched by ModifyPlan.
	if got.V4Domain.ValueString() != "new.example" {
		t.Errorf("v4_domain = %q, want %q (untouched by ModifyPlan)", got.V4Domain.ValueString(), "new.example")
	}
}

// TestModifyPlan_DestroyNoOp verifies that ModifyPlan does nothing (no
// error, no attribute changes attempted) when the plan is null, i.e. on
// destroy — there is no plan to modify.
func TestModifyPlan_DestroyNoOp(t *testing.T) {
	ctx := context.Background()
	s := resourceSchema()

	state := baseModel(ctx, t)
	state.ID = types.StringValue(nfsConfigResourceID)

	nullPlan := tfsdk.Plan{Schema: s, Raw: nullValue(ctx, s)}
	req := resource.ModifyPlanRequest{
		Plan:  nullPlan,
		State: tfsdk.State{Schema: s, Raw: planStateFromModel(ctx, t, s, state)},
	}
	resp := &resource.ModifyPlanResponse{Plan: nullPlan}

	r := &NFSConfigResource{}
	r.ModifyPlan(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics)
	}
	if !resp.Plan.Raw.IsNull() {
		t.Error("Plan should remain null on destroy")
	}
}

// TestModifyPlan_CreateNoOp verifies that ModifyPlan does nothing when the
// state is null, i.e. on create — there is no prior state to carry forward,
// so nothing needs correcting.
func TestModifyPlan_CreateNoOp(t *testing.T) {
	ctx := context.Background()
	s := resourceSchema()

	plan := baseModel(ctx, t)
	plan.ID = types.StringUnknown()
	plan.ManagedNFSD = types.BoolUnknown()
	plan.V4KrbEnabled = types.BoolUnknown()
	plan.KeytabHasNFSSPN = types.BoolUnknown()

	planRaw := planStateFromModel(ctx, t, s, plan)
	nullState := tfsdk.State{Schema: s, Raw: nullValue(ctx, s)}
	req := resource.ModifyPlanRequest{
		Plan:  tfsdk.Plan{Schema: s, Raw: planRaw},
		State: nullState,
	}
	resp := &resource.ModifyPlanResponse{Plan: req.Plan}

	r := &NFSConfigResource{}
	r.ModifyPlan(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics)
	}
	if !resp.Plan.Raw.Equal(planRaw) {
		t.Error("Plan should be untouched on create")
	}
}

// TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder
// returns exactly one warning (no errors) and does not require or touch a
// client — this is what makes "Delete makes no client calls" verifiable:
// NFSConfigResource.Delete calls only this pure function.
func TestDeleteWarningDiagnostics(t *testing.T) {
	diags := deleteWarningDiagnostics()
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if len(diags) != 1 {
		t.Fatalf("expected exactly 1 diagnostic, got %d: %v", len(diags), diags)
	}
	summary := diags[0].Summary()
	detail := diags[0].Detail()
	if summary == "" || detail == "" {
		t.Error("expected non-empty summary and detail")
	}
	if detail != "NFS configuration left in place; removed from Terraform state only" {
		t.Errorf("detail = %q, want the documented warning text", detail)
	}
}

// TestNFSConfigDataSourceModel_MatchesSchema verifies that every tfsdk tag on
// NFSConfigDataSourceModel has a corresponding attribute in the datasource
// schema, and vice versa. terraform-plugin-framework requires an exact
// field/attribute match: any mismatch causes every datasource Read to fail
// with "Struct defines fields not found in object: ...".
func TestNFSConfigDataSourceModel_MatchesSchema(t *testing.T) {
	d := &NFSConfigDataSource{}
	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	schemaAttrs := make([]string, 0, len(resp.Schema.Attributes))
	for name := range resp.Schema.Attributes {
		schemaAttrs = append(schemaAttrs, name)
	}
	sort.Strings(schemaAttrs)

	modelType := reflect.TypeOf(NFSConfigDataSourceModel{})
	modelFields := make([]string, 0, modelType.NumField())
	for i := 0; i < modelType.NumField(); i++ {
		tag := modelType.Field(i).Tag.Get("tfsdk")
		if tag == "" {
			t.Fatalf("field %q has no tfsdk tag", modelType.Field(i).Name)
		}
		modelFields = append(modelFields, tag)
	}
	sort.Strings(modelFields)

	if !reflect.DeepEqual(schemaAttrs, modelFields) {
		t.Fatalf("NFSConfigDataSourceModel tfsdk tags %v do not match datasource schema attributes %v", modelFields, schemaAttrs)
	}
}

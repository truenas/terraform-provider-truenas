// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_realm

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func strList(t *testing.T, vals ...string) types.List {
	t.Helper()
	elems := make([]attr.Value, len(vals))
	for i, v := range vals {
		elems[i] = types.StringValue(v)
	}
	l, diags := types.ListValue(types.StringType, elems)
	if diags.HasError() {
		t.Fatalf("building string list: %v", diags)
	}
	return l
}

func strPtr(s string) *string { return &s }

// TestApiPayload_AllFieldsSet verifies that apiPayload includes every
// field — realm, primary_kdc, kdc, admin_server, kpasswd_server — when all
// are known, with the exact keys observed in the kerberos.realm.create
// probe.
func TestApiPayload_AllFieldsSet(t *testing.T) {
	ctx := context.Background()
	m := &KerberosRealmModel{
		Realm:         types.StringValue("EXAMPLE.COM"),
		PrimaryKDC:    types.StringValue("kdc1.example.com"),
		KDC:           strList(t, "kdc1.example.com", "kdc2.example.com"),
		AdminServer:   strList(t, "admin.example.com"),
		KpasswdServer: strList(t, "kpasswd.example.com"),
	}

	p, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}

	requiredKeys := []string{"realm", "primary_kdc", "kdc", "admin_server", "kpasswd_server"}
	for _, key := range requiredKeys {
		if _, ok := p[key]; !ok {
			t.Errorf("payload missing key %q", key)
		}
	}
	if len(p) != len(requiredKeys) {
		t.Errorf("payload has %d keys (%v), want %d", len(p), p, len(requiredKeys))
	}

	if p["realm"] != "EXAMPLE.COM" {
		t.Errorf("payload[realm] = %v, want \"EXAMPLE.COM\"", p["realm"])
	}
	if p["primary_kdc"] != "kdc1.example.com" {
		t.Errorf("payload[primary_kdc] = %v, want \"kdc1.example.com\"", p["primary_kdc"])
	}

	kdc, ok := p["kdc"].([]string)
	if !ok {
		t.Fatalf("payload[kdc] is %T, want []string", p["kdc"])
	}
	if len(kdc) != 2 || kdc[0] != "kdc1.example.com" || kdc[1] != "kdc2.example.com" {
		t.Errorf("payload[kdc] = %v, want [kdc1.example.com kdc2.example.com]", kdc)
	}

	adminServer, ok := p["admin_server"].([]string)
	if !ok {
		t.Fatalf("payload[admin_server] is %T, want []string", p["admin_server"])
	}
	if len(adminServer) != 1 || adminServer[0] != "admin.example.com" {
		t.Errorf("payload[admin_server] = %v, want [admin.example.com]", adminServer)
	}
}

// TestApiPayload_UnsetOptionalsOmitted verifies that primary_kdc, kdc,
// admin_server, and kpasswd_server are omitted from the payload when null
// or unknown, leaving only the Required "realm" field — so the
// TrueNAS-side defaults (primary_kdc=null, kdc=[], admin_server=[],
// kpasswd_server=[]) take effect instead of an explicit zero value being
// sent.
func TestApiPayload_UnsetOptionalsOmitted(t *testing.T) {
	ctx := context.Background()
	m := &KerberosRealmModel{
		Realm:         types.StringValue("EXAMPLE.COM"),
		PrimaryKDC:    types.StringNull(),
		KDC:           types.ListUnknown(types.StringType),
		AdminServer:   types.ListNull(types.StringType),
		KpasswdServer: types.ListUnknown(types.StringType),
	}

	p, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}

	if len(p) != 1 {
		t.Fatalf("payload has %d keys (%v), want 1 (only realm)", len(p), p)
	}
	if p["realm"] != "EXAMPLE.COM" {
		t.Errorf("payload[realm] = %v, want \"EXAMPLE.COM\"", p["realm"])
	}
	for _, key := range []string{"primary_kdc", "kdc", "admin_server", "kpasswd_server"} {
		if _, ok := p[key]; ok {
			t.Errorf("expected %q to be omitted (null/unknown), got %v", key, p[key])
		}
	}
}

// TestApiPayload_EmptyListIsSentExplicitly verifies that a known, non-null
// empty list (e.g. kdc = []) IS sent in the payload — distinct from
// null/unknown — so a user can explicitly clear a previously-set list back
// to DNS-lookup defaults.
func TestApiPayload_EmptyListIsSentExplicitly(t *testing.T) {
	ctx := context.Background()
	m := &KerberosRealmModel{
		Realm:         types.StringValue("EXAMPLE.COM"),
		PrimaryKDC:    types.StringNull(),
		KDC:           strList(t),
		AdminServer:   types.ListNull(types.StringType),
		KpasswdServer: types.ListNull(types.StringType),
	}

	p, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}

	kdc, ok := p["kdc"].([]string)
	if !ok {
		t.Fatalf("payload[kdc] is %T, want []string (explicit empty list should be sent)", p["kdc"])
	}
	if len(kdc) != 0 {
		t.Errorf("payload[kdc] = %v, want empty", kdc)
	}
}

// TestResponseToModel verifies responseToModel against the exact shape
// observed from a live kerberos.realm.get_instance/query call, including a
// non-null primary_kdc.
func TestResponseToModel(t *testing.T) {
	ctx := context.Background()
	api := &kerberosRealmAPI{
		ID:            1,
		Realm:         "EXAMPLE.COM",
		PrimaryKDC:    strPtr("kdc1.example.com"),
		KDC:           []string{"kdc1.example.com"},
		AdminServer:   []string{"admin.example.com"},
		KpasswdServer: []string{},
	}

	m := &KerberosRealmModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueInt64() != 1 {
		t.Errorf("ID = %v, want 1", m.ID)
	}
	if m.Realm.ValueString() != "EXAMPLE.COM" {
		t.Errorf("Realm = %q, want \"EXAMPLE.COM\"", m.Realm.ValueString())
	}
	if m.PrimaryKDC.IsNull() || m.PrimaryKDC.ValueString() != "kdc1.example.com" {
		t.Errorf("PrimaryKDC = %v, want \"kdc1.example.com\"", m.PrimaryKDC)
	}

	var kdc []string
	diags = m.KDC.ElementsAs(ctx, &kdc, false)
	if diags.HasError() {
		t.Fatalf("reading back kdc: %v", diags)
	}
	if len(kdc) != 1 || kdc[0] != "kdc1.example.com" {
		t.Errorf("kdc = %v, want [kdc1.example.com]", kdc)
	}
}

// TestResponseToModel_NilPrimaryKDCStaysNull verifies that a nil
// primary_kdc from the API maps to a null (not empty-string) types.String,
// preserving the distinction the anyOf string|null API shape carries.
func TestResponseToModel_NilPrimaryKDCStaysNull(t *testing.T) {
	ctx := context.Background()
	api := &kerberosRealmAPI{ID: 1, Realm: "EXAMPLE.COM", PrimaryKDC: nil}

	m := &KerberosRealmModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !m.PrimaryKDC.IsNull() {
		t.Errorf("PrimaryKDC = %v, want null", m.PrimaryKDC)
	}
}

// TestResponseToModel_NilListsBecomeEmptyLists verifies that nil kdc/
// admin_server/kpasswd_server slices from the API map to empty (non-null)
// lists, matching the nil-guard convention used elsewhere for API-returned
// lists.
func TestResponseToModel_NilListsBecomeEmptyLists(t *testing.T) {
	ctx := context.Background()
	api := &kerberosRealmAPI{ID: 1, Realm: "EXAMPLE.COM"}

	m := &KerberosRealmModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	for name, l := range map[string]types.List{"kdc": m.KDC, "admin_server": m.AdminServer, "kpasswd_server": m.KpasswdServer} {
		if l.IsNull() {
			t.Errorf("%s should not be null when API returns nil, want empty list", name)
		}
		var v []string
		diags = l.ElementsAs(ctx, &v, false)
		if diags.HasError() {
			t.Fatalf("reading back %s: %v", name, diags)
		}
		if len(v) != 0 {
			t.Errorf("%s = %v, want empty", name, v)
		}
	}
}

// TestResponseToDataSourceModel mirrors TestResponseToModel for the
// datasource model.
func TestResponseToDataSourceModel(t *testing.T) {
	ctx := context.Background()
	api := &kerberosRealmAPI{
		ID:            1,
		Realm:         "EXAMPLE.COM",
		PrimaryKDC:    nil,
		KDC:           []string{"kdc1.example.com"},
		AdminServer:   []string{},
		KpasswdServer: []string{},
	}

	m := &KerberosRealmDataSourceModel{}
	diags := responseToDataSourceModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.Realm.ValueString() != "EXAMPLE.COM" {
		t.Errorf("Realm = %q, want \"EXAMPLE.COM\"", m.Realm.ValueString())
	}
	if !m.PrimaryKDC.IsNull() {
		t.Errorf("PrimaryKDC = %v, want null", m.PrimaryKDC)
	}
}

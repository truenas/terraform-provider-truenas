// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_config

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestUpdatePayload_AllFieldsSet verifies that updatePayload includes every
// field with the exact keys observed in the kerberos.update probe.
func TestUpdatePayload_AllFieldsSet(t *testing.T) {
	ctx := context.Background()
	m := &KerberosConfigModel{
		AppdefaultsAux: types.StringValue("no_addresses = true"),
		LibdefaultsAux: types.StringValue("dns_lookup_realm = false"),
	}

	p, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("updatePayload returned diagnostics errors: %v", diags)
	}

	want := map[string]any{
		"appdefaults_aux": "no_addresses = true",
		"libdefaults_aux": "dns_lookup_realm = false",
	}
	for k, v := range want {
		if p[k] != v {
			t.Errorf("payload[%q] = %v, want %v", k, p[k], v)
		}
	}
	if len(p) != 2 {
		t.Errorf("payload has %d keys (%v), want 2", len(p), p)
	}
}

// TestUpdatePayload_UnsetOptionalsOmitted verifies that both fields are
// omitted when null/unknown, so the current TrueNAS-side value is left
// unchanged rather than overwritten with a zero value.
func TestUpdatePayload_UnsetOptionalsOmitted(t *testing.T) {
	ctx := context.Background()
	m := &KerberosConfigModel{
		AppdefaultsAux: types.StringNull(),
		LibdefaultsAux: types.StringUnknown(),
	}

	p, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("updatePayload returned diagnostics errors: %v", diags)
	}
	if len(p) != 0 {
		t.Errorf("payload has %d keys (%v), want 0 (all omitted)", len(p), p)
	}
}

// TestResponseToModel verifies responseToModel against the shape probed
// from a live kerberos.config call.
func TestResponseToModel(t *testing.T) {
	ctx := context.Background()
	api := &kerberosConfigAPI{
		ID:             1,
		AppdefaultsAux: "no_addresses = true",
		LibdefaultsAux: "",
	}

	m := &KerberosConfigModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueString() != kerberosConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), kerberosConfigResourceID)
	}
	if m.AppdefaultsAux.ValueString() != "no_addresses = true" {
		t.Errorf("AppdefaultsAux = %q, want \"no_addresses = true\"", m.AppdefaultsAux.ValueString())
	}
	if m.LibdefaultsAux.ValueString() != "" {
		t.Errorf("LibdefaultsAux = %q, want \"\"", m.LibdefaultsAux.ValueString())
	}
}

// TestResponseToDataSourceModel mirrors TestResponseToModel for the
// datasource model.
func TestResponseToDataSourceModel(t *testing.T) {
	ctx := context.Background()
	api := &kerberosConfigAPI{ID: 1, AppdefaultsAux: "foo", LibdefaultsAux: "bar"}

	m := &KerberosConfigDataSourceModel{}
	diags := responseToDataSourceModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.ID.ValueString() != kerberosConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), kerberosConfigResourceID)
	}
	if m.AppdefaultsAux.ValueString() != "foo" {
		t.Errorf("AppdefaultsAux = %q, want \"foo\"", m.AppdefaultsAux.ValueString())
	}
	if m.LibdefaultsAux.ValueString() != "bar" {
		t.Errorf("LibdefaultsAux = %q, want \"bar\"", m.LibdefaultsAux.ValueString())
	}
}

// TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder
// returns exactly one warning (no errors) and does not require or touch a
// client — this is what makes "Delete makes no client calls" verifiable.
func TestDeleteWarningDiagnostics(t *testing.T) {
	diags := deleteWarningDiagnostics()
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if len(diags) != 1 {
		t.Fatalf("expected exactly 1 diagnostic, got %d: %v", len(diags), diags)
	}
	if diags[0].Detail() != "Kerberos configuration left in place; removed from Terraform state only" {
		t.Errorf("detail = %q, want the documented warning text", diags[0].Detail())
	}
}

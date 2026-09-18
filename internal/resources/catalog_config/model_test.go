// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package catalog_config

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestUpdatePayload_PreferredTrainsSet verifies updatePayload includes
// preferred_trains with the exact key catalog.update accepts (probed live —
// see task-3-report.md).
func TestUpdatePayload_PreferredTrainsSet(t *testing.T) {
	ctx := context.Background()
	trains, _ := types.ListValueFrom(ctx, types.StringType, []string{"community", "stable"})
	m := &CatalogConfigModel{PreferredTrains: trains}

	p, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	got, ok := p["preferred_trains"].([]string)
	if !ok || len(got) != 2 || got[0] != "community" || got[1] != "stable" {
		t.Errorf("preferred_trains = %#v, want [community stable]", p["preferred_trains"])
	}
	if len(p) != 1 {
		t.Errorf("payload has %d keys (%v), want 1", len(p), p)
	}
}

// TestUpdatePayload_UnsetOptionalOmitted verifies preferred_trains is
// omitted when null/unknown, so the current TrueNAS-side value is left
// unchanged rather than overwritten with an explicit empty list.
func TestUpdatePayload_UnsetOptionalOmitted(t *testing.T) {
	ctx := context.Background()

	for name, m := range map[string]*CatalogConfigModel{
		"null":    {PreferredTrains: types.ListNull(types.StringType)},
		"unknown": {PreferredTrains: types.ListUnknown(types.StringType)},
	} {
		p, diags := m.updatePayload(ctx)
		if diags.HasError() {
			t.Fatalf("%s: unexpected error: %v", name, diags)
		}
		if len(p) != 0 {
			t.Errorf("%s: payload has %d keys (%v), want 0 (omitted)", name, len(p), p)
		}
	}
}

// TestResponseToModel verifies responseToModel against the shape probed
// from a live catalog.config call (identical on TrueNAS 25.10 and 26.0).
func TestResponseToModel(t *testing.T) {
	ctx := context.Background()
	api := &catalogConfigAPI{
		ID:              "TRUENAS",
		Label:           "TRUENAS",
		Location:        "/mnt/.ix-apps/truenas_catalog",
		PreferredTrains: []string{"community", "stable"},
	}

	m := &CatalogConfigModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueString() != catalogConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), catalogConfigResourceID)
	}
	if m.Label.ValueString() != "TRUENAS" {
		t.Errorf("Label = %q, want TRUENAS", m.Label.ValueString())
	}
	if m.Location.ValueString() != "/mnt/.ix-apps/truenas_catalog" {
		t.Errorf("Location = %q, want /mnt/.ix-apps/truenas_catalog", m.Location.ValueString())
	}

	var trains []string
	diags = m.PreferredTrains.ElementsAs(ctx, &trains, false)
	if diags.HasError() {
		t.Fatalf("reading back preferred_trains: %v", diags)
	}
	if len(trains) != 2 || trains[0] != "community" || trains[1] != "stable" {
		t.Errorf("preferred_trains = %v, want [community stable]", trains)
	}
}

// TestResponseToModel_NilPreferredTrainsBecomesEmptyList verifies a nil
// []string from the API maps to an empty (non-null) list, matching the
// nil-guard convention established by audit_config's stringListOrEmpty.
func TestResponseToModel_NilPreferredTrainsBecomesEmptyList(t *testing.T) {
	ctx := context.Background()
	api := &catalogConfigAPI{ID: "TRUENAS", Label: "TRUENAS", Location: "/somewhere"}

	m := &CatalogConfigModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.PreferredTrains.IsNull() {
		t.Error("PreferredTrains should not be null when the API returns a nil slice, want empty list")
	}
	var trains []string
	diags = m.PreferredTrains.ElementsAs(ctx, &trains, false)
	if diags.HasError() {
		t.Fatalf("reading back preferred_trains: %v", diags)
	}
	if len(trains) != 0 {
		t.Errorf("preferred_trains = %v, want empty", trains)
	}
}

// TestResponseToDataSourceModel verifies responseToDataSourceModel maps both
// catalog.config's preferred_trains and the separately-fetched
// catalog.trains list correctly, including catalog.trains coming back empty
// (probed live on a TrueNAS 25.10 box with Docker/apps unconfigured).
func TestResponseToDataSourceModel(t *testing.T) {
	ctx := context.Background()
	api := &catalogConfigAPI{
		ID:              "TRUENAS",
		Label:           "TRUENAS",
		Location:        "/var/run/middleware/ix-apps/catalogs",
		PreferredTrains: []string{"community", "stable"},
	}

	m := &CatalogConfigDataSourceModel{}
	diags := responseToDataSourceModel(ctx, api, nil, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.ID.ValueString() != catalogConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), catalogConfigResourceID)
	}
	if m.Trains.IsNull() {
		t.Error("Trains should not be null when catalog.trains returns nil, want empty list")
	}
	var trains []string
	diags = m.Trains.ElementsAs(ctx, &trains, false)
	if diags.HasError() {
		t.Fatalf("reading back trains: %v", diags)
	}
	if len(trains) != 0 {
		t.Errorf("trains = %v, want empty", trains)
	}

	m2 := &CatalogConfigDataSourceModel{}
	diags = responseToDataSourceModel(ctx, api, []string{"stable", "enterprise", "community", "test", "dev"}, m2)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	var trains2 []string
	diags = m2.Trains.ElementsAs(ctx, &trains2, false)
	if diags.HasError() {
		t.Fatalf("reading back trains: %v", diags)
	}
	if len(trains2) != 5 || trains2[0] != "stable" || trains2[4] != "dev" {
		t.Errorf("trains = %v, want [stable enterprise community test dev]", trains2)
	}
}

// TestDeleteWarningDiagnostics verifies Delete's diagnostic builder returns
// exactly one warning and never touches a client.
func TestDeleteWarningDiagnostics(t *testing.T) {
	diags := deleteWarningDiagnostics()
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if len(diags) != 1 {
		t.Fatalf("expected exactly 1 diagnostic, got %d: %v", len(diags), diags)
	}
	if diags[0].Detail() != "Catalog configuration left in place; removed from Terraform state only" {
		t.Errorf("detail = %q, want the documented warning text", diags[0].Detail())
	}
}

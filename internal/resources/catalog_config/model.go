// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package catalog_config

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// catalogConfigResourceID is the fixed Terraform ID for this singleton
// resource: there is exactly one app catalog configuration per TrueNAS
// system (catalog.config always returns a single record), and it is never
// created or deleted on TrueNAS itself. The API's own "id" (probed: always
// the string "TRUENAS" on both releases — a string, unlike docker_config's
// numeric id) is an internal implementation detail and is intentionally not
// surfaced in the model, mirroring the audit_config/docker_config singleton
// pattern.
const catalogConfigResourceID = "catalog_config"

// CatalogConfigModel is the Terraform state model for truenas_catalog_config.
//
// Label and Location are read-only: catalog.update's accepts schema is
// {preferred_trains} only, confirmed identical on both probed releases (see
// task-3-report.md) — there is nothing this resource can do to change them,
// so they are surfaced purely for visibility.
type CatalogConfigModel struct {
	ID              types.String `tfsdk:"id"`
	Label           types.String `tfsdk:"label"`
	Location        types.String `tfsdk:"location"`
	PreferredTrains types.List   `tfsdk:"preferred_trains"`
}

// CatalogConfigDataSourceModel is the read-only model for the
// truenas_catalog_config datasource. Trains additionally exposes
// catalog.trains (the full list of train names the catalog currently
// knows about, independent of which ones are "preferred") — see
// datasource.go for why this extra call was judged trivially cheap enough
// to include.
type CatalogConfigDataSourceModel struct {
	ID              types.String `tfsdk:"id"`
	Label           types.String `tfsdk:"label"`
	Location        types.String `tfsdk:"location"`
	PreferredTrains types.List   `tfsdk:"preferred_trains"`
	Trains          types.List   `tfsdk:"trains"`
}

// catalogConfigAPI mirrors the JSON object returned by catalog.config and
// catalog.update. Probed live against TrueNAS 25.10 and 26.0 (see
// task-3-report.md): byte-identical shape on both releases, no version
// gating needed anywhere in this resource.
type catalogConfigAPI struct {
	ID              string   `json:"id"`
	Label           string   `json:"label"`
	Location        string   `json:"location"`
	PreferredTrains []string `json:"preferred_trains"`
}

// stringListOrEmpty converts a possibly-nil []string from the API into a
// non-null types.List, matching the nil-guard convention established by
// audit_config's stringListOrEmpty.
func stringListOrEmpty(ctx context.Context, vals []string) (types.List, diag.Diagnostics) {
	if vals == nil {
		vals = []string{}
	}
	return types.ListValueFrom(ctx, types.StringType, vals)
}

// responseToModel maps a catalogConfigAPI response onto a CatalogConfigModel.
func responseToModel(ctx context.Context, api *catalogConfigAPI, m *CatalogConfigModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(catalogConfigResourceID)
	m.Label = types.StringValue(api.Label)
	m.Location = types.StringValue(api.Location)

	trains, d := stringListOrEmpty(ctx, api.PreferredTrains)
	diags.Append(d...)
	m.PreferredTrains = trains

	return diags
}

// responseToDataSourceModel maps a catalogConfigAPI response (plus a
// separately-fetched catalog.trains list) onto a CatalogConfigDataSourceModel.
func responseToDataSourceModel(ctx context.Context, api *catalogConfigAPI, availableTrains []string, m *CatalogConfigDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(catalogConfigResourceID)
	m.Label = types.StringValue(api.Label)
	m.Location = types.StringValue(api.Location)

	preferred, d := stringListOrEmpty(ctx, api.PreferredTrains)
	diags.Append(d...)
	m.PreferredTrains = preferred

	trains, d := stringListOrEmpty(ctx, availableTrains)
	diags.Append(d...)
	m.Trains = trains

	return diags
}

// updatePayload builds the catalog.update argument. preferred_trains is
// guarded: only included when known (Optional+Computed), so an unset
// optional is omitted entirely and the TrueNAS-side current value is left
// unchanged rather than overwritten with an explicit empty list.
func (m *CatalogConfigModel) updatePayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{}

	if !m.PreferredTrains.IsNull() && !m.PreferredTrains.IsUnknown() {
		var trains []string
		diags.Append(m.PreferredTrains.ElementsAs(ctx, &trains, false)...)
		if diags.HasError() {
			return nil, diags
		}
		p["preferred_trains"] = trains
	}

	return p, diags
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by Delete.
// Delete makes NO client calls: removing this resource from Terraform state
// must never reset the box's catalog preferences. Splitting this into its
// own function keeps Delete's "no client calls" contract independently
// unit-testable.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"Catalog configuration left in place",
		"Catalog configuration left in place; removed from Terraform state only",
	)
	return diags
}

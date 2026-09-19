// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package reporting_exporter

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// graphiteExporterType is the only value reporting.exporters.exporter_schemas
// currently returns on either TrueNAS 25.10 or 26.0 (probed live
// against both): a single discriminated variant, "GRAPHITE". Since there is
// exactly one variant, "attributes" is modeled as a typed nested block with
// GRAPHITE's own fields hoisted directly into it, rather than the free-form
// JSON-string "attributes" pattern used by truenas_alert_service (which
// fronts several genuinely different variants: Mail, SNMPTrap, Slack,
// PagerDuty, ...). "exporter_type" itself is not exposed as a schema
// attribute — it is a fixed constant this package injects into every
// create/update payload. Adding a second exporter type in the future would
// require a schema change here (e.g. splitting "attributes" per type or
// moving to the JSON-string pattern); that is an acceptable, explicit
// tradeoff for a materially simpler and more type-safe schema today.
const graphiteExporterType = "GRAPHITE"

// attributesAttrTypes describes the attribute types of the nested
// "attributes" object (GRAPHITE-specific fields only).
var attributesAttrTypes = map[string]attr.Type{
	"destination_ip":            types.StringType,
	"destination_port":          types.Int64Type,
	"namespace":                 types.StringType,
	"prefix":                    types.StringType,
	"update_every":              types.Int64Type,
	"buffer_on_failures":        types.Int64Type,
	"send_names_instead_of_ids": types.BoolType,
	"matching_charts":           types.StringType,
}

// AttributesModel maps to the nested "attributes" attribute.
type AttributesModel struct {
	DestinationIP         types.String `tfsdk:"destination_ip"`
	DestinationPort       types.Int64  `tfsdk:"destination_port"`
	Namespace             types.String `tfsdk:"namespace"`
	Prefix                types.String `tfsdk:"prefix"`
	UpdateEvery           types.Int64  `tfsdk:"update_every"`
	BufferOnFailures      types.Int64  `tfsdk:"buffer_on_failures"`
	SendNamesInsteadOfIDs types.Bool   `tfsdk:"send_names_instead_of_ids"`
	MatchingCharts        types.String `tfsdk:"matching_charts"`
}

// ReportingExporterModel is the Terraform state model for
// truenas_reporting_exporter.
type ReportingExporterModel struct {
	ID         types.Int64  `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Enabled    types.Bool   `tfsdk:"enabled"`
	Attributes types.Object `tfsdk:"attributes"`
}

// ReportingExporterDataSourceModel is the read-only lookup model for the
// truenas_reporting_exporter datasource.
type ReportingExporterDataSourceModel struct {
	ID         types.Int64  `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Enabled    types.Bool   `tfsdk:"enabled"`
	Attributes types.Object `tfsdk:"attributes"`
}

// attributesAPI is the JSON wire format of the "attributes" object, probed
// live from reporting.exporters.create/get_instance/update on both TrueNAS
// 25.10 and 26.0 (identical shape, no version gating needed). Only the
// GRAPHITE variant exists; "exporter_type" is parsed but not surfaced in the
// Terraform model since it is currently constant.
type attributesAPI struct {
	ExporterType          string `json:"exporter_type"`
	DestinationIP         string `json:"destination_ip"`
	DestinationPort       int64  `json:"destination_port"`
	Prefix                string `json:"prefix"`
	Namespace             string `json:"namespace"`
	UpdateEvery           int64  `json:"update_every"`
	BufferOnFailures      int64  `json:"buffer_on_failures"`
	SendNamesInsteadOfIDs bool   `json:"send_names_instead_of_ids"`
	MatchingCharts        string `json:"matching_charts"`
}

// reportingExporterAPI mirrors the JSON object returned by
// reporting.exporters.create, .update, .get_instance, and .query.
type reportingExporterAPI struct {
	ID         int64         `json:"id"`
	Name       string        `json:"name"`
	Enabled    bool          `json:"enabled"`
	Attributes attributesAPI `json:"attributes"`
}

// attributesObjectValue builds a types.Object for the nested "attributes"
// attribute from an API response.
func attributesObjectValue(ctx context.Context, api attributesAPI) (types.Object, diag.Diagnostics) {
	return types.ObjectValueFrom(ctx, attributesAttrTypes, AttributesModel{
		DestinationIP:         types.StringValue(api.DestinationIP),
		DestinationPort:       types.Int64Value(api.DestinationPort),
		Namespace:             types.StringValue(api.Namespace),
		Prefix:                types.StringValue(api.Prefix),
		UpdateEvery:           types.Int64Value(api.UpdateEvery),
		BufferOnFailures:      types.Int64Value(api.BufferOnFailures),
		SendNamesInsteadOfIDs: types.BoolValue(api.SendNamesInsteadOfIDs),
		MatchingCharts:        types.StringValue(api.MatchingCharts),
	})
}

// responseToModel maps a reportingExporterAPI response onto a
// ReportingExporterModel.
func responseToModel(ctx context.Context, api *reportingExporterAPI, m *ReportingExporterModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.Enabled = types.BoolValue(api.Enabled)

	attrs, d := attributesObjectValue(ctx, api.Attributes)
	diags.Append(d...)
	m.Attributes = attrs

	return diags
}

// responseToDataSourceModel maps a reportingExporterAPI response onto a
// ReportingExporterDataSourceModel.
func responseToDataSourceModel(ctx context.Context, api *reportingExporterAPI, m *ReportingExporterDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.Enabled = types.BoolValue(api.Enabled)

	attrs, d := attributesObjectValue(ctx, api.Attributes)
	diags.Append(d...)
	m.Attributes = attrs

	return diags
}

// attributesPayload builds the "attributes" sub-map of the
// create/update payload from the plan's nested Attributes object.
// "exporter_type" is always injected as the fixed GRAPHITE constant.
// destination_ip, destination_port, and namespace are Required in the
// schema, so they are always included; the remaining fields are
// Optional+Computed and are included only when known and non-null, so an
// unset optional is omitted entirely and the TrueNAS-side default takes
// effect instead of an explicit zero value.
func attributesPayload(ctx context.Context, attrsObj types.Object) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	var a AttributesModel
	diags.Append(attrsObj.As(ctx, &a, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}

	p := map[string]any{
		"exporter_type":    graphiteExporterType,
		"destination_ip":   a.DestinationIP.ValueString(),
		"destination_port": a.DestinationPort.ValueInt64(),
		"namespace":        a.Namespace.ValueString(),
	}

	if !a.Prefix.IsNull() && !a.Prefix.IsUnknown() {
		p["prefix"] = a.Prefix.ValueString()
	}
	if !a.UpdateEvery.IsNull() && !a.UpdateEvery.IsUnknown() {
		p["update_every"] = a.UpdateEvery.ValueInt64()
	}
	if !a.BufferOnFailures.IsNull() && !a.BufferOnFailures.IsUnknown() {
		p["buffer_on_failures"] = a.BufferOnFailures.ValueInt64()
	}
	if !a.SendNamesInsteadOfIDs.IsNull() && !a.SendNamesInsteadOfIDs.IsUnknown() {
		p["send_names_instead_of_ids"] = a.SendNamesInsteadOfIDs.ValueBool()
	}
	if !a.MatchingCharts.IsNull() && !a.MatchingCharts.IsUnknown() {
		p["matching_charts"] = a.MatchingCharts.ValueString()
	}

	return p, diags
}

// apiPayload builds the {name, enabled, attributes} payload shared by
// reporting.exporters.create and reporting.exporters.update. Both "name" and
// "enabled" are Required in the schema (reporting.exporters.create requires
// them with no server-side default), so they are always included.
func (m *ReportingExporterModel) apiPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	attrs, diags := attributesPayload(ctx, m.Attributes)
	if diags.HasError() {
		return nil, diags
	}

	payload := map[string]any{
		"name":       m.Name.ValueString(),
		"enabled":    m.Enabled.ValueBool(),
		"attributes": attrs,
	}

	return payload, diags
}

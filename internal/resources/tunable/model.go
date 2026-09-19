// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package tunable

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TunableModel is the Terraform state model for truenas_tunable.
type TunableModel struct {
	ID              types.Int64  `tfsdk:"id"`
	Var             types.String `tfsdk:"var"`
	Value           types.String `tfsdk:"value"`
	Type            types.String `tfsdk:"type"`
	Comment         types.String `tfsdk:"comment"`
	Enabled         types.Bool   `tfsdk:"enabled"`
	UpdateInitramfs types.Bool   `tfsdk:"update_initramfs"`
	// Computed only
	OrigValue types.String `tfsdk:"orig_value"`
}

// TunableDataSourceModel is TunableModel without the write-only update_initramfs field.
type TunableDataSourceModel struct {
	ID        types.Int64  `tfsdk:"id"`
	Var       types.String `tfsdk:"var"`
	Value     types.String `tfsdk:"value"`
	Type      types.String `tfsdk:"type"`
	Comment   types.String `tfsdk:"comment"`
	Enabled   types.Bool   `tfsdk:"enabled"`
	OrigValue types.String `tfsdk:"orig_value"`
}

// tunableAPI is the JSON wire format for a TrueNAS tunable object.
type tunableAPI struct {
	ID        int64  `json:"id"`
	Var       string `json:"var"`
	Value     string `json:"value"`
	Type      string `json:"type"`
	Comment   string `json:"comment"`
	Enabled   bool   `json:"enabled"`
	OrigValue string `json:"orig_value"`
}

// responseToModel maps an API response onto a Terraform model.
// UpdateInitramfs is NOT set here (write-only, never echoed back by the API).
func responseToModel(_ context.Context, api *tunableAPI, m *TunableModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Var = types.StringValue(api.Var)
	m.Value = types.StringValue(api.Value)
	m.Type = types.StringValue(api.Type)
	m.Comment = types.StringValue(api.Comment)
	m.Enabled = types.BoolValue(api.Enabled)
	m.OrigValue = types.StringValue(api.OrigValue)

	// NOTE: UpdateInitramfs is NOT set here (write-only).

	return diags
}

// responseToDataSourceModel maps an API response onto a TunableDataSourceModel.
func responseToDataSourceModel(_ context.Context, api *tunableAPI, m *TunableDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Var = types.StringValue(api.Var)
	m.Value = types.StringValue(api.Value)
	m.Type = types.StringValue(api.Type)
	m.Comment = types.StringValue(api.Comment)
	m.Enabled = types.BoolValue(api.Enabled)
	m.OrigValue = types.StringValue(api.OrigValue)

	return diags
}

// createPayload includes var and value (always required for initial creation),
// plus type/comment/enabled/update_initramfs when known.
func (m *TunableModel) createPayload(_ context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	p := map[string]any{
		"var":   m.Var.ValueString(),
		"value": m.Value.ValueString(),
	}

	if !m.Type.IsNull() && !m.Type.IsUnknown() {
		p["type"] = m.Type.ValueString()
	}
	if !m.Comment.IsNull() && !m.Comment.IsUnknown() {
		p["comment"] = m.Comment.ValueString()
	}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		p["enabled"] = m.Enabled.ValueBool()
	}
	if !m.UpdateInitramfs.IsNull() && !m.UpdateInitramfs.IsUnknown() {
		p["update_initramfs"] = m.UpdateInitramfs.ValueBool()
	}

	return p, diags
}

// updatePayload includes value (always) plus comment/enabled/update_initramfs
// when known. var and type are immutable and are NEVER sent on update.
func (m *TunableModel) updatePayload(_ context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	p := map[string]any{
		"value": m.Value.ValueString(),
	}

	if !m.Comment.IsNull() && !m.Comment.IsUnknown() {
		p["comment"] = m.Comment.ValueString()
	}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		p["enabled"] = m.Enabled.ValueBool()
	}
	if !m.UpdateInitramfs.IsNull() && !m.UpdateInitramfs.IsUnknown() {
		p["update_initramfs"] = m.UpdateInitramfs.ValueBool()
	}

	return p, diags
}

// decodeCreateResult decodes the result of the tunable.create job, which may
// be returned by the middleware either as the full created tunable object or
// as a bare integer ID. It returns either a populated *tunableAPI (object
// shape) or a non-zero id (bare-int shape), never both.
func decodeCreateResult(raw json.RawMessage) (*tunableAPI, int64, error) {
	var api tunableAPI
	if err := json.Unmarshal(raw, &api); err == nil && api.ID != 0 {
		return &api, 0, nil
	}

	var id int64
	if err := json.Unmarshal(raw, &id); err == nil {
		return nil, id, nil
	}

	return nil, 0, fmt.Errorf("unable to decode tunable.create result: %s", string(raw))
}

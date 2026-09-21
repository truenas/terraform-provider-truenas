// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_extent

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ISCSIExtentModel is the Terraform state model for truenas_iscsi_extent.
type ISCSIExtentModel struct {
	ID             types.Int64  `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Type           types.String `tfsdk:"type"`     // DISK or FILE
	Disk           types.String `tfsdk:"disk"`     // Optional: zvol path e.g. "zvol/tank/myvol"
	Path           types.String `tfsdk:"path"`     // Optional: file path (FILE type)
	Filesize       types.Int64  `tfsdk:"filesize"` // FILE type: size in bytes; 0 = unset
	Comment        types.String `tfsdk:"comment"`
	Blocksize      types.Int64  `tfsdk:"blocksize"` // 512, 1024, 2048, 4096
	PBlocksize     types.Bool   `tfsdk:"pblocksize"`
	AvailThreshold types.Int64  `tfsdk:"avail_threshold"` // 0 = unset (maps from nil)
	InsecureTPC    types.Bool   `tfsdk:"insecure_tpc"`
	Xen            types.Bool   `tfsdk:"xen"`
	ReadOnly       types.Bool   `tfsdk:"ro"`
	RPM            types.String `tfsdk:"rpm"` // SSD, 5400, 7200, 10000, 15000
	Enabled        types.Bool   `tfsdk:"enabled"`
	// Computed only
	NAA       types.String `tfsdk:"naa"`
	Serial    types.String `tfsdk:"serial"`
	ProductID types.String `tfsdk:"product_id"`
	Vendor    types.String `tfsdk:"vendor"`
	Locked    types.Bool   `tfsdk:"locked"`
}

// extentAPI is the JSON wire format for a TrueNAS iSCSI extent object.
type extentAPI struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	Disk           *string `json:"disk"`
	Path           string  `json:"path"`
	Filesize       any     `json:"filesize"` // API returns string or integer bytes
	Comment        string  `json:"comment"`
	Blocksize      int64   `json:"blocksize"`
	PBlocksize     bool    `json:"pblocksize"`
	AvailThreshold *int64  `json:"avail_threshold"`
	InsecureTPC    bool    `json:"insecure_tpc"`
	Xen            bool    `json:"xen"`
	ReadOnly       bool    `json:"ro"`
	RPM            string  `json:"rpm"`
	Enabled        bool    `json:"enabled"`
	NAA            string  `json:"naa"`
	Serial         string  `json:"serial"`
	ProductID      string  `json:"product_id"`
	Vendor         string  `json:"vendor"`
	Locked         bool    `json:"locked"`
}

// responseToModel maps an API response onto a Terraform model.
func responseToModel(_ context.Context, api *extentAPI, m *ISCSIExtentModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.Type = types.StringValue(api.Type)
	m.Comment = types.StringValue(api.Comment)
	m.Blocksize = types.Int64Value(api.Blocksize)
	m.PBlocksize = types.BoolValue(api.PBlocksize)
	m.InsecureTPC = types.BoolValue(api.InsecureTPC)
	m.Xen = types.BoolValue(api.Xen)
	m.ReadOnly = types.BoolValue(api.ReadOnly)
	m.RPM = types.StringValue(api.RPM)
	m.Enabled = types.BoolValue(api.Enabled)
	m.NAA = types.StringValue(api.NAA)
	m.Serial = types.StringValue(api.Serial)
	m.ProductID = types.StringValue(api.ProductID)
	m.Vendor = types.StringValue(api.Vendor)
	m.Locked = types.BoolValue(api.Locked)
	m.Path = types.StringValue(api.Path)

	if api.Disk != nil {
		m.Disk = types.StringValue(*api.Disk)
	} else {
		m.Disk = types.StringValue("")
	}

	if api.AvailThreshold != nil {
		m.AvailThreshold = types.Int64Value(*api.AvailThreshold)
	} else {
		m.AvailThreshold = types.Int64Value(0)
	}

	m.Filesize = types.Int64Value(coerceInt64(api.Filesize))

	return diags
}

// coerceInt64 converts the API's filesize field, which may arrive as a JSON
// number (float64 after decode) or a numeric string, into an int64. Anything
// unparseable yields 0.
func coerceInt64(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case string:
		n, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return 0
		}
		return n
	default:
		return 0
	}
}

// apiPayload builds the map[string]any payload for iscsi.extent.create /
// update. name/type/disk/path are required (or type-dependent) and are
// always sent. Every other field is Optional+Computed in the schema, so it
// is only included when set (neither null nor unknown) in the model — the
// zero Go value for these types (0, "", false) is not a valid TrueNAS
// value for several of them (e.g. blocksize must be 512/1024/2048/4096,
// rpm must be one of a fixed enum), so sending it unconditionally on
// create/update sends an invalid value whenever the field is left unset.
func (m *ISCSIExtentModel) apiPayload(_ context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	// avail_threshold already three-way (nil pointer omits it semantically
	// via `0 = unset`): 0 sends nil, non-zero sends the value.
	var avail *int64
	if v := m.AvailThreshold.ValueInt64(); v != 0 {
		avail = &v
	}

	payload := map[string]any{
		"name":            m.Name.ValueString(),
		"type":            m.Type.ValueString(),
		"avail_threshold": avail,
	}

	if !m.Comment.IsNull() && !m.Comment.IsUnknown() {
		payload["comment"] = m.Comment.ValueString()
	}
	if !m.Blocksize.IsNull() && !m.Blocksize.IsUnknown() {
		payload["blocksize"] = m.Blocksize.ValueInt64()
	}
	if !m.PBlocksize.IsNull() && !m.PBlocksize.IsUnknown() {
		payload["pblocksize"] = m.PBlocksize.ValueBool()
	}
	if !m.InsecureTPC.IsNull() && !m.InsecureTPC.IsUnknown() {
		payload["insecure_tpc"] = m.InsecureTPC.ValueBool()
	}
	if !m.Xen.IsNull() && !m.Xen.IsUnknown() {
		payload["xen"] = m.Xen.ValueBool()
	}
	if !m.ReadOnly.IsNull() && !m.ReadOnly.IsUnknown() {
		payload["ro"] = m.ReadOnly.ValueBool()
	}
	if !m.RPM.IsNull() && !m.RPM.IsUnknown() {
		payload["rpm"] = m.RPM.ValueString()
	}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		payload["enabled"] = m.Enabled.ValueBool()
	}

	if m.Type.ValueString() == "DISK" {
		payload["disk"] = m.Disk.ValueString()
	} else {
		payload["path"] = m.Path.ValueString()
		// filesize applies to FILE extents only; send it when the user set a
		// non-zero size (0 = unset, mirroring avail_threshold above).
		if v := m.Filesize.ValueInt64(); v != 0 {
			payload["filesize"] = v
		}
	}

	return payload, diags
}

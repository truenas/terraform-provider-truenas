package iscsi_extent

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ISCSIExtentModel is the Terraform state model for truenas_iscsi_extent.
type ISCSIExtentModel struct {
	ID             types.Int64  `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Type           types.String `tfsdk:"type"` // DISK or FILE
	Disk           types.String `tfsdk:"disk"` // Optional: zvol path e.g. "zvol/tank/myvol"
	Path           types.String `tfsdk:"path"` // Optional: file path (FILE type)
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

	return diags
}

// apiPayload builds the map[string]any payload for iscsi.extent.create / update.
func (m *ISCSIExtentModel) apiPayload(_ context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	var avail *int64
	if v := m.AvailThreshold.ValueInt64(); v != 0 {
		avail = &v
	}

	payload := map[string]any{
		"name":            m.Name.ValueString(),
		"type":            m.Type.ValueString(),
		"comment":         m.Comment.ValueString(),
		"blocksize":       m.Blocksize.ValueInt64(),
		"pblocksize":      m.PBlocksize.ValueBool(),
		"avail_threshold": avail,
		"insecure_tpc":    m.InsecureTPC.ValueBool(),
		"xen":             m.Xen.ValueBool(),
		"ro":              m.ReadOnly.ValueBool(),
		"rpm":             m.RPM.ValueString(),
		"enabled":         m.Enabled.ValueBool(),
	}

	if m.Type.ValueString() == "DISK" {
		payload["disk"] = m.Disk.ValueString()
	} else {
		payload["path"] = m.Path.ValueString()
	}

	return payload, diags
}

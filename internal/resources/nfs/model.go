package nfs

import "github.com/hashicorp/terraform-plugin-framework/types"

type NFSShareModel struct {
	ID       types.String `tfsdk:"id"`
	Path     types.String `tfsdk:"path"`
	Comment  types.String `tfsdk:"comment"`
	Enabled  types.Bool   `tfsdk:"enabled"`
	ReadOnly types.Bool   `tfsdk:"ro"`
	Networks types.List   `tfsdk:"networks"` // list of CIDR strings
	Hosts    types.List   `tfsdk:"hosts"`    // list of hostnames/IPs
	MapRoot  types.String `tfsdk:"maproot_user"`
	MapGroup types.String `tfsdk:"maproot_group"`
}

type apiResponse struct {
	ID       int64    `json:"id"`
	Path     string   `json:"path"`
	Comment  string   `json:"comment"`
	Enabled  bool     `json:"enabled"`
	ReadOnly bool     `json:"ro"`
	Networks []string `json:"networks"`
	Hosts    []string `json:"hosts"`
	MapRoot  string   `json:"maproot_user"`
	MapGroup string   `json:"maproot_group"`
}

func (m *NFSShareModel) apiPayload() map[string]any {
	p := map[string]any{"path": m.Path.ValueString()}
	if !m.Comment.IsNull() && !m.Comment.IsUnknown() {
		p["comment"] = m.Comment.ValueString()
	}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		p["enabled"] = m.Enabled.ValueBool()
	}
	if !m.ReadOnly.IsNull() && !m.ReadOnly.IsUnknown() {
		p["ro"] = m.ReadOnly.ValueBool()
	}
	if !m.MapRoot.IsNull() && !m.MapRoot.IsUnknown() {
		p["maproot_user"] = m.MapRoot.ValueString()
	}
	if !m.MapGroup.IsNull() && !m.MapGroup.IsUnknown() {
		p["maproot_group"] = m.MapGroup.ValueString()
	}
	return p
}

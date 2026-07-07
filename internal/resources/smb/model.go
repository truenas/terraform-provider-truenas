package smb

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// SMBModel is the Terraform state model for truenas_smb_share.
type SMBModel struct {
	ID               types.Int64  `tfsdk:"id"`
	Path             types.String `tfsdk:"path"`
	Name             types.String `tfsdk:"name"`
	Comment          types.String `tfsdk:"comment"`
	ReadOnly         types.Bool   `tfsdk:"ro"`
	Browsable        types.Bool   `tfsdk:"browsable"`
	Recyclebin       types.Bool   `tfsdk:"recyclebin"`
	GuestOK          types.Bool   `tfsdk:"guestok"`
	HostsAllow       types.List   `tfsdk:"hostsallow"` // List[String]
	HostsDeny        types.List   `tfsdk:"hostsdeny"`  // List[String]
	ABE              types.Bool   `tfsdk:"abe"`
	ACL              types.Bool   `tfsdk:"acl"`
	DurableHandle    types.Bool   `tfsdk:"durablehandle"`
	Streams          types.Bool   `tfsdk:"streams"`
	TimeMachine      types.Bool   `tfsdk:"timemachine"`
	TimeMachineQuota types.Int64  `tfsdk:"timemachine_quota"`
	Enabled          types.Bool   `tfsdk:"enabled"`
	Home             types.Bool   `tfsdk:"home"`
	Purpose          types.String `tfsdk:"purpose"`
	// Computed-only — server generated
	VUID   types.String `tfsdk:"vuid"`
	Locked types.Bool   `tfsdk:"locked"`
}

// smbAPI is the JSON wire format for a TrueNAS SMB share object.
type smbAPI struct {
	ID               int64    `json:"id"`
	Path             string   `json:"path"`
	Name             string   `json:"name"`
	Comment          string   `json:"comment"`
	ReadOnly         bool     `json:"ro"`
	Browsable        bool     `json:"browsable"`
	Recyclebin       bool     `json:"recyclebin"`
	GuestOK          bool     `json:"guestok"`
	HostsAllow       []string `json:"hostsallow"`
	HostsDeny        []string `json:"hostsdeny"`
	ABE              bool     `json:"abe"`
	ACL              bool     `json:"acl"`
	DurableHandle    bool     `json:"durablehandle"`
	Streams          bool     `json:"streams"`
	TimeMachine      bool     `json:"timemachine"`
	TimeMachineQuota int64    `json:"timemachine_quota"`
	Enabled          bool     `json:"enabled"`
	Home             bool     `json:"home"`
	Purpose          string   `json:"purpose"`
	VUID             string   `json:"vuid"`
	Locked           bool     `json:"locked"`
}

// responseToModel maps an API response onto a Terraform model. It does NOT set
// write-only fields (e.g. passwords). hostsallow/hostsdeny are handled here
// using types.ListValueFrom because they require a context.
func responseToModel(ctx context.Context, api *smbAPI, m *SMBModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Path = types.StringValue(api.Path)
	m.Name = types.StringValue(api.Name)
	m.Comment = types.StringValue(api.Comment)
	m.ReadOnly = types.BoolValue(api.ReadOnly)
	m.Browsable = types.BoolValue(api.Browsable)
	m.Recyclebin = types.BoolValue(api.Recyclebin)
	m.GuestOK = types.BoolValue(api.GuestOK)
	m.ABE = types.BoolValue(api.ABE)
	m.ACL = types.BoolValue(api.ACL)
	m.DurableHandle = types.BoolValue(api.DurableHandle)
	m.Streams = types.BoolValue(api.Streams)
	m.TimeMachine = types.BoolValue(api.TimeMachine)
	m.TimeMachineQuota = types.Int64Value(api.TimeMachineQuota)
	m.Enabled = types.BoolValue(api.Enabled)
	m.Home = types.BoolValue(api.Home)
	m.Purpose = types.StringValue(api.Purpose)
	m.VUID = types.StringValue(api.VUID)
	m.Locked = types.BoolValue(api.Locked)

	hostsAllow := api.HostsAllow
	if hostsAllow == nil {
		hostsAllow = []string{}
	}
	haList, d := types.ListValueFrom(ctx, types.StringType, hostsAllow)
	diags.Append(d...)
	m.HostsAllow = haList

	hostsDeny := api.HostsDeny
	if hostsDeny == nil {
		hostsDeny = []string{}
	}
	hdList, d := types.ListValueFrom(ctx, types.StringType, hostsDeny)
	diags.Append(d...)
	m.HostsDeny = hdList

	return diags
}

// apiPayload builds the map[string]any payload for sharing.smb.create /
// sharing.smb.update. It includes 18 keys plus purpose when set to a known
// value (19 total); vuid and locked are omitted because they are
// server-generated.
func (m *SMBModel) apiPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	var hostsAllow []string
	if !m.HostsAllow.IsNull() && !m.HostsAllow.IsUnknown() {
		diags.Append(m.HostsAllow.ElementsAs(ctx, &hostsAllow, false)...)
	}
	if hostsAllow == nil {
		hostsAllow = []string{}
	}

	var hostsDeny []string
	if !m.HostsDeny.IsNull() && !m.HostsDeny.IsUnknown() {
		diags.Append(m.HostsDeny.ElementsAs(ctx, &hostsDeny, false)...)
	}
	if hostsDeny == nil {
		hostsDeny = []string{}
	}

	p := map[string]any{
		"path":              m.Path.ValueString(),
		"name":              m.Name.ValueString(),
		"comment":           m.Comment.ValueString(),
		"ro":                m.ReadOnly.ValueBool(),
		"browsable":         m.Browsable.ValueBool(),
		"recyclebin":        m.Recyclebin.ValueBool(),
		"guestok":           m.GuestOK.ValueBool(),
		"hostsallow":        hostsAllow,
		"hostsdeny":         hostsDeny,
		"abe":               m.ABE.ValueBool(),
		"acl":               m.ACL.ValueBool(),
		"durablehandle":     m.DurableHandle.ValueBool(),
		"streams":           m.Streams.ValueBool(),
		"timemachine":       m.TimeMachine.ValueBool(),
		"timemachine_quota": m.TimeMachineQuota.ValueInt64(),
		"enabled":           m.Enabled.ValueBool(),
		"home":              m.Home.ValueBool(),
	}

	// purpose: SCALE 26.0 requires purpose to be one of validSMBPurposes
	// (or omitted entirely) — sending "" unconditionally triggers EINVAL.
	// Only include it when it's set to a known, non-empty value.
	if v := m.Purpose.ValueString(); !m.Purpose.IsNull() && !m.Purpose.IsUnknown() && validSMBPurposes[v] {
		p["purpose"] = v
	}

	return p, diags
}

// validSMBPurposes is the set of purpose values accepted by TrueNAS SCALE
// 26.0's sharing.smb.create/update. Older 24.x values (NO_PRESET,
// ENHANCED_TIMEMACHINE, MULTI_PROTOCOL_AFP, MULTI_PROTOCOL_NFS,
// PRIVATE_DATASETS, WORM_DROPBOX, etc.) no longer exist on 26.0.
var validSMBPurposes = map[string]bool{
	"DEFAULT_SHARE":          true,
	"LEGACY_SHARE":           true,
	"TIMEMACHINE_SHARE":      true,
	"MULTIPROTOCOL_SHARE":    true,
	"TIME_LOCKED_SHARE":      true,
	"PRIVATE_DATASETS_SHARE": true,
	"EXTERNAL_SHARE":         true,
	"VEEAM_REPOSITORY_SHARE": true,
	"FCP_SHARE":              true,
}

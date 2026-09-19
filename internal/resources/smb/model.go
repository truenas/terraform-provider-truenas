// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// SMBAuditModel maps to the nested "audit" object.
type SMBAuditModel struct {
	Enable     types.Bool `tfsdk:"enable"`
	WatchList  types.List `tfsdk:"watch_list"`  // List[String] group names
	IgnoreList types.List `tfsdk:"ignore_list"` // List[String] group names
}

// smbAuditAttrTypes is the attribute type map for SMBAuditModel.
var smbAuditAttrTypes = map[string]attr.Type{
	"enable":      types.BoolType,
	"watch_list":  types.ListType{ElemType: types.StringType},
	"ignore_list": types.ListType{ElemType: types.StringType},
}

// SMBModel is the Terraform state model for truenas_smb_share. The schema
// stays flat for backward compatibility with existing configs; the wire
// payload/response mapping to TrueNAS 26.0's nested purpose/options shape
// happens in apiPayload and responseToModel below.
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
	Audit            types.Object `tfsdk:"audit"` // nested {enable, watch_list, ignore_list}
	// Computed-only — server generated
	VUID   types.String `tfsdk:"vuid"`
	Locked types.Bool   `tfsdk:"locked"`
}

// smbOptionsAPI is the JSON wire format for the discriminated-union `options`
// object returned/accepted by sharing.smb.create/update/query on TrueNAS 26.0.
// Only the LEGACY_SHARE variant fields that this provider models are
// represented here; other purpose variants (DefaultOpt, TimeMachineOpt, etc.)
// carry fields this resource does not track and are read via Purpose alone.
type smbOptionsAPI struct {
	Purpose          string   `json:"purpose"`
	Recyclebin       bool     `json:"recyclebin"`
	HostsAllow       []string `json:"hostsallow"`
	HostsDeny        []string `json:"hostsdeny"`
	GuestOK          bool     `json:"guestok"`
	Streams          bool     `json:"streams"`
	DurableHandle    bool     `json:"durablehandle"`
	Home             bool     `json:"home"`
	ACL              bool     `json:"acl"`
	TimeMachine      bool     `json:"timemachine"`
	TimeMachineQuota int64    `json:"timemachine_quota"`
	VUID             *string  `json:"vuid"`
}

// smbAPI is the JSON wire format for a TrueNAS SMB share object on TrueNAS
// 26.0: legacy flags that used to live at the top level now live nested
// under `options` when the share's purpose is LEGACY_SHARE.
type smbAPI struct {
	ID        int64          `json:"id"`
	Path      string         `json:"path"`
	Name      string         `json:"name"`
	Comment   string         `json:"comment"`
	ReadOnly  bool           `json:"readonly"`
	Browsable bool           `json:"browsable"`
	ABE       bool           `json:"access_based_share_enumeration"`
	Enabled   bool           `json:"enabled"`
	Purpose   string         `json:"purpose"`
	Locked    *bool          `json:"locked"`
	Options   *smbOptionsAPI `json:"options"`
	Audit     *struct {
		Enable     bool     `json:"enable"`
		WatchList  []string `json:"watch_list"`
		IgnoreList []string `json:"ignore_list"`
	} `json:"audit"`
}

// legacySharePurpose is the purpose value used to preserve the old flat
// (24.x-style) share behavior. It is the default applied when the caller
// has not set (or has set an unrecognized) purpose, and is the only purpose
// variant for which this resource's legacy boolean/list flags are sent.
const legacySharePurpose = "LEGACY_SHARE"

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
	m.ABE = types.BoolValue(api.ABE)
	m.Enabled = types.BoolValue(api.Enabled)
	m.Purpose = types.StringValue(api.Purpose)

	// locked is required (bool|null) in the 26.0 schema; null just means lock
	// status wasn't requested/available, so fall back to the zero value.
	if api.Locked != nil {
		m.Locked = types.BoolValue(*api.Locked)
	} else {
		m.Locked = types.BoolValue(false)
	}

	// Legacy flags now live nested under options, and only when the share's
	// purpose is LEGACY_SHARE. NOTE: the discriminator lives at the TOP level
	// of the response — the options object itself carries no "purpose" key
	// (verified live on TrueNAS 26.0) — so branch on api.Purpose, not
	// opts.Purpose. For any other purpose, the legacy fields this resource
	// models don't apply server-side; leave them at their zero values.
	opts := api.Options
	if opts != nil && api.Purpose == legacySharePurpose {
		m.Recyclebin = types.BoolValue(opts.Recyclebin)
		m.GuestOK = types.BoolValue(opts.GuestOK)
		m.ACL = types.BoolValue(opts.ACL)
		m.DurableHandle = types.BoolValue(opts.DurableHandle)
		m.Streams = types.BoolValue(opts.Streams)
		m.TimeMachine = types.BoolValue(opts.TimeMachine)
		m.TimeMachineQuota = types.Int64Value(opts.TimeMachineQuota)
		m.Home = types.BoolValue(opts.Home)
		if opts.VUID != nil {
			m.VUID = types.StringValue(*opts.VUID)
		} else {
			m.VUID = types.StringValue("")
		}

		hostsAllow := opts.HostsAllow
		if hostsAllow == nil {
			hostsAllow = []string{}
		}
		haList, d := types.ListValueFrom(ctx, types.StringType, hostsAllow)
		diags.Append(d...)
		m.HostsAllow = haList

		hostsDeny := opts.HostsDeny
		if hostsDeny == nil {
			hostsDeny = []string{}
		}
		hdList, d := types.ListValueFrom(ctx, types.StringType, hostsDeny)
		diags.Append(d...)
		m.HostsDeny = hdList
	} else {
		m.Recyclebin = types.BoolValue(false)
		m.GuestOK = types.BoolValue(false)
		m.ACL = types.BoolValue(false)
		m.DurableHandle = types.BoolValue(false)
		m.Streams = types.BoolValue(false)
		m.TimeMachine = types.BoolValue(false)
		m.TimeMachineQuota = types.Int64Value(0)
		m.Home = types.BoolValue(false)
		m.VUID = types.StringValue("")

		emptyList, d := types.ListValueFrom(ctx, types.StringType, []string{})
		diags.Append(d...)
		m.HostsAllow = emptyList
		m.HostsDeny = emptyList
	}

	// audit is a top-level object the API always returns (default enable=false,
	// empty lists). Build it unconditionally so the Optional+Computed attribute
	// always has a concrete value.
	enable := false
	watch := []string{}
	ignore := []string{}
	if api.Audit != nil {
		enable = api.Audit.Enable
		if api.Audit.WatchList != nil {
			watch = api.Audit.WatchList
		}
		if api.Audit.IgnoreList != nil {
			ignore = api.Audit.IgnoreList
		}
	}
	watchList, dw := types.ListValueFrom(ctx, types.StringType, watch)
	diags.Append(dw...)
	ignoreList, di := types.ListValueFrom(ctx, types.StringType, ignore)
	diags.Append(di...)
	auditObj, da := types.ObjectValueFrom(ctx, smbAuditAttrTypes, SMBAuditModel{
		Enable:     types.BoolValue(enable),
		WatchList:  watchList,
		IgnoreList: ignoreList,
	})
	diags.Append(da...)
	m.Audit = auditObj

	return diags
}

// apiPayload builds the map[string]any payload for sharing.smb.create /
// sharing.smb.update, targeting TrueNAS 26.0's wire format: top-level
// path/name/comment/enabled/browsable/readonly/access_based_share_enumeration
// /purpose, plus a nested `options` object. `options` always carries
// `purpose`; when purpose is (or defaults to) LEGACY_SHARE it also carries
// this resource's legacy flags (each included only when set in config, to
// avoid clobbering server defaults with zero values). For any other purpose,
// options contains purpose only — the legacy flags are not sent anywhere,
// since they don't apply to that variant.
func (m *SMBModel) apiPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	// purpose: TrueNAS 26.0 requires purpose to be one of validSMBPurposes (or
	// omitted, which defaults server-side to DEFAULT_SHARE). This resource
	// preserves its historical flat/legacy behavior by defaulting to
	// LEGACY_SHARE whenever the caller hasn't set a recognized 26.0 purpose.
	purpose := legacySharePurpose
	if v := m.Purpose.ValueString(); !m.Purpose.IsNull() && !m.Purpose.IsUnknown() && validSMBPurposes[v] {
		purpose = v
	}

	p := map[string]any{
		"path":                           m.Path.ValueString(),
		"name":                           m.Name.ValueString(),
		"comment":                        m.Comment.ValueString(),
		"enabled":                        m.Enabled.ValueBool(),
		"browsable":                      m.Browsable.ValueBool(),
		"readonly":                       m.ReadOnly.ValueBool(),
		"access_based_share_enumeration": m.ABE.ValueBool(),
		"purpose":                        purpose,
	}

	options := map[string]any{
		"purpose": purpose,
	}

	if purpose == legacySharePurpose {
		if !m.Recyclebin.IsNull() && !m.Recyclebin.IsUnknown() {
			options["recyclebin"] = m.Recyclebin.ValueBool()
		}
		if !m.GuestOK.IsNull() && !m.GuestOK.IsUnknown() {
			options["guestok"] = m.GuestOK.ValueBool()
		}
		if !m.Streams.IsNull() && !m.Streams.IsUnknown() {
			options["streams"] = m.Streams.ValueBool()
		}
		if !m.DurableHandle.IsNull() && !m.DurableHandle.IsUnknown() {
			options["durablehandle"] = m.DurableHandle.ValueBool()
		}
		if !m.Home.IsNull() && !m.Home.IsUnknown() {
			options["home"] = m.Home.ValueBool()
		}
		if !m.ACL.IsNull() && !m.ACL.IsUnknown() {
			options["acl"] = m.ACL.ValueBool()
		}
		if !m.TimeMachine.IsNull() && !m.TimeMachine.IsUnknown() {
			options["timemachine"] = m.TimeMachine.ValueBool()
		}
		if !m.TimeMachineQuota.IsNull() && !m.TimeMachineQuota.IsUnknown() {
			options["timemachine_quota"] = m.TimeMachineQuota.ValueInt64()
		}

		if !m.HostsAllow.IsNull() && !m.HostsAllow.IsUnknown() {
			var hostsAllow []string
			diags.Append(m.HostsAllow.ElementsAs(ctx, &hostsAllow, false)...)
			if hostsAllow == nil {
				hostsAllow = []string{}
			}
			options["hostsallow"] = hostsAllow
		}
		if !m.HostsDeny.IsNull() && !m.HostsDeny.IsUnknown() {
			var hostsDeny []string
			diags.Append(m.HostsDeny.ElementsAs(ctx, &hostsDeny, false)...)
			if hostsDeny == nil {
				hostsDeny = []string{}
			}
			options["hostsdeny"] = hostsDeny
		}
	}

	p["options"] = options

	// audit is a top-level object (not nested under options). Send it when the
	// user set it; each sub-field is included only when known.
	if !m.Audit.IsNull() && !m.Audit.IsUnknown() {
		var a SMBAuditModel
		diags.Append(m.Audit.As(ctx, &a, basetypes.ObjectAsOptions{})...)
		audit := map[string]any{}
		if !a.Enable.IsNull() && !a.Enable.IsUnknown() {
			audit["enable"] = a.Enable.ValueBool()
		}
		if !a.WatchList.IsNull() && !a.WatchList.IsUnknown() {
			var wl []string
			diags.Append(a.WatchList.ElementsAs(ctx, &wl, false)...)
			if wl == nil {
				wl = []string{}
			}
			audit["watch_list"] = wl
		}
		if !a.IgnoreList.IsNull() && !a.IgnoreList.IsUnknown() {
			var il []string
			diags.Append(a.IgnoreList.ElementsAs(ctx, &il, false)...)
			if il == nil {
				il = []string{}
			}
			audit["ignore_list"] = il
		}
		p["audit"] = audit
	}

	return p, diags
}

// validSMBPurposes is the set of purpose values accepted by TrueNAS
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

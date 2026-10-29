// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb

import (
	"context"
	"sort"

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

// SMBOptionsModel is the typed `options` object: the superset of every
// purpose-specific field TrueNAS 26.0/27.0 carries under sharing.smb's
// discriminated-union `options`. Only the subset valid for the share's
// `purpose` is sent on the wire (see smbOptionFieldsByPurpose); the rest read
// back null. Field sets verified identical on 25.10.7 and 27.0.
type SMBOptionsModel struct {
	Recyclebin          types.Bool   `tfsdk:"recyclebin"`
	PathSuffix          types.String `tfsdk:"path_suffix"`
	HostsAllow          types.List   `tfsdk:"hostsallow"`
	HostsDeny           types.List   `tfsdk:"hostsdeny"`
	GuestOK             types.Bool   `tfsdk:"guestok"`
	Streams             types.Bool   `tfsdk:"streams"`
	DurableHandle       types.Bool   `tfsdk:"durablehandle"`
	Shadowcopy          types.Bool   `tfsdk:"shadowcopy"`
	FSRVP               types.Bool   `tfsdk:"fsrvp"`
	Home                types.Bool   `tfsdk:"home"`
	ACL                 types.Bool   `tfsdk:"acl"`
	AFP                 types.Bool   `tfsdk:"afp"`
	TimeMachine         types.Bool   `tfsdk:"timemachine"`
	TimeMachineQuota    types.Int64  `tfsdk:"timemachine_quota"`
	AaplNameMangling    types.Bool   `tfsdk:"aapl_name_mangling"`
	VUID                types.String `tfsdk:"vuid"`
	AuxSMBConf          types.String `tfsdk:"auxsmbconf"`
	AutoSnapshot        types.Bool   `tfsdk:"auto_snapshot"`
	AutoDatasetCreation types.Bool   `tfsdk:"auto_dataset_creation"`
	DatasetNamingSchema types.String `tfsdk:"dataset_naming_schema"`
	GracePeriod         types.Int64  `tfsdk:"grace_period"`
	AutoQuota           types.Int64  `tfsdk:"auto_quota"`
	RemotePath          types.List   `tfsdk:"remote_path"`
}

var smbOptionsAttrTypes = map[string]attr.Type{
	"recyclebin":            types.BoolType,
	"path_suffix":           types.StringType,
	"hostsallow":            types.ListType{ElemType: types.StringType},
	"hostsdeny":             types.ListType{ElemType: types.StringType},
	"guestok":               types.BoolType,
	"streams":               types.BoolType,
	"durablehandle":         types.BoolType,
	"shadowcopy":            types.BoolType,
	"fsrvp":                 types.BoolType,
	"home":                  types.BoolType,
	"acl":                   types.BoolType,
	"afp":                   types.BoolType,
	"timemachine":           types.BoolType,
	"timemachine_quota":     types.Int64Type,
	"aapl_name_mangling":    types.BoolType,
	"vuid":                  types.StringType,
	"auxsmbconf":            types.StringType,
	"auto_snapshot":         types.BoolType,
	"auto_dataset_creation": types.BoolType,
	"dataset_naming_schema": types.StringType,
	"grace_period":          types.Int64Type,
	"auto_quota":            types.Int64Type,
	"remote_path":           types.ListType{ElemType: types.StringType},
}

// smbOptionFieldsByPurpose lists which options fields each purpose's variant
// accepts on the wire (probed live on 25.10.7 and 27.0). apiPayload sends only
// the intersection of these and what the user set, so we never send a field the
// chosen variant rejects.
var smbOptionFieldsByPurpose = map[string]map[string]bool{
	"LEGACY_SHARE": {
		"recyclebin": true, "path_suffix": true, "hostsallow": true, "hostsdeny": true,
		"guestok": true, "streams": true, "durablehandle": true, "shadowcopy": true,
		"fsrvp": true, "home": true, "acl": true, "afp": true, "timemachine": true,
		"timemachine_quota": true, "aapl_name_mangling": true, "vuid": true, "auxsmbconf": true,
	},
	"DEFAULT_SHARE":          {"aapl_name_mangling": true, "hostsallow": true, "hostsdeny": true},
	"MULTIPROTOCOL_SHARE":    {"aapl_name_mangling": true, "hostsallow": true, "hostsdeny": true},
	"FCP_SHARE":              {"aapl_name_mangling": true, "hostsallow": true, "hostsdeny": true},
	"VEEAM_REPOSITORY_SHARE": {"hostsallow": true, "hostsdeny": true},
	"TIMEMACHINE_SHARE": {
		"timemachine_quota": true, "auto_snapshot": true, "auto_dataset_creation": true,
		"dataset_naming_schema": true, "vuid": true, "hostsallow": true, "hostsdeny": true,
	},
	"TIME_LOCKED_SHARE": {"grace_period": true, "aapl_name_mangling": true, "hostsallow": true, "hostsdeny": true},
	"PRIVATE_DATASETS_SHARE": {
		"dataset_naming_schema": true, "auto_quota": true, "aapl_name_mangling": true,
		"hostsallow": true, "hostsdeny": true,
	},
	"EXTERNAL_SHARE": {"remote_path": true},
}

// SMBModel is the Terraform state model for truenas_smb_share. The flat legacy
// attributes are retained for backward compatibility (they map to the
// LEGACY_SHARE variant); the typed `options` object exposes the full
// purpose-specific surface for every purpose.
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
	Options          types.Object `tfsdk:"options"` // typed superset, all purposes
	Audit            types.Object `tfsdk:"audit"`   // nested {enable, watch_list, ignore_list}
	// Computed-only — server generated
	VUID   types.String `tfsdk:"vuid"`
	Locked types.Bool   `tfsdk:"locked"`
}

// smbOptionsAPI is the JSON wire format for the discriminated-union `options`
// object. All fields are pointers/nilable so an absent field (one not part of
// the current purpose's variant) is distinguished from a zero value.
type smbOptionsAPI struct {
	Recyclebin          *bool     `json:"recyclebin,omitempty"`
	PathSuffix          *string   `json:"path_suffix,omitempty"`
	HostsAllow          *[]string `json:"hostsallow,omitempty"`
	HostsDeny           *[]string `json:"hostsdeny,omitempty"`
	GuestOK             *bool     `json:"guestok,omitempty"`
	Streams             *bool     `json:"streams,omitempty"`
	DurableHandle       *bool     `json:"durablehandle,omitempty"`
	Shadowcopy          *bool     `json:"shadowcopy,omitempty"`
	FSRVP               *bool     `json:"fsrvp,omitempty"`
	Home                *bool     `json:"home,omitempty"`
	ACL                 *bool     `json:"acl,omitempty"`
	AFP                 *bool     `json:"afp,omitempty"`
	TimeMachine         *bool     `json:"timemachine,omitempty"`
	TimeMachineQuota    *int64    `json:"timemachine_quota,omitempty"`
	AaplNameMangling    *bool     `json:"aapl_name_mangling,omitempty"`
	VUID                *string   `json:"vuid,omitempty"`
	AuxSMBConf          *string   `json:"auxsmbconf,omitempty"`
	AutoSnapshot        *bool     `json:"auto_snapshot,omitempty"`
	AutoDatasetCreation *bool     `json:"auto_dataset_creation,omitempty"`
	DatasetNamingSchema *string   `json:"dataset_naming_schema,omitempty"`
	GracePeriod         *int64    `json:"grace_period,omitempty"`
	AutoQuota           *int64    `json:"auto_quota,omitempty"`
	RemotePath          *[]string `json:"remote_path,omitempty"`
}

// smbAPI is the JSON wire format for a TrueNAS SMB share object. The
// discriminator `purpose` lives at the top level; `options` carries the
// purpose's variant fields.
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
// has not set (or has set an unrecognized) purpose.
const legacySharePurpose = "LEGACY_SHARE"

func boolPtrValue(p *bool) types.Bool {
	if p == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*p)
}

func int64PtrValue(p *int64) types.Int64 {
	if p == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*p)
}

func strPtrValue(p *string) types.String {
	if p == nil {
		return types.StringNull()
	}
	return types.StringValue(*p)
}

func strListPtrValue(ctx context.Context, p *[]string, diags *diag.Diagnostics) types.List {
	if p == nil {
		return types.ListNull(types.StringType)
	}
	v := *p
	if v == nil {
		v = []string{}
	}
	l, d := types.ListValueFrom(ctx, types.StringType, v)
	diags.Append(d...)
	return l
}

// optionsToObject maps the wire options onto the typed SMBOptionsModel object.
// Fields the wire didn't carry (not part of this purpose's variant) read null.
func optionsToObject(ctx context.Context, opts *smbOptionsAPI, diags *diag.Diagnostics) types.Object {
	if opts == nil {
		return types.ObjectNull(smbOptionsAttrTypes)
	}
	m := SMBOptionsModel{
		Recyclebin:          boolPtrValue(opts.Recyclebin),
		PathSuffix:          strPtrValue(opts.PathSuffix),
		HostsAllow:          strListPtrValue(ctx, opts.HostsAllow, diags),
		HostsDeny:           strListPtrValue(ctx, opts.HostsDeny, diags),
		GuestOK:             boolPtrValue(opts.GuestOK),
		Streams:             boolPtrValue(opts.Streams),
		DurableHandle:       boolPtrValue(opts.DurableHandle),
		Shadowcopy:          boolPtrValue(opts.Shadowcopy),
		FSRVP:               boolPtrValue(opts.FSRVP),
		Home:                boolPtrValue(opts.Home),
		ACL:                 boolPtrValue(opts.ACL),
		AFP:                 boolPtrValue(opts.AFP),
		TimeMachine:         boolPtrValue(opts.TimeMachine),
		TimeMachineQuota:    int64PtrValue(opts.TimeMachineQuota),
		AaplNameMangling:    boolPtrValue(opts.AaplNameMangling),
		VUID:                strPtrValue(opts.VUID),
		AuxSMBConf:          strPtrValue(opts.AuxSMBConf),
		AutoSnapshot:        boolPtrValue(opts.AutoSnapshot),
		AutoDatasetCreation: boolPtrValue(opts.AutoDatasetCreation),
		DatasetNamingSchema: strPtrValue(opts.DatasetNamingSchema),
		GracePeriod:         int64PtrValue(opts.GracePeriod),
		AutoQuota:           int64PtrValue(opts.AutoQuota),
		RemotePath:          strListPtrValue(ctx, opts.RemotePath, diags),
	}
	obj, d := types.ObjectValueFrom(ctx, smbOptionsAttrTypes, m)
	diags.Append(d...)
	return obj
}

// responseToModel maps an API response onto a Terraform model. It does NOT set
// write-only fields. The typed `options` object is populated for EVERY purpose
// (so drift is always visible); the flat legacy attributes remain populated for
// LEGACY_SHARE for backward compatibility.
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

	if api.Locked != nil {
		m.Locked = types.BoolValue(*api.Locked)
	} else {
		m.Locked = types.BoolValue(false)
	}

	// Typed options: reflect exactly what the server has, for every purpose.
	m.Options = optionsToObject(ctx, api.Options, &diags)

	// Flat legacy attributes: kept for backward compatibility, populated from
	// options when the share is a LEGACY_SHARE (their historical behavior).
	opts := api.Options
	if opts != nil && api.Purpose == legacySharePurpose {
		m.Recyclebin = boolOrFalse(opts.Recyclebin)
		m.GuestOK = boolOrFalse(opts.GuestOK)
		m.ACL = boolOrFalse(opts.ACL)
		m.DurableHandle = boolOrFalse(opts.DurableHandle)
		m.Streams = boolOrFalse(opts.Streams)
		m.TimeMachine = boolOrFalse(opts.TimeMachine)
		m.TimeMachineQuota = int64OrZero(opts.TimeMachineQuota)
		m.Home = boolOrFalse(opts.Home)
		m.VUID = strOrEmpty(opts.VUID)
		m.HostsAllow = strListOrEmpty(ctx, opts.HostsAllow, &diags)
		m.HostsDeny = strListOrEmpty(ctx, opts.HostsDeny, &diags)
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
		empty, d := types.ListValueFrom(ctx, types.StringType, []string{})
		diags.Append(d...)
		m.HostsAllow = empty
		m.HostsDeny = empty
	}

	// audit — top-level object always returned.
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

func boolOrFalse(p *bool) types.Bool {
	if p == nil {
		return types.BoolValue(false)
	}
	return types.BoolValue(*p)
}
func int64OrZero(p *int64) types.Int64 {
	if p == nil {
		return types.Int64Value(0)
	}
	return types.Int64Value(*p)
}
func strOrEmpty(p *string) types.String {
	if p == nil {
		return types.StringValue("")
	}
	return types.StringValue(*p)
}
func strListOrEmpty(ctx context.Context, p *[]string, diags *diag.Diagnostics) types.List {
	v := []string{}
	if p != nil && *p != nil {
		v = *p
	}
	l, d := types.ListValueFrom(ctx, types.StringType, v)
	diags.Append(d...)
	return l
}

// apiPayload builds the sharing.smb.create/update payload. The options object
// carries `purpose` plus the subset of fields valid for that purpose that the
// user actually set. The typed `options` block is authoritative; for
// LEGACY_SHARE, a flat legacy attribute is used as a fallback when the
// corresponding options field is unset (backward compatibility).
func (m *SMBModel) apiPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	purpose := legacySharePurpose
	if v := m.Purpose.ValueString(); !m.Purpose.IsNull() && !m.Purpose.IsUnknown() && validSMBPurposes[v] {
		purpose = v
	}

	p := map[string]any{
		"path":    m.Path.ValueString(),
		"name":    m.Name.ValueString(),
		"purpose": purpose,
	}
	// Optional+Computed top-level fields are sent only when known: an unset
	// value is planned unknown, and sending its Go zero value would create the
	// share disabled and hidden (server defaults: enabled and browsable true),
	// or overwrite the current value on update.
	if !m.Comment.IsNull() && !m.Comment.IsUnknown() {
		p["comment"] = m.Comment.ValueString()
	}
	for key, v := range map[string]types.Bool{
		"enabled":                        m.Enabled,
		"browsable":                      m.Browsable,
		"readonly":                       m.ReadOnly,
		"access_based_share_enumeration": m.ABE,
	} {
		if !v.IsNull() && !v.IsUnknown() {
			p[key] = v.ValueBool()
		}
	}

	valid := smbOptionFieldsByPurpose[purpose]
	options := map[string]any{"purpose": purpose}

	// Source 1: the typed options block (authoritative).
	var opt SMBOptionsModel
	haveOpt := !m.Options.IsNull() && !m.Options.IsUnknown()
	if haveOpt {
		diags.Append(m.Options.As(ctx, &opt, basetypes.ObjectAsOptions{})...)
	}
	setBool := func(key string, v types.Bool) {
		if valid[key] && !v.IsNull() && !v.IsUnknown() {
			options[key] = v.ValueBool()
		}
	}
	setInt := func(key string, v types.Int64) {
		if valid[key] && !v.IsNull() && !v.IsUnknown() {
			options[key] = v.ValueInt64()
		}
	}
	setStr := func(key string, v types.String) {
		if valid[key] && !v.IsNull() && !v.IsUnknown() {
			options[key] = v.ValueString()
		}
	}
	setList := func(key string, v types.List) {
		if valid[key] && !v.IsNull() && !v.IsUnknown() {
			var xs []string
			diags.Append(v.ElementsAs(ctx, &xs, false)...)
			if xs == nil {
				xs = []string{}
			}
			options[key] = xs
		}
	}
	if haveOpt {
		setBool("recyclebin", opt.Recyclebin)
		setStr("path_suffix", opt.PathSuffix)
		setList("hostsallow", opt.HostsAllow)
		setList("hostsdeny", opt.HostsDeny)
		setBool("guestok", opt.GuestOK)
		setBool("streams", opt.Streams)
		setBool("durablehandle", opt.DurableHandle)
		setBool("shadowcopy", opt.Shadowcopy)
		setBool("fsrvp", opt.FSRVP)
		setBool("home", opt.Home)
		setBool("acl", opt.ACL)
		setBool("afp", opt.AFP)
		setBool("timemachine", opt.TimeMachine)
		setInt("timemachine_quota", opt.TimeMachineQuota)
		setBool("aapl_name_mangling", opt.AaplNameMangling)
		setStr("vuid", opt.VUID)
		setStr("auxsmbconf", opt.AuxSMBConf)
		setBool("auto_snapshot", opt.AutoSnapshot)
		setBool("auto_dataset_creation", opt.AutoDatasetCreation)
		setStr("dataset_naming_schema", opt.DatasetNamingSchema)
		setInt("grace_period", opt.GracePeriod)
		setInt("auto_quota", opt.AutoQuota)
		setList("remote_path", opt.RemotePath)
	}

	// Source 2 (LEGACY only): flat legacy attributes, as a fallback for any
	// field the options block didn't set. Preserves pre-options configs.
	if purpose == legacySharePurpose {
		fallbackBool := func(key string, v types.Bool) {
			if _, ok := options[key]; !ok && valid[key] && !v.IsNull() && !v.IsUnknown() {
				options[key] = v.ValueBool()
			}
		}
		fallbackInt := func(key string, v types.Int64) {
			if _, ok := options[key]; !ok && valid[key] && !v.IsNull() && !v.IsUnknown() {
				options[key] = v.ValueInt64()
			}
		}
		fallbackList := func(key string, v types.List) {
			if _, ok := options[key]; !ok && valid[key] && !v.IsNull() && !v.IsUnknown() {
				var xs []string
				diags.Append(v.ElementsAs(ctx, &xs, false)...)
				if xs == nil {
					xs = []string{}
				}
				options[key] = xs
			}
		}
		fallbackBool("recyclebin", m.Recyclebin)
		fallbackBool("guestok", m.GuestOK)
		fallbackBool("streams", m.Streams)
		fallbackBool("durablehandle", m.DurableHandle)
		fallbackBool("home", m.Home)
		fallbackBool("acl", m.ACL)
		fallbackBool("timemachine", m.TimeMachine)
		fallbackInt("timemachine_quota", m.TimeMachineQuota)
		fallbackList("hostsallow", m.HostsAllow)
		fallbackList("hostsdeny", m.HostsDeny)
	}

	p["options"] = options

	// audit — top-level object.
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

// validSMBPurposes is the set of purpose values accepted by TrueNAS 26.0+'s
// sharing.smb.create/update.
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

// resolveEffectivePurpose returns the purpose the provider will actually send,
// mirroring apiPayload: LEGACY_SHARE unless a recognized purpose is set.
func resolveEffectivePurpose(m *SMBModel) string {
	if v := m.Purpose.ValueString(); !m.Purpose.IsNull() && !m.Purpose.IsUnknown() && validSMBPurposes[v] {
		return v
	}
	return legacySharePurpose
}

// sortedValidOptions returns the option field names valid for a purpose, sorted.
func sortedValidOptions(purpose string) []string {
	keys := make([]string, 0, len(smbOptionFieldsByPurpose[purpose]))
	for k := range smbOptionFieldsByPurpose[purpose] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// invalidOptionKeys returns the wire keys of any options fields that are set in
// the config but not valid for the effective purpose. Such fields are silently
// dropped by apiPayload; ValidateConfig surfaces them as plan-time errors so a
// misfiled or copy-pasted option (e.g. recyclebin on a TIMEMACHINE_SHARE) is
// caught instead of ignored. Returns (purpose, badKeys). Skips validation when
// purpose or options are unknown (interpolated).
func (m *SMBModel) invalidOptionKeys(ctx context.Context) (string, []string, diag.Diagnostics) {
	var diags diag.Diagnostics
	purpose := resolveEffectivePurpose(m)
	if m.Purpose.IsUnknown() || m.Options.IsNull() || m.Options.IsUnknown() {
		return purpose, nil, diags
	}
	valid := smbOptionFieldsByPurpose[purpose]

	var opt SMBOptionsModel
	diags.Append(m.Options.As(ctx, &opt, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return purpose, nil, diags
	}

	set := setOptionKeys(&opt)
	var bad []string
	for _, key := range set {
		if !valid[key] {
			bad = append(bad, key)
		}
	}
	return purpose, bad, diags
}

// setOptionKeys returns the wire keys of the options fields that are set
// (known, non-null) in opt, in schema order.
func setOptionKeys(opt *SMBOptionsModel) []string {
	setB := func(v types.Bool) bool { return !v.IsNull() && !v.IsUnknown() }
	setI := func(v types.Int64) bool { return !v.IsNull() && !v.IsUnknown() }
	setS := func(v types.String) bool { return !v.IsNull() && !v.IsUnknown() }
	setL := func(v types.List) bool { return !v.IsNull() && !v.IsUnknown() }

	type field struct {
		key string
		set bool
	}
	fields := []field{
		{"recyclebin", setB(opt.Recyclebin)},
		{"path_suffix", setS(opt.PathSuffix)},
		{"hostsallow", setL(opt.HostsAllow)},
		{"hostsdeny", setL(opt.HostsDeny)},
		{"guestok", setB(opt.GuestOK)},
		{"streams", setB(opt.Streams)},
		{"durablehandle", setB(opt.DurableHandle)},
		{"shadowcopy", setB(opt.Shadowcopy)},
		{"fsrvp", setB(opt.FSRVP)},
		{"home", setB(opt.Home)},
		{"acl", setB(opt.ACL)},
		{"afp", setB(opt.AFP)},
		{"timemachine", setB(opt.TimeMachine)},
		{"timemachine_quota", setI(opt.TimeMachineQuota)},
		{"aapl_name_mangling", setB(opt.AaplNameMangling)},
		{"vuid", setS(opt.VUID)},
		{"auxsmbconf", setS(opt.AuxSMBConf)},
		{"auto_snapshot", setB(opt.AutoSnapshot)},
		{"auto_dataset_creation", setB(opt.AutoDatasetCreation)},
		{"dataset_naming_schema", setS(opt.DatasetNamingSchema)},
		{"grace_period", setI(opt.GracePeriod)},
		{"auto_quota", setI(opt.AutoQuota)},
		{"remote_path", setL(opt.RemotePath)},
	}
	var keys []string
	for _, f := range fields {
		if f.set {
			keys = append(keys, f.key)
		}
	}
	return keys
}

// flatLegacyAttr is one flat top-level attribute that mirrors an
// options.<key> field of the LEGACY_SHARE variant.
type flatLegacyAttr struct {
	key   string     // wire key under options (same as the attribute name)
	value attr.Value // the flat attribute's value in the model
}

// flatLegacyAttrs returns the flat legacy attributes of m, in schema order.
// Every key is also an options field name.
func (m *SMBModel) flatLegacyAttrs() []flatLegacyAttr {
	return []flatLegacyAttr{
		{"recyclebin", m.Recyclebin},
		{"guestok", m.GuestOK},
		{"hostsallow", m.HostsAllow},
		{"hostsdeny", m.HostsDeny},
		{"acl", m.ACL},
		{"durablehandle", m.DurableHandle},
		{"streams", m.Streams},
		{"timemachine", m.TimeMachine},
		{"timemachine_quota", m.TimeMachineQuota},
		{"home", m.Home},
	}
}

// configuredFlatLegacyKeys returns the keys of the flat legacy attributes that
// are set (non-null, known or unknown) in m. Used on the config model.
func (m *SMBModel) configuredFlatLegacyKeys() []string {
	var keys []string
	for _, f := range m.flatLegacyAttrs() {
		if !f.value.IsNull() {
			keys = append(keys, f.key)
		}
	}
	return keys
}

// overlayFlatOnOptions returns the plan's options object with every known,
// non-null flat legacy attribute copied onto the matching options field, and
// whether that changed anything. sharing.smb.update replaces the whole options
// object (unsent fields reset to their defaults), so the payload must carry
// the full object; when options is not configured it is pinned from state,
// and without this overlay the stale pinned values would win over a changed
// flat attribute (B-011). Requires m.Options to be known and non-null.
func (m *SMBModel) overlayFlatOnOptions(ctx context.Context) (types.Object, bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	var opt SMBOptionsModel
	diags.Append(m.Options.As(ctx, &opt, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return m.Options, false, diags
	}
	known := func(v attr.Value) bool { return !v.IsNull() && !v.IsUnknown() }
	if known(m.Recyclebin) {
		opt.Recyclebin = m.Recyclebin
	}
	if known(m.GuestOK) {
		opt.GuestOK = m.GuestOK
	}
	if known(m.HostsAllow) {
		opt.HostsAllow = m.HostsAllow
	}
	if known(m.HostsDeny) {
		opt.HostsDeny = m.HostsDeny
	}
	if known(m.ACL) {
		opt.ACL = m.ACL
	}
	if known(m.DurableHandle) {
		opt.DurableHandle = m.DurableHandle
	}
	if known(m.Streams) {
		opt.Streams = m.Streams
	}
	if known(m.TimeMachine) {
		opt.TimeMachine = m.TimeMachine
	}
	if known(m.TimeMachineQuota) {
		opt.TimeMachineQuota = m.TimeMachineQuota
	}
	if known(m.Home) {
		opt.Home = m.Home
	}
	obj, d := types.ObjectValueFrom(ctx, smbOptionsAttrTypes, opt)
	diags.Append(d...)
	if diags.HasError() {
		return m.Options, false, diags
	}
	return obj, !obj.Equal(m.Options), diags
}

// resetUnsetFlatLegacy sets every flat legacy attribute that cfg leaves unset
// to its LEGACY_SHARE server default (the same on 25.10, 26.0 and 28.0) and
// returns the attribute names it set. Removing a flat attribute from the
// configuration then resets it on the share instead of keeping the old value
// with an empty plan (B-113). Only used on a LEGACY_SHARE update without a
// configured options object.
func (m *SMBModel) resetUnsetFlatLegacy(cfg *SMBModel) []string {
	empty := types.ListValueMust(types.StringType, []attr.Value{})
	var set []string
	reset := func(key string, cfgV attr.Value, dst any, def attr.Value) {
		if !cfgV.IsNull() {
			return
		}
		switch d := dst.(type) {
		case *types.Bool:
			*d = def.(types.Bool)
		case *types.List:
			*d = def.(types.List)
		case *types.Int64:
			*d = def.(types.Int64)
		}
		set = append(set, key)
	}
	reset("recyclebin", cfg.Recyclebin, &m.Recyclebin, types.BoolValue(false))
	reset("guestok", cfg.GuestOK, &m.GuestOK, types.BoolValue(false))
	reset("hostsallow", cfg.HostsAllow, &m.HostsAllow, empty)
	reset("hostsdeny", cfg.HostsDeny, &m.HostsDeny, empty)
	reset("acl", cfg.ACL, &m.ACL, types.BoolValue(true))
	reset("durablehandle", cfg.DurableHandle, &m.DurableHandle, types.BoolValue(true))
	reset("streams", cfg.Streams, &m.Streams, types.BoolValue(true))
	reset("timemachine", cfg.TimeMachine, &m.TimeMachine, types.BoolValue(false))
	reset("timemachine_quota", cfg.TimeMachineQuota, &m.TimeMachineQuota, types.Int64Value(0))
	reset("home", cfg.Home, &m.Home, types.BoolValue(false))
	return set
}

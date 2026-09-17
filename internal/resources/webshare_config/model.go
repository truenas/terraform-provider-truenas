// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package webshare_config

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// webshareConfigResourceID is the fixed Terraform ID for this singleton
// resource: there is exactly one Webshare service configuration per TrueNAS
// system (webshare.config always returns a single record), and it is never
// created or deleted on TrueNAS itself. The API's own numeric "id" (probed
// live on TrueNAS 26.0: always 1) is an internal implementation detail and is
// intentionally not surfaced in the model, mirroring the
// lxc_config/docker_config/audit_config singleton pattern.
const webshareConfigResourceID = "webshare_config"

// webshareConfigVersionFloorMajor/Minor is the minimum TrueNAS release
// that exposes the webshare.config/webshare.update namespace at all. Probed
// live: TrueNAS 25.10 returns 0 webshare.* and sharing.webshare.* methods from
// core.get_methods (namespace genuinely absent); TrueNAS 26.0 supports it in
// full (webshare.config, webshare.update, webshare.bindip_choices, all
// job:false). See resource.go/datasource.go for where this gates every
// Create/Read/Update/datasource-Read before any webshare.* call reaches the
// wire.
const (
	webshareConfigVersionFloorMajor = 26
	webshareConfigVersionFloorMinor = 0
)

// versionGateDiagnostics reports whether the given TrueNAS release string
// (as returned by system.version_short via client.ServerVersion, e.g.
// "25.10.3.1" or "26.0.0") is at or above the TrueNAS 26.0 floor
// webshare.config/webshare.update require, returning a single clean error
// diagnostic when it is not (instead of letting a raw "Method does not
// exist" JSON-RPC error reach the practitioner). It is a pure function of
// the already-probed version string — no live client access — matching the
// lxc_config/container precedent so it stays independently unit-testable.
func versionGateDiagnostics(version string) diag.Diagnostics {
	var diags diag.Diagnostics
	if !client.VersionAtLeastString(version, webshareConfigVersionFloorMajor, webshareConfigVersionFloorMinor) {
		diags.AddError(
			"TrueNAS version too old",
			"truenas_webshare_config requires TrueNAS 26.0 or later",
		)
	}
	return diags
}

// WebshareConfigModel is the Terraform state model for
// truenas_webshare_config.
//
// None of webshare.config's fields are nullable at the API level (probed
// live: bindip/groups are always arrays — [] when empty, never null;
// search/passkey are always present with a concrete value) so, unlike
// lxc_config's PreferredPool/Bridge, no three-way nullable convention is
// needed here — every field uses the plain Optional+Computed
// omit-when-unknown guard (see updatePayload).
type WebshareConfigModel struct {
	ID      types.String `tfsdk:"id"`
	BindIP  types.List   `tfsdk:"bindip"` // List[String]
	Search  types.Bool   `tfsdk:"search"`
	Passkey types.String `tfsdk:"passkey"`
	Groups  types.List   `tfsdk:"groups"` // List[String]
}

// WebshareConfigDataSourceModel is the read-only model for the
// truenas_webshare_config datasource.
type WebshareConfigDataSourceModel struct {
	ID      types.String `tfsdk:"id"`
	BindIP  types.List   `tfsdk:"bindip"`
	Search  types.Bool   `tfsdk:"search"`
	Passkey types.String `tfsdk:"passkey"`
	Groups  types.List   `tfsdk:"groups"`
}

// webshareConfigAPI mirrors the JSON object returned by webshare.config and
// webshare.update. Probed live against TrueNAS 26.0 (the only release that
// exposes this namespace — see versionGateDiagnostics):
//
//	{"id": 1, "bindip": [], "search": false, "passkey": "DISABLED", "groups": []}
type webshareConfigAPI struct {
	ID      int64    `json:"id"`
	BindIP  []string `json:"bindip"`
	Search  bool     `json:"search"`
	Passkey string   `json:"passkey"`
	Groups  []string `json:"groups"`
}

// nonNilStrings returns s unchanged, or an empty (non-nil) slice, so
// types.ListValueFrom always produces a known empty list rather than a null
// one when the API returns an empty/absent array (probed: bindip/groups
// default to [], never omitted or null).
func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// responseToModel maps a webshareConfigAPI response onto a
// WebshareConfigModel.
func responseToModel(ctx context.Context, api *webshareConfigAPI, m *WebshareConfigModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(webshareConfigResourceID)

	bindip, d := types.ListValueFrom(ctx, types.StringType, nonNilStrings(api.BindIP))
	diags.Append(d...)
	m.BindIP = bindip

	m.Search = types.BoolValue(api.Search)
	m.Passkey = types.StringValue(api.Passkey)

	groups, d := types.ListValueFrom(ctx, types.StringType, nonNilStrings(api.Groups))
	diags.Append(d...)
	m.Groups = groups

	return diags
}

// responseToDataSourceModel maps a webshareConfigAPI response onto a
// WebshareConfigDataSourceModel.
func responseToDataSourceModel(ctx context.Context, api *webshareConfigAPI, m *WebshareConfigDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(webshareConfigResourceID)

	bindip, d := types.ListValueFrom(ctx, types.StringType, nonNilStrings(api.BindIP))
	diags.Append(d...)
	m.BindIP = bindip

	m.Search = types.BoolValue(api.Search)
	m.Passkey = types.StringValue(api.Passkey)

	groups, d := types.ListValueFrom(ctx, types.StringType, nonNilStrings(api.Groups))
	diags.Append(d...)
	m.Groups = groups

	return diags
}

// updatePayload builds the map[string]any payload for webshare.update.
// Every field is a plain Optional+Computed omit-when-unknown guard (see the
// WebshareConfigModel doc comment for why no three-way nullable handling is
// needed): webshare.update's own accepts schema treats every field as a
// genuine partial update (no "default" on any accepted field, probed live —
// omitting a field leaves its current TrueNAS-side value unchanged, unlike
// e.g. ups.update which requires several fields on every call).
//
// CALLERS MUST invoke this on a model populated from req.Config
// (req.Config.Get), never from req.Plan. "bindip", "search", "passkey", and
// "groups" are Optional+Computed with UseStateForUnknown plan modifiers:
// for any of them left unset by the user, the *plan* value is not
// null — the modifier copies the prior state's value into the plan so
// Terraform can show a stable diff. Building the payload from the plan
// would therefore resend a field's last-known value on every Create/Update
// even when the user never configured it, silently reasserting it against
// whatever the box's current value happens to be (a revert race if that
// value changed since the last read). req.Config, by contrast, stays null
// for anything the user did not set in HCL regardless of plan modifiers,
// so inclusion here is driven strictly by what the user explicitly
// configured.
func (m *WebshareConfigModel) updatePayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{}

	if !m.BindIP.IsNull() && !m.BindIP.IsUnknown() {
		var bindip []string
		diags.Append(m.BindIP.ElementsAs(ctx, &bindip, false)...)
		if bindip == nil {
			bindip = []string{}
		}
		p["bindip"] = bindip
	}
	if !m.Search.IsNull() && !m.Search.IsUnknown() {
		p["search"] = m.Search.ValueBool()
	}
	if !m.Passkey.IsNull() && !m.Passkey.IsUnknown() {
		p["passkey"] = m.Passkey.ValueString()
	}
	if !m.Groups.IsNull() && !m.Groups.IsUnknown() {
		var groups []string
		diags.Append(m.Groups.ElementsAs(ctx, &groups, false)...)
		if groups == nil {
			groups = []string{}
		}
		p["groups"] = groups
	}

	return p, diags
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by Delete.
// Delete makes NO client calls: webshare.config is a system-wide service
// singleton (authentication mode, allowed groups, bind addresses) that
// exists independently of Terraform and must never be reset just because
// the Terraform resource is removed from state — mirrors the
// lxc_config/docker_config/audit_config precedent exactly.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"Webshare configuration left in place",
		"Webshare configuration left in place; removed from Terraform state only",
	)
	return diags
}

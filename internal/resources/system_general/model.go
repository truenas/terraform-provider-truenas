// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_general

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// systemGeneralResourceID is the fixed Terraform ID for this singleton
// resource: there is exactly one system general configuration per TrueNAS
// system, and it is never created or deleted on TrueNAS itself.
const systemGeneralResourceID = "system_general"

// SystemGeneralModel is the Terraform state model for
// truenas_system_general.
type SystemGeneralModel struct {
	ID               types.String `tfsdk:"id"` // fixed: "system_general"
	DSAuth           types.Bool   `tfsdk:"ds_auth"`
	Kbdmap           types.String `tfsdk:"kbdmap"`
	Timezone         types.String `tfsdk:"timezone"`
	UIAddress        types.List   `tfsdk:"ui_address"`     // List[String]
	UIAllowlist      types.List   `tfsdk:"ui_allowlist"`   // List[String]
	UICertificate    types.Int64  `tfsdk:"ui_certificate"` // nullable
	UIConsolemsg     types.Bool   `tfsdk:"ui_consolemsg"`
	UIHTTPSPort      types.Int64  `tfsdk:"ui_httpsport"`
	UIHTTPSProtocols types.List   `tfsdk:"ui_httpsprotocols"` // List[String]
	UIHTTPSRedirect  types.Bool   `tfsdk:"ui_httpsredirect"`
	UIPort           types.Int64  `tfsdk:"ui_port"`
	UIV6Address      types.List   `tfsdk:"ui_v6address"` // List[String]
	UIXFrameOptions  types.String `tfsdk:"ui_x_frame_options"`
	UsageCollection  types.Bool   `tfsdk:"usage_collection"` // nullable

	// Computed-only: never sent to system.general.update.
	UICertificateName    types.String `tfsdk:"ui_certificate_name"`
	UsageCollectionIsSet types.Bool   `tfsdk:"usage_collection_is_set"`
	Wizardshown          types.Bool   `tfsdk:"wizardshown"`
}

// SystemGeneralDataSourceModel is the read-only model for the
// truenas_system_general datasource.
type SystemGeneralDataSourceModel struct {
	ID                   types.String `tfsdk:"id"`
	DSAuth               types.Bool   `tfsdk:"ds_auth"`
	Kbdmap               types.String `tfsdk:"kbdmap"`
	Timezone             types.String `tfsdk:"timezone"`
	UIAddress            types.List   `tfsdk:"ui_address"`
	UIAllowlist          types.List   `tfsdk:"ui_allowlist"`
	UICertificate        types.Int64  `tfsdk:"ui_certificate"`
	UIConsolemsg         types.Bool   `tfsdk:"ui_consolemsg"`
	UIHTTPSPort          types.Int64  `tfsdk:"ui_httpsport"`
	UIHTTPSProtocols     types.List   `tfsdk:"ui_httpsprotocols"`
	UIHTTPSRedirect      types.Bool   `tfsdk:"ui_httpsredirect"`
	UIPort               types.Int64  `tfsdk:"ui_port"`
	UIV6Address          types.List   `tfsdk:"ui_v6address"`
	UIXFrameOptions      types.String `tfsdk:"ui_x_frame_options"`
	UsageCollection      types.Bool   `tfsdk:"usage_collection"`
	UICertificateName    types.String `tfsdk:"ui_certificate_name"`
	UsageCollectionIsSet types.Bool   `tfsdk:"usage_collection_is_set"`
	Wizardshown          types.Bool   `tfsdk:"wizardshown"`
}

// uiCertificate accepts both wire shapes of ui_certificate in
// system.general.config: TrueNAS 26.0 returns the certificate ID as a bare
// integer (with the name in a separate top-level ui_certificate_name
// field), while 25.10 returns the full certificate object (and has no
// top-level name field). Both shapes decode to the id/name pair; a JSON
// null leaves both nil.
//
// A session without full-admin rights (a READONLY_ADMIN API key) gets the
// string "********" in place of the id (probed live on 28.0); that decodes to
// Redacted with no id, rather than failing the whole read (B-030).
type uiCertificate struct {
	ID       *int64
	Name     *string
	Redacted bool
}

// redactionMask is the value system.general.config returns in place of
// ui_certificate to a READONLY_ADMIN session.
const redactionMask = "********"

func (c *uiCertificate) UnmarshalJSON(b []byte) error {
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		return nil
	}
	var str string
	if err := json.Unmarshal(b, &str); err == nil {
		if str == redactionMask {
			c.Redacted = true
			return nil
		}
		return fmt.Errorf("unexpected ui_certificate string %q", str)
	}
	var id int64
	if err := json.Unmarshal(b, &id); err == nil {
		c.ID = &id
		return nil
	}
	var obj struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(b, &obj); err != nil {
		return err
	}
	c.ID = &obj.ID
	c.Name = &obj.Name
	return nil
}

// idValue maps the decoded certificate ID to a Terraform value: null when
// no certificate is set.
func (c uiCertificate) idValue() types.Int64 {
	if c.ID == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*c.ID)
}

// certificateName resolves the certificate name across wire shapes: 26.0's
// top-level ui_certificate_name wins, then 25.10's nested object name, then
// empty (no certificate set — matches the 26.0 wire, which reports "").
func (api *systemGeneralAPI) certificateName() types.String {
	switch {
	case api.UICertificateName != "":
		return types.StringValue(api.UICertificateName)
	case api.UICertificate.Name != nil:
		return types.StringValue(*api.UICertificate.Name)
	case api.UICertificate.Redacted:
		// 25.10 redacts the whole certificate object (name included) for a
		// READONLY_ADMIN session and has no top-level name: unknown, not "".
		return types.StringNull()
	default:
		return types.StringValue("")
	}
}

// systemGeneralAPI mirrors the JSON object returned by
// system.general.config and accepted (as a subset) by
// system.general.update. ui_certificate and usage_collection are nullable
// on the wire, so they are modeled as pointers; every other writable field
// is always present as a concrete value. ui_certificate_name,
// usage_collection_is_set, and wizardshown are server-computed and are
// never accepted by system.general.update.
type systemGeneralAPI struct {
	ID                   int64         `json:"id"`
	DSAuth               bool          `json:"ds_auth"`
	Kbdmap               string        `json:"kbdmap"`
	Timezone             string        `json:"timezone"`
	UIAddress            []string      `json:"ui_address"`
	UIAllowlist          []string      `json:"ui_allowlist"`
	UICertificate        uiCertificate `json:"ui_certificate"`
	UICertificateName    string        `json:"ui_certificate_name"`
	UIConsolemsg         bool          `json:"ui_consolemsg"`
	UIHTTPSPort          int64         `json:"ui_httpsport"`
	UIHTTPSProtocols     []string      `json:"ui_httpsprotocols"`
	UIHTTPSRedirect      bool          `json:"ui_httpsredirect"`
	UIPort               int64         `json:"ui_port"`
	UIV6Address          []string      `json:"ui_v6address"`
	UIXFrameOptions      string        `json:"ui_x_frame_options"`
	UsageCollection      *bool         `json:"usage_collection"`
	UsageCollectionIsSet bool          `json:"usage_collection_is_set"`
	Wizardshown          bool          `json:"wizardshown"`
}

// responseToModel maps an API response onto a Terraform model. Nil pointer
// fields (ui_certificate, usage_collection) map to null rather than a zero
// value, so that an update payload built from this model omits them (see
// updatePayload) instead of sending an explicit 0/false that overwrites a
// server-side null.
func responseToModel(ctx context.Context, api *systemGeneralAPI, m *SystemGeneralModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(systemGeneralResourceID)
	m.DSAuth = types.BoolValue(api.DSAuth)
	m.Kbdmap = types.StringValue(api.Kbdmap)
	m.Timezone = types.StringValue(api.Timezone)
	m.UIConsolemsg = types.BoolValue(api.UIConsolemsg)
	m.UIHTTPSPort = types.Int64Value(api.UIHTTPSPort)
	m.UIHTTPSRedirect = types.BoolValue(api.UIHTTPSRedirect)
	m.UIPort = types.Int64Value(api.UIPort)
	m.UIXFrameOptions = types.StringValue(api.UIXFrameOptions)
	if name := api.certificateName(); !name.IsNull() || m.UICertificateName.IsUnknown() {
		m.UICertificateName = name // a redacted name (25.10) keeps the prior value
	}
	m.UsageCollectionIsSet = types.BoolValue(api.UsageCollectionIsSet)
	m.Wizardshown = types.BoolValue(api.Wizardshown)

	// A READONLY_ADMIN session gets the redaction mask, not the id: keep the
	// prior value so a read-only refresh plans nothing (B-030).
	switch {
	case !api.UICertificate.Redacted:
		m.UICertificate = api.UICertificate.idValue()
	case m.UICertificate.IsUnknown():
		m.UICertificate = types.Int64Null()
	}

	if api.UsageCollection != nil {
		m.UsageCollection = types.BoolValue(*api.UsageCollection)
	} else {
		m.UsageCollection = types.BoolNull()
	}

	uiAddress := api.UIAddress
	if uiAddress == nil {
		uiAddress = []string{}
	}
	uiAddressList, d := types.ListValueFrom(ctx, types.StringType, uiAddress)
	diags.Append(d...)
	m.UIAddress = uiAddressList

	uiAllowlist := api.UIAllowlist
	if uiAllowlist == nil {
		uiAllowlist = []string{}
	}
	uiAllowlistList, d := types.ListValueFrom(ctx, types.StringType, uiAllowlist)
	diags.Append(d...)
	m.UIAllowlist = uiAllowlistList

	uiHTTPSProtocols := api.UIHTTPSProtocols
	if uiHTTPSProtocols == nil {
		uiHTTPSProtocols = []string{}
	}
	uiHTTPSProtocolsList, d := types.ListValueFrom(ctx, types.StringType, uiHTTPSProtocols)
	diags.Append(d...)
	m.UIHTTPSProtocols = uiHTTPSProtocolsList

	uiV6Address := api.UIV6Address
	if uiV6Address == nil {
		uiV6Address = []string{}
	}
	uiV6AddressList, d := types.ListValueFrom(ctx, types.StringType, uiV6Address)
	diags.Append(d...)
	m.UIV6Address = uiV6AddressList

	return diags
}

// responseToDataSourceModel maps an API response onto a
// SystemGeneralDataSourceModel using the same nil-pointer-to-null rules as
// responseToModel.
func responseToDataSourceModel(ctx context.Context, api *systemGeneralAPI, m *SystemGeneralDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(systemGeneralResourceID)
	m.DSAuth = types.BoolValue(api.DSAuth)
	m.Kbdmap = types.StringValue(api.Kbdmap)
	m.Timezone = types.StringValue(api.Timezone)
	m.UIConsolemsg = types.BoolValue(api.UIConsolemsg)
	m.UIHTTPSPort = types.Int64Value(api.UIHTTPSPort)
	m.UIHTTPSRedirect = types.BoolValue(api.UIHTTPSRedirect)
	m.UIPort = types.Int64Value(api.UIPort)
	m.UIXFrameOptions = types.StringValue(api.UIXFrameOptions)
	m.UICertificateName = api.certificateName()
	m.UsageCollectionIsSet = types.BoolValue(api.UsageCollectionIsSet)
	m.Wizardshown = types.BoolValue(api.Wizardshown)

	m.UICertificate = api.UICertificate.idValue()

	if api.UsageCollection != nil {
		m.UsageCollection = types.BoolValue(*api.UsageCollection)
	} else {
		m.UsageCollection = types.BoolNull()
	}

	uiAddress := api.UIAddress
	if uiAddress == nil {
		uiAddress = []string{}
	}
	uiAddressList, d := types.ListValueFrom(ctx, types.StringType, uiAddress)
	diags.Append(d...)
	m.UIAddress = uiAddressList

	uiAllowlist := api.UIAllowlist
	if uiAllowlist == nil {
		uiAllowlist = []string{}
	}
	uiAllowlistList, d := types.ListValueFrom(ctx, types.StringType, uiAllowlist)
	diags.Append(d...)
	m.UIAllowlist = uiAllowlistList

	uiHTTPSProtocols := api.UIHTTPSProtocols
	if uiHTTPSProtocols == nil {
		uiHTTPSProtocols = []string{}
	}
	uiHTTPSProtocolsList, d := types.ListValueFrom(ctx, types.StringType, uiHTTPSProtocols)
	diags.Append(d...)
	m.UIHTTPSProtocols = uiHTTPSProtocolsList

	uiV6Address := api.UIV6Address
	if uiV6Address == nil {
		uiV6Address = []string{}
	}
	uiV6AddressList, d := types.ListValueFrom(ctx, types.StringType, uiV6Address)
	diags.Append(d...)
	m.UIV6Address = uiV6AddressList

	return diags
}

// updatePayload builds the system.general.update argument.
//
// CALLERS MUST invoke this on a model populated from req.Config, never from
// req.Plan: every writable field is Optional+Computed with
// UseStateForUnknown, so the plan carries the prior state's value for a field
// the user never set, and sending it back is not harmless here. Re-sending
// usage_collection recorded an explicit opt-in (usage_collection_is_set
// flipped to true) on every unrelated update (B-143). Only what the user
// configured is sent; everything else is left as it is on the box.
//
// ui_certificate_name, usage_collection_is_set, and wizardshown are
// Computed-only and are NEVER included. Lists are normalized to an empty
// slice when nil so an explicitly-set-but-empty list clears the value on
// TrueNAS rather than being omitted.
func (m *SystemGeneralModel) updatePayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{}

	list := func(key string, l types.List) {
		if l.IsNull() || l.IsUnknown() {
			return
		}
		var v []string
		diags.Append(l.ElementsAs(ctx, &v, false)...)
		if v == nil {
			v = []string{}
		}
		p[key] = v
	}

	if !m.DSAuth.IsNull() && !m.DSAuth.IsUnknown() {
		p["ds_auth"] = m.DSAuth.ValueBool()
	}
	if !m.Kbdmap.IsNull() && !m.Kbdmap.IsUnknown() {
		p["kbdmap"] = m.Kbdmap.ValueString()
	}
	if !m.Timezone.IsNull() && !m.Timezone.IsUnknown() {
		p["timezone"] = m.Timezone.ValueString()
	}
	list("ui_address", m.UIAddress)
	list("ui_allowlist", m.UIAllowlist)
	if !m.UICertificate.IsNull() && !m.UICertificate.IsUnknown() {
		p["ui_certificate"] = m.UICertificate.ValueInt64()
	}
	if !m.UIConsolemsg.IsNull() && !m.UIConsolemsg.IsUnknown() {
		p["ui_consolemsg"] = m.UIConsolemsg.ValueBool()
	}
	if !m.UIHTTPSPort.IsNull() && !m.UIHTTPSPort.IsUnknown() {
		p["ui_httpsport"] = m.UIHTTPSPort.ValueInt64()
	}
	list("ui_httpsprotocols", m.UIHTTPSProtocols)
	if !m.UIHTTPSRedirect.IsNull() && !m.UIHTTPSRedirect.IsUnknown() {
		p["ui_httpsredirect"] = m.UIHTTPSRedirect.ValueBool()
	}
	if !m.UIPort.IsNull() && !m.UIPort.IsUnknown() {
		p["ui_port"] = m.UIPort.ValueInt64()
	}
	list("ui_v6address", m.UIV6Address)
	if !m.UIXFrameOptions.IsNull() && !m.UIXFrameOptions.IsUnknown() {
		p["ui_x_frame_options"] = m.UIXFrameOptions.ValueString()
	}
	if !m.UsageCollection.IsNull() && !m.UsageCollection.IsUnknown() {
		p["usage_collection"] = m.UsageCollection.ValueBool()
	}

	return p, diags
}

// webServerSettingsChanged reports whether payload changes any setting the
// web server (nginx) only picks up when it restarts: the bind addresses,
// ports, HTTPS redirect, TLS protocols, certificate, allowlist and
// X-Frame-Options. system.general.update saves these but does not apply them
// (B-027); the caller asks for a restart when this returns true.
// ui_consolemsg is a console setting and is not included.
func webServerSettingsChanged(current *systemGeneralAPI, payload map[string]any) bool {
	var currentCert any
	if current.UICertificate.ID != nil {
		currentCert = *current.UICertificate.ID
	}
	currentValues := map[string]any{
		"ui_address":         nonNil(current.UIAddress),
		"ui_v6address":       nonNil(current.UIV6Address),
		"ui_allowlist":       nonNil(current.UIAllowlist),
		"ui_httpsprotocols":  nonNil(current.UIHTTPSProtocols),
		"ui_port":            current.UIPort,
		"ui_httpsport":       current.UIHTTPSPort,
		"ui_httpsredirect":   current.UIHTTPSRedirect,
		"ui_x_frame_options": current.UIXFrameOptions,
		"ui_certificate":     currentCert,
	}
	for key, now := range currentValues {
		want, ok := payload[key]
		if !ok {
			continue
		}
		if current.UICertificate.Redacted && key == "ui_certificate" {
			return true
		}
		if !reflect.DeepEqual(want, now) {
			return true
		}
	}
	return false
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by Delete.
// Delete makes NO client calls: the system general configuration controls
// the management UI (network bind addresses, ports, certificate), so
// removing this resource from Terraform state must never rewrite the box's
// configuration and risk cutting off management access. Splitting this into
// its own function keeps Delete's "no client calls" contract independently
// unit-testable.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"System general configuration left in place",
		"System general configuration left in place; removed from Terraform state only",
	)
	return diags
}

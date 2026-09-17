// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package audit_config

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// auditConfigResourceID is the fixed Terraform ID for this singleton
// resource: there is exactly one audit configuration per TrueNAS system
// (audit.config always returns a single record), and it is never created or
// deleted on TrueNAS itself. The API's own numeric "id" (probed: always 1)
// is an internal implementation detail and is intentionally not surfaced in
// the model, mirroring the resilver_config singleton pattern.
const auditConfigResourceID = "audit_config"

// spaceAttrTypes describes the attribute types of the nested, read-only
// "space" object.
var spaceAttrTypes = map[string]attr.Type{
	"used":                types.Int64Type,
	"used_by_dataset":     types.Int64Type,
	"used_by_reservation": types.Int64Type,
	"used_by_snapshots":   types.Int64Type,
	"available":           types.Int64Type,
}

// enabledServicesAttrTypes describes the attribute types of the nested,
// read-only "enabled_services" object.
var enabledServicesAttrTypes = map[string]attr.Type{
	"middleware": types.ListType{ElemType: types.StringType},
	"smb":        types.ListType{ElemType: types.StringType},
	"sudo":       types.ListType{ElemType: types.StringType},
}

// SpaceModel maps to the nested, read-only "space" attribute: ZFS dataset
// space accounting for where the audit databases are stored.
type SpaceModel struct {
	Used              types.Int64 `tfsdk:"used"`
	UsedByDataset     types.Int64 `tfsdk:"used_by_dataset"`
	UsedByReservation types.Int64 `tfsdk:"used_by_reservation"`
	UsedBySnapshots   types.Int64 `tfsdk:"used_by_snapshots"`
	Available         types.Int64 `tfsdk:"available"`
}

// EnabledServicesModel maps to the nested, read-only "enabled_services"
// attribute: which audit event types/shares/commands are currently being
// audited per service. This is driven by other resources (SMB share audit
// settings, sudo config, etc), not by truenas_audit_config itself.
type EnabledServicesModel struct {
	Middleware types.List `tfsdk:"middleware"`
	Smb        types.List `tfsdk:"smb"`
	Sudo       types.List `tfsdk:"sudo"`
}

// AuditConfigModel is the Terraform state model for truenas_audit_config.
type AuditConfigModel struct {
	ID                   types.String `tfsdk:"id"`
	Retention            types.Int64  `tfsdk:"retention"`
	Reservation          types.Int64  `tfsdk:"reservation"`
	Quota                types.Int64  `tfsdk:"quota"`
	QuotaFillWarning     types.Int64  `tfsdk:"quota_fill_warning"`
	QuotaFillCritical    types.Int64  `tfsdk:"quota_fill_critical"`
	RemoteLoggingEnabled types.Bool   `tfsdk:"remote_logging_enabled"`
	Space                types.Object `tfsdk:"space"`
	EnabledServices      types.Object `tfsdk:"enabled_services"`
}

// AuditConfigDataSourceModel is the read-only model for the
// truenas_audit_config datasource.
type AuditConfigDataSourceModel struct {
	ID                   types.String `tfsdk:"id"`
	Retention            types.Int64  `tfsdk:"retention"`
	Reservation          types.Int64  `tfsdk:"reservation"`
	Quota                types.Int64  `tfsdk:"quota"`
	QuotaFillWarning     types.Int64  `tfsdk:"quota_fill_warning"`
	QuotaFillCritical    types.Int64  `tfsdk:"quota_fill_critical"`
	RemoteLoggingEnabled types.Bool   `tfsdk:"remote_logging_enabled"`
	Space                types.Object `tfsdk:"space"`
	EnabledServices      types.Object `tfsdk:"enabled_services"`
}

// spaceAPI mirrors the nested "space" object returned by audit.config and
// audit.update. Probed live: identical shape on TrueNAS 25.10 and 26.0.
type spaceAPI struct {
	Used              int64 `json:"used"`
	UsedByDataset     int64 `json:"used_by_dataset"`
	UsedByReservation int64 `json:"used_by_reservation"`
	UsedBySnapshots   int64 `json:"used_by_snapshots"`
	Available         int64 `json:"available"`
}

// enabledServicesAPI mirrors the nested "enabled_services" object returned
// by audit.config and audit.update. The three keys (MIDDLEWARE, SMB, SUDO)
// are fixed per the probed schema's "_attrs_order_"; SUDO is explicitly
// typed as an array of strings, and MIDDLEWARE/SMB are documented as
// event-type names / share names (also strings in practice — probed empty
// on both test boxes).
type enabledServicesAPI struct {
	Middleware []string `json:"MIDDLEWARE"`
	SMB        []string `json:"SMB"`
	SUDO       []string `json:"SUDO"`
}

// auditConfigAPI mirrors the JSON object returned by audit.config and
// audit.update. Probed live against TrueNAS 25.10 and 26.0: identical shape,
// no version gating needed. Note there is no top-level "enable" toggle:
// auditing is enabled per-service (SMB share audit settings, sudo config,
// etc), and remote_logging_enabled/space/enabled_services are read-only —
// audit.update accepts only retention/reservation/quota/quota_fill_warning/
// quota_fill_critical.
type auditConfigAPI struct {
	ID                   int64              `json:"id"`
	Retention            int64              `json:"retention"`
	Reservation          int64              `json:"reservation"`
	Quota                int64              `json:"quota"`
	QuotaFillWarning     int64              `json:"quota_fill_warning"`
	QuotaFillCritical    int64              `json:"quota_fill_critical"`
	RemoteLoggingEnabled bool               `json:"remote_logging_enabled"`
	Space                spaceAPI           `json:"space"`
	EnabledServices      enabledServicesAPI `json:"enabled_services"`
}

// spaceObjectValue builds a types.Object for the nested "space" attribute
// from an API response.
func spaceObjectValue(ctx context.Context, api spaceAPI) (types.Object, diag.Diagnostics) {
	return types.ObjectValueFrom(ctx, spaceAttrTypes, SpaceModel{
		Used:              types.Int64Value(api.Used),
		UsedByDataset:     types.Int64Value(api.UsedByDataset),
		UsedByReservation: types.Int64Value(api.UsedByReservation),
		UsedBySnapshots:   types.Int64Value(api.UsedBySnapshots),
		Available:         types.Int64Value(api.Available),
	})
}

// stringListOrEmpty converts a possibly-nil []string from the API into a
// non-null types.List, matching the nil-guard convention used elsewhere for
// API-returned lists (e.g. resilver_config's weekday).
func stringListOrEmpty(ctx context.Context, vals []string) (types.List, diag.Diagnostics) {
	if vals == nil {
		vals = []string{}
	}
	return types.ListValueFrom(ctx, types.StringType, vals)
}

// enabledServicesObjectValue builds a types.Object for the nested
// "enabled_services" attribute from an API response.
func enabledServicesObjectValue(ctx context.Context, api enabledServicesAPI) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics

	middleware, d := stringListOrEmpty(ctx, api.Middleware)
	diags.Append(d...)
	smb, d := stringListOrEmpty(ctx, api.SMB)
	diags.Append(d...)
	sudo, d := stringListOrEmpty(ctx, api.SUDO)
	diags.Append(d...)
	if diags.HasError() {
		return types.ObjectNull(enabledServicesAttrTypes), diags
	}

	obj, d := types.ObjectValueFrom(ctx, enabledServicesAttrTypes, EnabledServicesModel{
		Middleware: middleware,
		Smb:        smb,
		Sudo:       sudo,
	})
	diags.Append(d...)
	return obj, diags
}

// responseToModel maps an auditConfigAPI response onto an AuditConfigModel.
func responseToModel(ctx context.Context, api *auditConfigAPI, m *AuditConfigModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(auditConfigResourceID)
	m.Retention = types.Int64Value(api.Retention)
	m.Reservation = types.Int64Value(api.Reservation)
	m.Quota = types.Int64Value(api.Quota)
	m.QuotaFillWarning = types.Int64Value(api.QuotaFillWarning)
	m.QuotaFillCritical = types.Int64Value(api.QuotaFillCritical)
	m.RemoteLoggingEnabled = types.BoolValue(api.RemoteLoggingEnabled)

	space, d := spaceObjectValue(ctx, api.Space)
	diags.Append(d...)
	m.Space = space

	enabledServices, d := enabledServicesObjectValue(ctx, api.EnabledServices)
	diags.Append(d...)
	m.EnabledServices = enabledServices

	return diags
}

// responseToDataSourceModel maps an auditConfigAPI response onto an
// AuditConfigDataSourceModel.
func responseToDataSourceModel(ctx context.Context, api *auditConfigAPI, m *AuditConfigDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(auditConfigResourceID)
	m.Retention = types.Int64Value(api.Retention)
	m.Reservation = types.Int64Value(api.Reservation)
	m.Quota = types.Int64Value(api.Quota)
	m.QuotaFillWarning = types.Int64Value(api.QuotaFillWarning)
	m.QuotaFillCritical = types.Int64Value(api.QuotaFillCritical)
	m.RemoteLoggingEnabled = types.BoolValue(api.RemoteLoggingEnabled)

	space, d := spaceObjectValue(ctx, api.Space)
	diags.Append(d...)
	m.Space = space

	enabledServices, d := enabledServicesObjectValue(ctx, api.EnabledServices)
	diags.Append(d...)
	m.EnabledServices = enabledServices

	return diags
}

// updatePayload builds the audit.update argument. Every field is guarded:
// each is only included when known (Optional+Computed), so an unset
// optional is omitted entirely and the TrueNAS-side current value is left
// unchanged rather than overwritten with an explicit zero value.
//
// CALLERS MUST invoke this on a model populated from req.Config
// (req.Config.Get), never from req.Plan: "retention", "reservation",
// "quota", "quota_fill_warning", and "quota_fill_critical" are all
// Optional+Computed with UseStateForUnknown plan modifiers, so for any of
// them left unset by the user the *plan* value is not null — the modifier
// copies the prior state's value into the plan so Terraform can show a
// stable diff. Building the payload from the plan would therefore resend a
// field's last-known value on every Create/Update even when the user never
// configured it, silently reasserting it against whatever the box's
// current value happens to be (a revert race if that value changed since
// the last read). req.Config, by contrast, stays null for anything the
// user did not set in HCL regardless of plan modifiers, so inclusion here
// is driven strictly by what the user explicitly configured. Mirrors the
// webshare_config/twofactor_auth precedent.
func (m *AuditConfigModel) updatePayload() map[string]any {
	p := map[string]any{}

	if !m.Retention.IsNull() && !m.Retention.IsUnknown() {
		p["retention"] = m.Retention.ValueInt64()
	}
	if !m.Reservation.IsNull() && !m.Reservation.IsUnknown() {
		p["reservation"] = m.Reservation.ValueInt64()
	}
	if !m.Quota.IsNull() && !m.Quota.IsUnknown() {
		p["quota"] = m.Quota.ValueInt64()
	}
	if !m.QuotaFillWarning.IsNull() && !m.QuotaFillWarning.IsUnknown() {
		p["quota_fill_warning"] = m.QuotaFillWarning.ValueInt64()
	}
	if !m.QuotaFillCritical.IsNull() && !m.QuotaFillCritical.IsUnknown() {
		p["quota_fill_critical"] = m.QuotaFillCritical.ValueInt64()
	}

	return p
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by Delete.
// Delete makes NO client calls: audit retention/quota configuration is
// system-critical, so removing this resource from Terraform state must
// never reset the box's audit settings. Splitting this into its own
// function keeps Delete's "no client calls" contract independently
// unit-testable.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"Audit configuration left in place",
		"Audit configuration left in place; removed from Terraform state only",
	)
	return diags
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vmware

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// stateAttrTypes describes the attribute types of the nested "state"
// object.
var stateAttrTypes = map[string]attr.Type{
	"state":    types.StringType,
	"error":    types.StringType,
	"datetime": types.StringType,
}

// StateModel maps to the nested "state" attribute.
type StateModel struct {
	State    types.String `tfsdk:"state"`
	Error    types.String `tfsdk:"error"`
	Datetime types.String `tfsdk:"datetime"`
}

// VMwareModel is the Terraform state/plan model for a VMware
// snapshot-integration entry. Used for both the resource and the
// datasource: field types line up with both schemas, differing only in
// Required/Optional/Computed markers declared in schema.go / datasource.go.
//
// "Password" is Required + Sensitive + WriteOnly: vmware.create validates
// hostname/username/password against the real vCenter/ESXi endpoint before
// persisting anything (decisive live probe, both TrueNAS 25.10.4 HA and 26.0
// — see schema.go's doc comment), so — exactly like
// internal/resources/app_registry's identically-situated "password" — no
// live entry was ever obtainable to directly observe a read-back value.
// This mirrors app_registry's conservative, framework-native choice
// (Required + Sensitive + WriteOnly, never read back), itself modeled on
// truenas_iscsi_auth's "secret"/"peersecret".
type VMwareModel struct {
	ID         types.Int64  `tfsdk:"id"`
	Datastore  types.String `tfsdk:"datastore"`
	Filesystem types.String `tfsdk:"filesystem"`
	Hostname   types.String `tfsdk:"hostname"`
	Username   types.String `tfsdk:"username"`
	Password   types.String `tfsdk:"password"` // write-only, Sensitive; never read back
	State      types.Object `tfsdk:"state"`    // {state, error, datetime}; Computed-only
}

// VMwareDataSourceModel is the read-only lookup model for the
// truenas_vmware datasource. It intentionally has no "password" field:
// mirrors truenas_app_registry's datasource, which omits "password" for the
// same reason (never observed server-side in a form this provider could
// read back, and never appropriate to surface through a read-only lookup
// even if it were).
type VMwareDataSourceModel struct {
	ID         types.Int64  `tfsdk:"id"`
	Datastore  types.String `tfsdk:"datastore"`
	Filesystem types.String `tfsdk:"filesystem"`
	Hostname   types.String `tfsdk:"hostname"`
	Username   types.String `tfsdk:"username"`
	State      types.Object `tfsdk:"state"`
}

// stateAPI mirrors the nested "state" object returned by every vmware.*
// method that returns a full record (create/update/get_instance/query).
// Probed live via core.get_methods against TrueNAS 26.0
// (192.168.1.68): {"state": {"state": <enum>, "error": <string>,
// "datetime": <string, date-time format>}}, where the inner "state" enum is
// one of PENDING/SUCCESS/ERROR/BLOCKED and "error"/"datetime" are
// individually optional (schema "_required_": false) — no live vmware
// record was obtainable to observe their concrete JSON presence/absence
// (vmware.create validates against the real endpoint before persisting
// anything on both probed releases; see schema.go), so both are modeled as
// nullable pointers, matching the schema's own declared optionality rather
// than an observed instance. TrueNAS 25.10.4 HA's core.get_methods
// introspection reported "state" only as an unexpanded `"type": "object"`
// (with an identical top-level description to 26.0's), consistent with
// this provider's repeated finding elsewhere (enclosure2.*/ipmi.*/
// failover.*) that core.get_methods can under-report schema detail on that
// release — the 26.0 box's fuller introspection is treated as authoritative
// for both, since nothing in the 25.10.4 output contradicts it.
type stateAPI struct {
	State    string  `json:"state"`
	Error    *string `json:"error"`
	Datetime *string `json:"datetime"`
}

// vmwareAPI is the JSON wire format returned by vmware.create/get_instance/
// query/update, probed live via core.get_methods against both TrueNAS
// 25.10.4 HA (wss://10.220.16.188) and 26.0 (wss://192.168.1.68): identical
// field names and required-ness on both releases.
type vmwareAPI struct {
	ID         int64    `json:"id"`
	Datastore  string   `json:"datastore"`
	Filesystem string   `json:"filesystem"`
	Hostname   string   `json:"hostname"`
	Username   string   `json:"username"`
	Password   string   `json:"password"`
	State      stateAPI `json:"state"`
}

// nullableStringValue converts a nullable API string pointer to a
// types.String, preserving true null rather than substituting "".
func nullableStringValue(v *string) types.String {
	if v == nil {
		return types.StringNull()
	}
	return types.StringValue(*v)
}

// stateObjectValue builds a types.Object for the nested "state" attribute
// from an API response.
func stateObjectValue(ctx context.Context, api stateAPI) (types.Object, diag.Diagnostics) {
	return types.ObjectValueFrom(ctx, stateAttrTypes, StateModel{
		State:    types.StringValue(api.State),
		Error:    nullableStringValue(api.Error),
		Datetime: nullableStringValue(api.Datetime),
	})
}

// apiPayload converts the Terraform model into the map expected by
// vmware.create / vmware.update. All five fields ("datastore", "filesystem",
// "hostname", "username", "password") are Required in the schema (both on
// create and — per vmware.update's own accepts schema, probed identical on
// both releases — every field remains individually settable on update too,
// with no immutable/RequiresReplace field), so all five are always
// included. "password" is Required + WriteOnly: its live value is only
// available via req.Config (the terraform-plugin-framework nulls WriteOnly
// attributes in req.Plan), so callers pass it in explicitly rather than
// reading m.Password — mirrors app_registry's apiPayload.
func (m *VMwareModel) apiPayload(cfgPassword types.String) map[string]any {
	return map[string]any{
		"datastore":  m.Datastore.ValueString(),
		"filesystem": m.Filesystem.ValueString(),
		"hostname":   m.Hostname.ValueString(),
		"username":   m.Username.ValueString(),
		"password":   cfgPassword.ValueString(),
	}
}

// responseToModel maps a vmwareAPI response onto a VMwareModel. Password is
// NOT set here (write-only, never read back from the API) — whatever the
// caller already has in m.Password (from plan/config) is left untouched,
// mirroring app_registry's responseToModel treatment of its own "password".
func responseToModel(ctx context.Context, api *vmwareAPI, m *VMwareModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Datastore = types.StringValue(api.Datastore)
	m.Filesystem = types.StringValue(api.Filesystem)
	m.Hostname = types.StringValue(api.Hostname)
	m.Username = types.StringValue(api.Username)

	state, d := stateObjectValue(ctx, api.State)
	diags.Append(d...)
	m.State = state

	return diags
}

// responseToDataSourceModel maps a vmwareAPI response onto a
// VMwareDataSourceModel. Password is never populated (see
// VMwareDataSourceModel's doc comment).
func responseToDataSourceModel(ctx context.Context, api *vmwareAPI, m *VMwareDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Datastore = types.StringValue(api.Datastore)
	m.Filesystem = types.StringValue(api.Filesystem)
	m.Hostname = types.StringValue(api.Hostname)
	m.Username = types.StringValue(api.Username)

	state, d := stateObjectValue(ctx, api.State)
	diags.Append(d...)
	m.State = state

	return diags
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app_registry

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// AppRegistryModel is the Terraform state model for truenas_app_registry.
//
// Password is write-only: probed live against TrueNAS 26.0
// (app.registry.create), the API documents both the "password" request and
// response fields as "masked for security" (identical wording on both
// TrueNAS 25.10 and 26.0, via core.get_methods). No pre-existing app.registry
// entry was available on either probed box to directly observe a read-back
// value (both app.registry.query calls returned an empty list — see
// model_test.go / schema.go doc comments for the full decisive-probe
// evidence), so this mirrors the conservative, framework-native pattern
// used by truenas_iscsi_auth's "secret"/"peersecret" (also masked,
// credential-shaped fields): Required + Sensitive + WriteOnly, never read
// back into state.
type AppRegistryModel struct {
	ID          types.Int64  `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"` // nullable
	URI         types.String `tfsdk:"uri"`
	Username    types.String `tfsdk:"username"`
	Password    types.String `tfsdk:"password"` // write-only, Sensitive; never read back
}

// AppRegistryDataSourceModel is the read-only lookup model for the
// truenas_app_registry datasource. It intentionally has no "password"
// field: registry credentials must never be exposed through a datasource
// read (mirrors truenas_iscsi_auth's datasource, which omits "secret"/
// "peersecret" for the same reason).
type AppRegistryDataSourceModel struct {
	ID          types.Int64  `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	URI         types.String `tfsdk:"uri"`
	Username    types.String `tfsdk:"username"`
}

// appRegistryAPI is the JSON wire format returned by app.registry.create,
// .get_instance, .query, and .update, probed live via core.get_methods
// against both TrueNAS 25.10.3.1 and 26.0 (identical shape on both;
// no version gating needed). "description" is nullable (defaults to null
// when omitted on create). "uri" defaults server-side to
// "https://index.docker.io/v1/" (Docker Hub) when omitted on create, and is
// always present (non-null) on every read. "password" is documented
// ("masked for security") but its actual read-back value could not be
// observed live: app.registry.create validates username/password/uri
// against the real registry endpoint before persisting anything (see the
// DECISIVE probe evidence in schema.go), so no entry survives long enough
// to query back, and both probed boxes had zero pre-existing entries
// (app.registry.query returned `[]` on both 25.10 and 26.0, despite 26.0
// having apps configured and running — there is no default "docker.io"
// entry auto-created, contrary to the task brief's expectation). Password
// is therefore intentionally excluded from this struct's read path
// (responseToModel never assigns it) and kept out of the API struct
// entirely to make that impossible to get wrong.
type appRegistryAPI struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	URI         string  `json:"uri"`
	Username    string  `json:"username"`
}

// responseToModel maps an appRegistryAPI response onto an AppRegistryModel.
// Password is write-only and is NEVER assigned here: whatever the caller
// already has in m.Password (from plan/config) is left untouched. Mirrors
// internal/resources/iscsi_auth's responseToModel treatment of
// Secret/PeerSecret.
func responseToModel(api *appRegistryAPI, m *AppRegistryModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	if api.Description != nil {
		m.Description = types.StringValue(*api.Description)
	} else {
		m.Description = types.StringNull()
	}
	m.URI = types.StringValue(api.URI)
	m.Username = types.StringValue(api.Username)

	// NOTE: Password is NOT set here (write-only, never read back from the
	// API).

	return diags
}

// responseToDataSourceModel maps an appRegistryAPI response onto an
// AppRegistryDataSourceModel. Password is never populated (see
// AppRegistryDataSourceModel doc comment).
func responseToDataSourceModel(api *appRegistryAPI, m *AppRegistryDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	if api.Description != nil {
		m.Description = types.StringValue(*api.Description)
	} else {
		m.Description = types.StringNull()
	}
	m.URI = types.StringValue(api.URI)
	m.Username = types.StringValue(api.Username)

	return diags
}

// apiPayload builds the map[string]any payload shared by
// app.registry.create and app.registry.update. name/username are Required
// in the schema, so they are always included. password is Required +
// WriteOnly: its live value is only available via req.Config (the
// terraform-plugin-framework nulls WriteOnly attributes in req.Plan), so
// callers pass it in explicitly rather than reading m.Password. uri is
// Optional+Computed (server default "https://index.docker.io/v1/" applies
// when omitted on create); it is included whenever known, which by
// app.registry.update time is always the case (UseStateForUnknown resolves
// it from prior state when the user doesn't set it). description is
// Optional (nullable, not Computed) and is always included — explicitly as
// null when unset — so that app.registry.update (a partial update where
// omitted keys mean "no change") can still clear a previously-set
// description back to null.
func (m *AppRegistryModel) apiPayload(cfgPassword types.String) map[string]any {
	p := map[string]any{
		"name":     m.Name.ValueString(),
		"username": m.Username.ValueString(),
		"password": cfgPassword.ValueString(),
	}

	if m.Description.IsNull() {
		p["description"] = nil
	} else if !m.Description.IsUnknown() {
		p["description"] = m.Description.ValueString()
	}

	if !m.URI.IsNull() && !m.URI.IsUnknown() {
		p["uri"] = m.URI.ValueString()
	}

	return p
}

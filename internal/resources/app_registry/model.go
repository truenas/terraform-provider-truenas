// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app_registry

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

// AppRegistryModel is the Terraform state model for truenas_app_registry.
//
// PasswordWO is write-only: probed live against TrueNAS 26.0
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
//
// PasswordWOVersion is the rotation trigger for PasswordWO. A write-only
// value is nulled in the plan, so Terraform cannot see when the token itself
// changes; bumping this integer produces an ordinary diff that drives
// app.registry.update, re-sending the current password_wo value. This mirrors
// the house password_wo/password_wo_version pattern (cloud_backup, #36). (#48)
type AppRegistryModel struct {
	ID                types.Int64   `tfsdk:"id"`
	Name              types.String  `tfsdk:"name"`
	Description       types.String  `tfsdk:"description"` // nullable
	URI               normalize.URL `tfsdk:"uri"`         // server appends "/" to http(s) URIs (#47, B-122)
	Username          types.String  `tfsdk:"username"`
	PasswordWO        types.String  `tfsdk:"password_wo"`         // write-only, Sensitive; never read back
	PasswordWOVersion types.Int64   `tfsdk:"password_wo_version"` // rotation trigger for password_wo
}

// AppRegistryDataSourceModel is the read-only lookup model for the
// truenas_app_registry datasource. It intentionally has no "password"
// field: registry credentials must never be exposed through a datasource
// read (mirrors truenas_iscsi_auth's datasource, which omits "secret"/
// "peersecret" for the same reason).
type AppRegistryDataSourceModel struct {
	ID          types.Int64   `tfsdk:"id"`
	Name        types.String  `tfsdk:"name"`
	Description types.String  `tfsdk:"description"`
	URI         normalize.URL `tfsdk:"uri"`
	Username    types.String  `tfsdk:"username"`
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
// PasswordWO is write-only and is NEVER assigned here; PasswordWOVersion is a
// configuration-only trigger with no API counterpart, so it is left untouched
// too (its value carries through from plan/state). Mirrors
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
	// The server appends "/" to an http(s) URI; the URL type's semantic
	// equality keeps the configured spelling (#47).
	m.URI = normalize.NewURLValue(api.URI)
	// username is a Secret in the API: a READONLY_ADMIN session gets the
	// redaction mask instead. Keep the prior value then (null after import,
	// so the next apply records the configured username), rather than storing
	// the mask (B-032).
	if api.Username != redactedSecret {
		m.Username = types.StringValue(api.Username)
	}

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
	m.URI = normalize.NewURLValue(api.URI)
	// A redacted username (READONLY_ADMIN session) is reported as null, not
	// as "********" (B-032).
	m.Username = types.StringNull()
	if api.Username != redactedSecret {
		m.Username = types.StringValue(api.Username)
	}

	return diags
}

// redactedSecret is what app.registry returns for a Secret field (username,
// password) to a session without full-admin rights (a READONLY_ADMIN key).
const redactedSecret = "********"

// apiPayload builds the map[string]any payload shared by
// app.registry.create and app.registry.update. name/username are Required
// in the schema, so they are always included. The registry credential is
// Required + WriteOnly (password_wo): its live value is only available via
// req.Config (the terraform-plugin-framework nulls WriteOnly attributes in
// req.Plan), so callers pass it in explicitly rather than reading the model.
// It is always sent as the API's "password" field. uri is
// Optional+Computed (server default "https://index.docker.io/v1/" applies
// when omitted on create); it is included whenever known, which by
// app.registry.update time is always the case (UseStateForUnknown resolves
// it from prior state when the user doesn't set it). description is
// Optional (nullable, not Computed) and is always included — explicitly as
// null when unset — so that app.registry.update (a partial update where
// omitted keys mean "no change") can still clear a previously-set
// description back to null.
func (m *AppRegistryModel) apiPayload(cfgPasswordWO types.String) map[string]any {
	p := map[string]any{
		"name":     m.Name.ValueString(),
		"username": m.Username.ValueString(),
	}
	// Update passes a null password unless password_wo_version changed
	// (B-141); app.registry.update then keeps the stored one.
	if !cfgPasswordWO.IsNull() && !cfgPasswordWO.IsUnknown() {
		p["password"] = cfgPasswordWO.ValueString()
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

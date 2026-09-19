// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package api_key

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// APIKeyModel is the Terraform state/plan model for truenas_api_key.
//
// Key is Computed + Sensitive: api_key.create is the only call that ever
// returns the plaintext key (probed against a live TrueNAS box —
// api_key.query, api_key.get_instance, and a plain api_key.update all omit
// it; only api_key.update with reset:true, which this resource never sends,
// echoes a new one). responseToModel deliberately never touches Key, so
// Create must set it explicitly from the create response, and Read/Update
// carry forward whatever value is already in state/plan.
type APIKeyModel struct {
	ID        types.Int64  `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Username  types.String `tfsdk:"username"` // immutable: api_key.update rejects it (RequiresReplace)
	ExpiresAt types.String `tfsdk:"expires_at"`
	Key       types.String `tfsdk:"key"` // Sensitive+Computed; set once at Create, never overwritten after
	CreatedAt types.String `tfsdk:"created_at"`
	Local     types.Bool   `tfsdk:"local"`
	Revoked   types.Bool   `tfsdk:"revoked"`
}

// APIKeyDataSourceModel is the read-only model for the truenas_api_key data
// source, looked up by name. It omits Key entirely: api_key.query never
// returns it, so there is nothing for a datasource to surface.
type APIKeyDataSourceModel struct {
	ID        types.Int64  `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Username  types.String `tfsdk:"username"`
	ExpiresAt types.String `tfsdk:"expires_at"`
	CreatedAt types.String `tfsdk:"created_at"`
	Local     types.Bool   `tfsdk:"local"`
	Revoked   types.Bool   `tfsdk:"revoked"`
}

// apiKeyAPI mirrors the JSON object returned by api_key.create,
// api_key.update, api_key.get_instance, and api_key.query. Probed against a
// live TrueNAS 25.10 box:
//   - created_at and expires_at are serialized as extended-JSON datetime
//     objects, {"$date": <milliseconds-since-epoch>} — never as the ISO
//     "date-time" strings core.get_methods' schema advertises. An ISO
//     string (e.g. "2030-01-01T00:00:00+00:00") is REJECTED by
//     api_key.create with "Input should be a valid datetime"; only the
//     {"$date": ms} shape is accepted on input too.
//   - expires_at is null when the key has no expiration (both an omitted
//     create field and an explicit `"expires_at": null` produce this).
//   - key is present only in the api_key.create response and in an
//     api_key.update response when that update sent reset:true (rotation).
//     This resource never sends reset, so on Update the field is always
//     absent — Key decodes to "" and must not overwrite state.
//   - user_identifier is a *string* in the create response ("33") but an
//     *integer* in query/get_instance responses (33) — an inconsistency in
//     the API itself. Not modeled here: the brief's field list omits it, as
//     well as keyhash and revoked_reason.
type apiKeyAPI struct {
	ID        int64           `json:"id"`
	Name      string          `json:"name"`
	Username  string          `json:"username"`
	CreatedAt json.RawMessage `json:"created_at"`
	ExpiresAt json.RawMessage `json:"expires_at"`
	Local     bool            `json:"local"`
	Revoked   bool            `json:"revoked"`
	Key       string          `json:"key"`
}

// decodeDateField decodes a wire datetime field shaped {"$date": ms} into a
// UTC time.Time. It returns (nil, nil) for a JSON null (no value), and an
// error for anything else undecodable.
func decodeDateField(raw json.RawMessage) (*time.Time, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	var dv struct {
		Date int64 `json:"$date"`
	}
	if err := json.Unmarshal(raw, &dv); err != nil {
		return nil, fmt.Errorf("cannot decode datetime field %q as {\"$date\": <ms>}: %w", string(raw), err)
	}

	t := time.UnixMilli(dv.Date).UTC()
	return &t, nil
}

// nullableRFC3339Value converts a nullable *time.Time to a types.String
// formatted as RFC3339 in UTC, preserving true null.
func nullableRFC3339Value(t *time.Time) types.String {
	if t == nil {
		return types.StringNull()
	}
	return types.StringValue(t.Format(time.RFC3339))
}

// expiresAtToPayload converts the "expires_at" model field to the wire
// value api_key.create/api_key.update expect: nil (encodes to JSON null,
// meaning no expiration) when unset, or {"$date": <ms>} when set. Returns a
// diagnostic error rather than a fallback value when the configured string
// is not a valid RFC3339 timestamp.
func expiresAtToPayload(v types.String) (any, diag.Diagnostics) {
	var diags diag.Diagnostics

	if v.IsNull() || v.IsUnknown() {
		return nil, diags
	}

	t, err := time.Parse(time.RFC3339, v.ValueString())
	if err != nil {
		diags.AddError(
			"Invalid expires_at",
			fmt.Sprintf("expires_at %q is not a valid RFC3339 timestamp: %s", v.ValueString(), err),
		)
		return nil, diags
	}

	return map[string]any{"$date": t.UnixMilli()}, diags
}

// responseToModel maps an apiKeyAPI response onto an APIKeyModel. Key is
// intentionally left untouched (see APIKeyModel's doc comment) — callers
// that just created or rotated a key must set m.Key separately from the
// raw create/reset-update response.
func responseToModel(api *apiKeyAPI, m *APIKeyModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.Username = types.StringValue(api.Username)

	createdAt, err := decodeDateField(api.CreatedAt)
	if err != nil {
		diags.AddError("Invalid created_at in API response", err.Error())
		return diags
	}
	if createdAt == nil {
		diags.AddError("Invalid created_at in API response", "created_at was unexpectedly null")
		return diags
	}
	m.CreatedAt = types.StringValue(createdAt.Format(time.RFC3339))

	expiresAt, err := decodeDateField(api.ExpiresAt)
	if err != nil {
		diags.AddError("Invalid expires_at in API response", err.Error())
		return diags
	}
	m.ExpiresAt = nullableRFC3339Value(expiresAt)

	m.Local = types.BoolValue(api.Local)
	m.Revoked = types.BoolValue(api.Revoked)

	return diags
}

// responseToDataSourceModel maps an apiKeyAPI response onto an
// APIKeyDataSourceModel.
func responseToDataSourceModel(api *apiKeyAPI, m *APIKeyDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.Username = types.StringValue(api.Username)

	createdAt, err := decodeDateField(api.CreatedAt)
	if err != nil {
		diags.AddError("Invalid created_at in API response", err.Error())
		return diags
	}
	if createdAt == nil {
		diags.AddError("Invalid created_at in API response", "created_at was unexpectedly null")
		return diags
	}
	m.CreatedAt = types.StringValue(createdAt.Format(time.RFC3339))

	expiresAt, err := decodeDateField(api.ExpiresAt)
	if err != nil {
		diags.AddError("Invalid expires_at in API response", err.Error())
		return diags
	}
	m.ExpiresAt = nullableRFC3339Value(expiresAt)

	m.Local = types.BoolValue(api.Local)
	m.Revoked = types.BoolValue(api.Revoked)

	return diags
}

// createPayload builds the map expected by api_key.create. username and
// name are always included (Required in the schema; name additionally has
// a server-side default but this resource always sends it explicitly for a
// predictable result). expires_at is always included, as either
// {"$date": ms} or JSON null — api_key.create treats an omitted
// expires_at identically to an explicit null (both probed to produce "no
// expiration"), so there is no need to distinguish omission from null here.
func (m *APIKeyModel) createPayload() (map[string]any, diag.Diagnostics) {
	expiresAt, diags := expiresAtToPayload(m.ExpiresAt)
	if diags.HasError() {
		return nil, diags
	}

	p := map[string]any{
		"username":   m.Username.ValueString(),
		"name":       m.Name.ValueString(),
		"expires_at": expiresAt,
	}
	return p, diags
}

// updatePayload builds the map expected by api_key.update. username is
// never included: api_key.update rejects it outright ("Extra inputs are
// not permitted") since the key owner is immutable after creation (the
// schema's RequiresReplace plan modifier on username guarantees Update is
// never called across an owner change). reset is never included either —
// key rotation is out of scope for this resource; taint the resource to
// rotate. expires_at is always included, as either {"$date": ms} or JSON
// null: unlike api_key.create, api_key.update treats an *omitted*
// expires_at as "leave unchanged" (probed: renaming a key without
// resending expires_at left its prior expiration intact). Note that
// removing expires_at from config does NOT reach this "clear" path in
// practice: expires_at is Optional+Computed, so Terraform carries the
// prior state value forward into the plan when the attribute is absent
// from config, and m.ExpiresAt here is never actually null/unknown on an
// in-place update triggered that way — clearing a previously-set
// expiration requires tainting the resource (Destroy+Create, which goes
// through createPayload instead). Sending expires_at unconditionally on
// every update is still correct: it's harmless (a no-op when unchanged)
// and keeps the update payload shape deterministic and identical in
// structure to createPayload's.
func (m *APIKeyModel) updatePayload() (map[string]any, diag.Diagnostics) {
	expiresAt, diags := expiresAtToPayload(m.ExpiresAt)
	if diags.HasError() {
		return nil, diags
	}

	p := map[string]any{
		"name":       m.Name.ValueString(),
		"expires_at": expiresAt,
	}
	return p, diags
}

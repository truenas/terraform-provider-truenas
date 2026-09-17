// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package api_key

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// --- expiresAtToPayload: the three input states the create/update payload
// builders rely on. ---

// TestExpiresAtToPayload_Null verifies an unset/null model value encodes to
// a nil payload entry (JSON null on the wire), matching "no expiration".
func TestExpiresAtToPayload_Null(t *testing.T) {
	v, diags := expiresAtToPayload(types.StringNull())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if v != nil {
		t.Errorf("expiresAtToPayload(null) = %v, want nil", v)
	}
}

// TestExpiresAtToPayload_Value verifies a valid RFC3339 string encodes to
// the {"$date": <ms>} shape api_key.create/update accept (probed live: an
// ISO datetime string is rejected outright, only this extended-JSON shape
// works).
func TestExpiresAtToPayload_Value(t *testing.T) {
	v, diags := expiresAtToPayload(types.StringValue("2030-01-01T00:00:00Z"))
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expiresAtToPayload(value) = %T, want map[string]any", v)
	}
	const wantMS = int64(1893456000000) // 2030-01-01T00:00:00Z in ms
	if m["$date"] != wantMS {
		t.Errorf("payload[$date] = %v, want %d", m["$date"], wantMS)
	}
}

// TestExpiresAtToPayload_InvalidFormat verifies a non-RFC3339 string
// produces a diagnostic error rather than a fallback value.
func TestExpiresAtToPayload_InvalidFormat(t *testing.T) {
	_, diags := expiresAtToPayload(types.StringValue("not-a-timestamp"))
	if !diags.HasError() {
		t.Fatal("expected a diagnostic error for an invalid expires_at string")
	}
}

// --- decodeDateField ---

func TestDecodeDateField_Null(t *testing.T) {
	tm, err := decodeDateField(json.RawMessage("null"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tm != nil {
		t.Errorf("decodeDateField(null) = %v, want nil", tm)
	}
}

func TestDecodeDateField_Value(t *testing.T) {
	tm, err := decodeDateField(json.RawMessage(`{"$date": 1784667041000}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tm == nil {
		t.Fatal("decodeDateField returned nil, want a value")
	}
	if got := tm.Format("2006-01-02"); got != "2026-07-21" {
		t.Errorf("decoded date = %q, want 2026-07-21", got)
	}
}

func TestDecodeDateField_Invalid(t *testing.T) {
	_, err := decodeDateField(json.RawMessage(`"not a date object"`))
	if err == nil {
		t.Fatal("expected an error for an undecodable datetime shape")
	}
}

// --- createPayload / updatePayload ---

// TestCreatePayload_IncludesUsername verifies username is present in the
// create payload (Required, api_key.create-only field).
func TestCreatePayload_IncludesUsername(t *testing.T) {
	m := &APIKeyModel{
		Name:      types.StringValue("tf-acc-key"),
		Username:  types.StringValue("truenas_admin"),
		ExpiresAt: types.StringNull(),
	}
	p, diags := m.createPayload()
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if p["username"] != "truenas_admin" {
		t.Errorf("payload[username] = %v, want truenas_admin", p["username"])
	}
	if p["name"] != "tf-acc-key" {
		t.Errorf("payload[name] = %v, want tf-acc-key", p["name"])
	}
	if p["expires_at"] != nil {
		t.Errorf("payload[expires_at] = %v, want nil", p["expires_at"])
	}
}

// TestCreatePayload_InvalidExpiresAt verifies createPayload surfaces the
// diagnostic error from expiresAtToPayload instead of building a partial
// payload.
func TestCreatePayload_InvalidExpiresAt(t *testing.T) {
	m := &APIKeyModel{
		Name:      types.StringValue("tf-acc-key"),
		Username:  types.StringValue("truenas_admin"),
		ExpiresAt: types.StringValue("garbage"),
	}
	p, diags := m.createPayload()
	if !diags.HasError() {
		t.Fatal("expected a diagnostic error")
	}
	if p != nil {
		t.Errorf("expected a nil payload on error, got %v", p)
	}
}

// TestUpdatePayload_OmitsUsername verifies username is never present in the
// update payload: api_key.update rejects it outright as an unrecognized
// field (probed live: "Extra inputs are not permitted"), matching the
// schema's RequiresReplace plan modifier on username.
func TestUpdatePayload_OmitsUsername(t *testing.T) {
	m := &APIKeyModel{
		Name:      types.StringValue("tf-acc-key-renamed"),
		Username:  types.StringValue("truenas_admin"),
		ExpiresAt: types.StringNull(),
	}
	p, diags := m.updatePayload()
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, ok := p["username"]; ok {
		t.Errorf("expected username to be omitted from the update payload, got %v", p["username"])
	}
	if p["name"] != "tf-acc-key-renamed" {
		t.Errorf("payload[name] = %v, want tf-acc-key-renamed", p["name"])
	}
}

// TestUpdatePayload_AlwaysSendsExpiresAt verifies expires_at is present in
// the update payload even when null — unlike api_key.create, an *omitted*
// expires_at on api_key.update means "leave unchanged" (probed live), so
// clearing a previously-set expiration requires sending an explicit null
// on every update, not omitting the field.
func TestUpdatePayload_AlwaysSendsExpiresAt(t *testing.T) {
	m := &APIKeyModel{
		Name:      types.StringValue("tf-acc-key"),
		Username:  types.StringValue("truenas_admin"),
		ExpiresAt: types.StringNull(),
	}
	p, diags := m.updatePayload()
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	v, present := p["expires_at"]
	if !present {
		t.Fatal("expected expires_at key to be present in the update payload even when null")
	}
	if v != nil {
		t.Errorf("payload[expires_at] = %v, want nil (JSON null)", v)
	}
}

// --- responseToModel: key preservation ---

// TestResponseToModel_PreservesKeyOnRead verifies responseToModel never
// touches m.Key, regardless of what is already there — the central
// invariant this resource depends on to avoid ever nulling out a
// previously-captured plaintext key on Read/Update.
func TestResponseToModel_PreservesKeyOnRead(t *testing.T) {
	api := &apiKeyAPI{
		ID:        7,
		Name:      "tf-acc-key-renamed",
		Username:  "truenas_admin",
		CreatedAt: json.RawMessage(`{"$date": 1784667041000}`),
		ExpiresAt: json.RawMessage("null"),
		Local:     true,
		Revoked:   false,
		// Key deliberately left as the zero value: api_key.get_instance
		// and a plain api_key.update never return it.
	}

	m := &APIKeyModel{Key: types.StringValue("7-preexisting-plaintext-key")}
	diags := responseToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.Key.ValueString() != "7-preexisting-plaintext-key" {
		t.Errorf("Key = %q, want prior state value preserved", m.Key.ValueString())
	}
	if m.Name.ValueString() != "tf-acc-key-renamed" {
		t.Errorf("Name = %q, want tf-acc-key-renamed", m.Name.ValueString())
	}
	if !m.ExpiresAt.IsNull() {
		t.Errorf("ExpiresAt = %v, want null", m.ExpiresAt)
	}
	if m.CreatedAt.ValueString() != "2026-07-21T20:50:41Z" {
		t.Errorf("CreatedAt = %q, want 2026-07-21T20:50:41Z", m.CreatedAt.ValueString())
	}
	if !m.Local.ValueBool() {
		t.Error("Local = false, want true")
	}
	if m.Revoked.ValueBool() {
		t.Error("Revoked = true, want false")
	}
}

// TestResponseToModel_ExpiresAtValue verifies a set expires_at round-trips
// through decodeDateField into the RFC3339 UTC string form.
func TestResponseToModel_ExpiresAtValue(t *testing.T) {
	api := &apiKeyAPI{
		ID:        1,
		Name:      "k",
		Username:  "truenas_admin",
		CreatedAt: json.RawMessage(`{"$date": 1784667041000}`),
		ExpiresAt: json.RawMessage(`{"$date": 1893456000000}`),
	}
	m := &APIKeyModel{}
	diags := responseToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.ExpiresAt.ValueString() != "2030-01-01T00:00:00Z" {
		t.Errorf("ExpiresAt = %q, want 2030-01-01T00:00:00Z", m.ExpiresAt.ValueString())
	}
}

// TestResponseToModel_MissingCreatedAt verifies a null created_at (which
// the live API never actually returns — it is always required — but which
// would indicate a wire-shape regression) surfaces as an error rather than
// silently producing a zero-value timestamp.
func TestResponseToModel_MissingCreatedAt(t *testing.T) {
	api := &apiKeyAPI{
		ID:        1,
		Name:      "k",
		Username:  "truenas_admin",
		CreatedAt: json.RawMessage("null"),
	}
	m := &APIKeyModel{}
	diags := responseToModel(api, m)
	if !diags.HasError() {
		t.Fatal("expected an error for a null created_at")
	}
}

// --- responseToDataSourceModel ---

// TestResponseToDataSourceModel_Basic verifies the datasource mapping
// (which has no Key field to preserve or omit).
func TestResponseToDataSourceModel_Basic(t *testing.T) {
	api := &apiKeyAPI{
		ID:        5,
		Name:      "tf-probe-apikey-noexpiry",
		Username:  "truenas_admin",
		CreatedAt: json.RawMessage(`{"$date": 1784667001000}`),
		ExpiresAt: json.RawMessage("null"),
		Local:     true,
		Revoked:   false,
	}
	m := &APIKeyDataSourceModel{}
	diags := responseToDataSourceModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.ID.ValueInt64() != 5 {
		t.Errorf("ID = %v, want 5", m.ID)
	}
	if m.Username.ValueString() != "truenas_admin" {
		t.Errorf("Username = %q, want truenas_admin", m.Username.ValueString())
	}
	if !m.ExpiresAt.IsNull() {
		t.Errorf("ExpiresAt = %v, want null", m.ExpiresAt)
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync_credentials

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestProviderMap_InvalidJSON verifies that providerMap rejects malformed
// JSON with an error diagnostic.
func TestProviderMap_InvalidJSON(t *testing.T) {
	m := CredentialsModel{Provider: types.StringValue("{not valid json")}

	p, diags := m.providerMap()
	if !diags.HasError() {
		t.Fatal("expected error diagnostics for invalid JSON, got none")
	}
	if p != nil {
		t.Errorf("expected nil provider map on error, got %v", p)
	}
}

// TestProviderMap_MissingType verifies that providerMap rejects valid JSON
// that lacks the required "type" key.
func TestProviderMap_MissingType(t *testing.T) {
	m := CredentialsModel{Provider: types.StringValue(`{"access_key_id": "AKIA..."}`)}

	p, diags := m.providerMap()
	if !diags.HasError() {
		t.Fatal("expected error diagnostics for missing type, got none")
	}
	if p != nil {
		t.Errorf("expected nil provider map on error, got %v", p)
	}
}

// TestProviderMap_Valid verifies that providerMap parses valid JSON with a
// type key without error.
func TestProviderMap_Valid(t *testing.T) {
	m := CredentialsModel{Provider: types.StringValue(`{"type": "S3", "access_key_id": "AKIA..."}`)}

	p, diags := m.providerMap()
	if diags.HasError() {
		t.Fatalf("unexpected error diagnostics: %v", diags)
	}
	if p["type"] != "S3" {
		t.Errorf("p[type] = %v, want S3", p["type"])
	}
}

// TestCreatePayload_WireKeyIsProvider verifies createPayload sends
// provider_config as the "provider" wire object ({"type": ..., ...settings},
// the /api/current shape) and includes name.
func TestCreatePayload_WireKeyIsProvider(t *testing.T) {
	m := CredentialsModel{
		Name:     types.StringValue("my-creds"),
		Provider: types.StringValue(`{"type": "S3", "access_key_id": "AKIA..."}`),
	}

	payload, diags := m.createPayload()
	if diags.HasError() {
		t.Fatalf("unexpected error diagnostics: %v", diags)
	}
	if payload["name"] != "my-creds" {
		t.Errorf("payload[name] = %v, want my-creds", payload["name"])
	}
	pm, ok := payload["provider"].(map[string]any)
	if !ok {
		t.Fatalf("payload[provider] is %T, want map[string]any", payload["provider"])
	}
	if pm["type"] != "S3" {
		t.Errorf("payload[provider][type] = %v, want S3", pm["type"])
	}
	if pm["access_key_id"] != "AKIA..." {
		t.Errorf("payload[provider][access_key_id] = %v, want AKIA...", pm["access_key_id"])
	}
	if _, ok := payload["provider_config"]; ok {
		t.Error("create payload should not contain key 'provider_config'")
	}
	if _, ok := payload["attributes"]; ok {
		t.Error("create payload should not contain key 'attributes' (/api/current rejects it)")
	}
}

// TestUpdatePayload_WireKeyIsProvider verifies updatePayload also sends
// provider_config as the "provider" wire object and includes name.
func TestUpdatePayload_WireKeyIsProvider(t *testing.T) {
	m := CredentialsModel{
		ID:       types.Int64Value(7),
		Name:     types.StringValue("my-creds"),
		Provider: types.StringValue(`{"type": "B2", "account": "abc"}`),
	}

	payload, diags := m.updatePayload()
	if diags.HasError() {
		t.Fatalf("unexpected error diagnostics: %v", diags)
	}
	if payload["name"] != "my-creds" {
		t.Errorf("payload[name] = %v, want my-creds", payload["name"])
	}
	pm, ok := payload["provider"].(map[string]any)
	if !ok {
		t.Fatalf("payload[provider] is %T, want map[string]any", payload["provider"])
	}
	if pm["type"] != "B2" {
		t.Errorf("payload[provider][type] = %v, want B2", pm["type"])
	}
	if pm["account"] != "abc" {
		t.Errorf("payload[provider][account] = %v, want abc", pm["account"])
	}
	if _, ok := payload["provider_config"]; ok {
		t.Error("update payload should not contain key 'provider_config'")
	}
	if _, ok := payload["attributes"]; ok {
		t.Error("update payload should not contain key 'attributes' (/api/current rejects it)")
	}
}

// TestProviderDrifted_Identical verifies that identical maps are not
// considered drifted.
func TestProviderDrifted_Identical(t *testing.T) {
	state := map[string]any{"type": "S3", "access_key_id": "AKIA..."}
	api := map[string]any{"type": "S3", "access_key_id": "AKIA..."}

	if providerDrifted(state, api) {
		t.Error("providerDrifted() = true for identical maps, want false")
	}
}

// TestProviderDrifted_ChangedValue verifies that a changed value for a
// user-set key is reported as drift.
func TestProviderDrifted_ChangedValue(t *testing.T) {
	state := map[string]any{"type": "S3", "access_key_id": "AKIA-OLD"}
	api := map[string]any{"type": "S3", "access_key_id": "AKIA-NEW"}

	if !providerDrifted(state, api) {
		t.Error("providerDrifted() = false for changed value, want true")
	}
}

// TestProviderDrifted_APIAddedKey verifies that a key present only in the
// API response (a server-added default) is NOT considered drift.
func TestProviderDrifted_APIAddedKey(t *testing.T) {
	state := map[string]any{"type": "S3", "access_key_id": "AKIA..."}
	api := map[string]any{"type": "S3", "access_key_id": "AKIA...", "endpoint": "https://s3.amazonaws.com"}

	if providerDrifted(state, api) {
		t.Error("providerDrifted() = true for API-added extra key, want false")
	}
}

// TestProviderDrifted_MissingUserKey verifies that a user-set key absent
// from the API response is considered drift.
func TestProviderDrifted_MissingUserKey(t *testing.T) {
	state := map[string]any{"type": "S3", "access_key_id": "AKIA..."}
	api := map[string]any{"type": "S3"}

	if !providerDrifted(state, api) {
		t.Error("providerDrifted() = false for missing user key in API, want true")
	}
}

// TestApiProviderJSON_MergesProviderAndAttributes verifies apiProviderJSON
// serializes the wire provider object back into a provider_config-shaped
// JSON string.
func TestApiProviderJSON_MergesProviderAndAttributes(t *testing.T) {
	api := &credentialsAPI{
		ID:   1,
		Name: "my-creds",
		Provider: map[string]any{
			"type":              "STORJ_IX",
			"access_key_id":     "AKIA...",
			"secret_access_key": "shh",
		},
	}

	providerJSON, diags := apiProviderJSON(api)
	if diags.HasError() {
		t.Fatalf("unexpected error diagnostics: %v", diags)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(providerJSON), &got); err != nil {
		t.Fatalf("apiProviderJSON produced invalid JSON: %v", err)
	}
	if got["type"] != "STORJ_IX" {
		t.Errorf("got[type] = %v, want STORJ_IX", got["type"])
	}
	if got["access_key_id"] != "AKIA..." {
		t.Errorf("got[access_key_id] = %v, want AKIA...", got["access_key_id"])
	}
	if got["secret_access_key"] != "shh" {
		t.Errorf("got[secret_access_key] = %v, want shh", got["secret_access_key"])
	}
}

// TestCredentialsAPI_DecodesWireFormat verifies credentialsAPI decodes the
// /api/current response shape where "provider" is an object carrying the
// type discriminator and settings inline.
func TestCredentialsAPI_DecodesWireFormat(t *testing.T) {
	raw := []byte(`{"id": 1, "name": "tf-probe-cscreds", "provider": {"type": "STORJ_IX", "access_key_id": "x", "secret_access_key": "y"}}`)

	var api credentialsAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if api.Provider["type"] != "STORJ_IX" {
		t.Errorf("api.Provider[type] = %v, want STORJ_IX", api.Provider["type"])
	}
	if api.Provider["access_key_id"] != "x" {
		t.Errorf("api.Provider[access_key_id] = %v, want x", api.Provider["access_key_id"])
	}
}

// TestSchema_ProviderConfigSensitiveRequired verifies that provider_config
// is Required and Sensitive.
func TestSchema_ProviderConfigSensitiveRequired(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["provider_config"]
	if !ok {
		t.Fatal("schema missing 'provider_config' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'provider_config' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsRequired() {
		t.Error("'provider_config' should be Required")
	}
	if !strAttr.IsSensitive() {
		t.Error("'provider_config' should be Sensitive")
	}
}

// TestSchema_IDComputedUseStateForUnknown verifies that the "id" attribute
// is Computed with a UseStateForUnknown plan modifier.
func TestSchema_IDComputedUseStateForUnknown(t *testing.T) {
	s := resourceSchema()

	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	idInt64, ok := idAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.Int64Attribute", idAttr)
	}
	if !idInt64.IsComputed() {
		t.Error("'id' should be Computed")
	}
	if idInt64.IsRequired() || idInt64.IsOptional() {
		t.Error("'id' should be Computed-only (not Required/Optional)")
	}

	foundUseState := false
	for _, pm := range idInt64.PlanModifiers {
		if pm.Description(context.Background()) == int64planmodifier.UseStateForUnknown().Description(context.Background()) {
			foundUseState = true
		}
	}
	if !foundUseState {
		t.Error("'id' should have a UseStateForUnknown plan modifier")
	}
}

// TestSchema_NameRequired verifies that "name" is a Required StringAttribute.
func TestSchema_NameRequired(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["name"]
	if !ok {
		t.Fatal("schema missing 'name' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'name' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsRequired() {
		t.Error("'name' should be Required")
	}
}

// TestProviderDrifted_EndpointTrailingSlash verifies a trailing-slash-only
// endpoint difference (TrueNAS normalizes the S3 endpoint on read-back) is not
// treated as drift, while a genuinely different endpoint still is.
func TestProviderDrifted_EndpointTrailingSlash(t *testing.T) {
	cases := []struct {
		name      string
		stateEP   string
		apiEP     string
		wantDrift bool
	}{
		{"trailing slash added by server", "http://h:9000", "http://h:9000/", false},
		{"identical", "http://h:9000/", "http://h:9000/", false},
		{"genuinely different host", "http://h:9000", "http://other:9000/", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := map[string]any{"type": "S3", "access_key_id": "AKIA...", "endpoint": tc.stateEP}
			api := map[string]any{"type": "S3", "access_key_id": "AKIA...", "endpoint": tc.apiEP}
			if got := providerDrifted(st, api); got != tc.wantDrift {
				t.Errorf("providerDrifted = %v, want %v", got, tc.wantDrift)
			}
		})
	}
}

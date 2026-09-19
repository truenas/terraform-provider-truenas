// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func validCloudSyncModel() CloudSyncModel {
	return CloudSyncModel{
		Description:  types.StringValue("my-task"),
		Path:         types.StringValue("/mnt/tank/data"),
		Credentials:  types.Int64Value(5),
		Direction:    types.StringValue("PUSH"),
		TransferMode: types.StringValue("SYNC"),
		Attributes:   types.StringValue(`{"bucket": "my-bucket", "folder": "backups"}`),
		Schedule: ScheduleModel{
			Minute: types.StringValue("0"),
			Hour:   types.StringValue("0"),
			Dom:    types.StringValue("*"),
			Month:  types.StringValue("*"),
			Dow:    types.StringValue("*"),
		},
		Enabled:    types.BoolValue(true),
		Snapshot:   types.BoolValue(false),
		Include:    types.ListValueMust(types.StringType, []attr.Value{}),
		Exclude:    types.ListValueMust(types.StringType, []attr.Value{}),
		PreScript:  types.StringValue(""),
		PostScript: types.StringValue(""),
	}
}

// TestApiPayload_IncludesScheduleMap verifies apiPayload produces a
// schedule map with all 5 required keys and correct values.
func TestApiPayload_IncludesScheduleMap(t *testing.T) {
	ctx := context.Background()
	m := validCloudSyncModel()

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}

	requiredKeys := []string{
		"description", "path", "credentials", "direction", "transfer_mode",
		"attributes", "schedule", "enabled", "snapshot", "include", "exclude",
		"pre_script", "post_script",
	}
	for _, key := range requiredKeys {
		if _, ok := payload[key]; !ok {
			t.Errorf("payload missing required key %q", key)
		}
	}

	sched, ok := payload["schedule"].(map[string]string)
	if !ok {
		t.Fatalf("payload[schedule] is %T, want map[string]string", payload["schedule"])
	}
	for _, field := range []string{"minute", "hour", "dom", "month", "dow"} {
		if _, ok := sched[field]; !ok {
			t.Errorf("schedule map missing key %q", field)
		}
	}
	if sched["minute"] != "0" {
		t.Errorf("schedule.minute = %q, want \"0\"", sched["minute"])
	}
	if sched["dom"] != "*" {
		t.Errorf("schedule.dom = %q, want \"*\"", sched["dom"])
	}

	if payload["credentials"] != int64(5) {
		t.Errorf("payload[credentials] = %v, want 5", payload["credentials"])
	}
}

// TestApiPayload_InvalidAttributesJSON verifies that malformed attributes
// JSON produces an error diagnostic and a nil payload.
func TestApiPayload_InvalidAttributesJSON(t *testing.T) {
	ctx := context.Background()
	m := validCloudSyncModel()
	m.Attributes = types.StringValue("{not valid json")

	payload, diags := m.apiPayload(ctx)
	if !diags.HasError() {
		t.Fatal("expected error diagnostics for invalid attributes JSON, got none")
	}
	if payload != nil {
		t.Errorf("expected nil payload on error, got %v", payload)
	}
}

// TestDecodeCredentialsID_Object verifies decoding when credentials is an
// embedded object: {"id": 5, ...}.
func TestDecodeCredentialsID_Object(t *testing.T) {
	raw := json.RawMessage(`{"id": 5, "name": "my-creds", "provider": "S3"}`)

	id, err := decodeCredentialsID(raw)
	if err != nil {
		t.Fatalf("decodeCredentialsID returned error: %v", err)
	}
	if id != 5 {
		t.Errorf("decodeCredentialsID = %d, want 5", id)
	}
}

// TestDecodeCredentialsID_BareInt verifies decoding when credentials is a
// bare integer: 5.
func TestDecodeCredentialsID_BareInt(t *testing.T) {
	raw := json.RawMessage(`5`)

	id, err := decodeCredentialsID(raw)
	if err != nil {
		t.Fatalf("decodeCredentialsID returned error: %v", err)
	}
	if id != 5 {
		t.Errorf("decodeCredentialsID = %d, want 5", id)
	}
}

// TestDecodeCredentialsID_Invalid verifies decoding fails cleanly for
// unrecognized shapes.
func TestDecodeCredentialsID_Invalid(t *testing.T) {
	raw := json.RawMessage(`"not-an-id"`)

	_, err := decodeCredentialsID(raw)
	if err == nil {
		t.Fatal("expected error for unrecognized credentials shape, got none")
	}
}

// TestDecodeCredentialsID_Null verifies that a JSON null credentials field
// returns an explicit error rather than silently decoding to 0. Without the
// null check, json.Unmarshal([]byte("null"), &obj) succeeds leaving obj's
// zero value, and decodeCredentialsID would wrongly return (0, nil).
func TestDecodeCredentialsID_Null(t *testing.T) {
	raw := json.RawMessage(`null`)

	_, err := decodeCredentialsID(raw)
	if err == nil {
		t.Fatal("expected error for null credentials, got none")
	}
}

// TestDecodeCredentialsID_Empty verifies that an empty (zero-length) raw
// message is also treated as an error, not decoded to 0.
func TestDecodeCredentialsID_Empty(t *testing.T) {
	raw := json.RawMessage(``)

	_, err := decodeCredentialsID(raw)
	if err == nil {
		t.Fatal("expected error for empty credentials, got none")
	}
}

// TestResponseToModel_CredentialsNull verifies that responseToModel surfaces
// an error diagnostic (rather than silently defaulting Credentials to 0)
// when the API returns a null credentials field.
func TestResponseToModel_CredentialsNull(t *testing.T) {
	ctx := context.Background()
	api := &cloudSyncAPI{
		ID:          1,
		Description: "task",
		Credentials: json.RawMessage(`null`),
	}
	var m CloudSyncModel
	diags := responseToModel(ctx, api, &m)
	if !diags.HasError() {
		t.Fatal("expected error diagnostics for null credentials, got none")
	}
}

// TestApiPayload_IncludeExcludeNil verifies that null/unknown include and
// exclude lists produce empty slices, never nil, in the payload.
func TestApiPayload_IncludeExcludeNil(t *testing.T) {
	ctx := context.Background()
	m := validCloudSyncModel()
	m.Include = types.ListNull(types.StringType)
	m.Exclude = types.ListNull(types.StringType)

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}

	include, ok := payload["include"].([]string)
	if !ok {
		t.Fatalf("payload[include] is %T, want []string", payload["include"])
	}
	if include == nil || len(include) != 0 {
		t.Errorf("payload[include] = %v, want empty slice", include)
	}

	exclude, ok := payload["exclude"].([]string)
	if !ok {
		t.Fatalf("payload[exclude] is %T, want []string", payload["exclude"])
	}
	if exclude == nil || len(exclude) != 0 {
		t.Errorf("payload[exclude] = %v, want empty slice", exclude)
	}
}

// TestApiPayload_OmitsUnsetOptionals verifies that enabled, snapshot,
// pre_script, and post_script are omitted from the payload when null/unknown
// (the plan-modifier state for unset Optional+Computed attributes), rather
// than being sent as their Go zero values (false/"").
func TestApiPayload_OmitsUnsetOptionals(t *testing.T) {
	ctx := context.Background()
	m := validCloudSyncModel()
	m.Enabled = types.BoolNull()
	m.Snapshot = types.BoolUnknown()
	m.PreScript = types.StringNull()
	m.PostScript = types.StringUnknown()

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}

	for _, key := range []string{"enabled", "snapshot", "pre_script", "post_script"} {
		if _, ok := payload[key]; ok {
			t.Errorf("payload should not contain unset optional key %q, got %v", key, payload[key])
		}
	}
}

// TestResponseToModel_CredentialsObject verifies responseToModel decodes
// an embedded credentials object correctly.
func TestResponseToModel_CredentialsObject(t *testing.T) {
	ctx := context.Background()
	api := &cloudSyncAPI{
		ID:          1,
		Description: "task",
		Credentials: json.RawMessage(`{"id": 7}`),
	}
	var m CloudSyncModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostics errors: %v", diags)
	}
	if m.Credentials.ValueInt64() != 7 {
		t.Errorf("m.Credentials = %d, want 7", m.Credentials.ValueInt64())
	}
}

// TestResponseToModel_CredentialsBareInt verifies responseToModel decodes
// a bare integer credentials field correctly.
func TestResponseToModel_CredentialsBareInt(t *testing.T) {
	ctx := context.Background()
	api := &cloudSyncAPI{
		ID:          1,
		Description: "task",
		Credentials: json.RawMessage(`7`),
	}
	var m CloudSyncModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostics errors: %v", diags)
	}
	if m.Credentials.ValueInt64() != 7 {
		t.Errorf("m.Credentials = %d, want 7", m.Credentials.ValueInt64())
	}
}

// --- attributes drift tests (copy pattern from vm_device) ---

func TestAttributesDrifted_Identical(t *testing.T) {
	state := map[string]any{"bucket": "b1", "folder": "f1"}
	api := map[string]any{"bucket": "b1", "folder": "f1"}

	if attributesDrifted(state, api) {
		t.Error("attributesDrifted() = true for identical maps, want false")
	}
}

func TestAttributesDrifted_ChangedValue(t *testing.T) {
	state := map[string]any{"bucket": "b1", "folder": "f1"}
	api := map[string]any{"bucket": "b1", "folder": "f2"}

	if !attributesDrifted(state, api) {
		t.Error("attributesDrifted() = false for changed value, want true")
	}
}

func TestAttributesDrifted_APIAddedKey(t *testing.T) {
	state := map[string]any{"bucket": "b1"}
	api := map[string]any{"bucket": "b1", "region": "us-east-1"}

	if attributesDrifted(state, api) {
		t.Error("attributesDrifted() = true for API-added extra key, want false")
	}
}

func TestAttributesMap_InvalidJSON(t *testing.T) {
	m := CloudSyncModel{Attributes: types.StringValue("{not valid json")}

	attrs, diags := m.attributesMap()
	if !diags.HasError() {
		t.Fatal("expected error diagnostics for invalid JSON, got none")
	}
	if attrs != nil {
		t.Errorf("expected nil attrs on error, got %v", attrs)
	}
}

// --- schema tests ---

func TestCloudSyncSchema_AttributesRequired(t *testing.T) {
	s := resourceSchema()

	a, ok := s.Attributes["attributes"]
	if !ok {
		t.Fatal("schema missing 'attributes' attribute")
	}
	strAttr, ok := a.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'attributes' attribute is %T, want schema.StringAttribute", a)
	}
	if !strAttr.IsRequired() {
		t.Error("'attributes' should be Required")
	}
}

func TestCloudSyncSchema_DescriptionRequired(t *testing.T) {
	s := resourceSchema()

	a, ok := s.Attributes["description"]
	if !ok {
		t.Fatal("schema missing 'description' attribute")
	}
	strAttr, ok := a.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'description' attribute is %T, want schema.StringAttribute", a)
	}
	if !strAttr.IsRequired() {
		t.Error("'description' should be Required")
	}
}

func TestCloudSyncSchema_ScheduleNestedRequired(t *testing.T) {
	s := resourceSchema()

	schedAttr, ok := s.Attributes["schedule"]
	if !ok {
		t.Fatal("schema missing 'schedule' attribute")
	}
	nested, ok := schedAttr.(schema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("'schedule' attribute is %T, want schema.SingleNestedAttribute", schedAttr)
	}
	if !nested.IsRequired() {
		t.Error("'schedule' should be Required")
	}
	for _, field := range []string{"minute", "hour", "dom", "month", "dow"} {
		a, exists := nested.Attributes[field]
		if !exists {
			t.Errorf("schedule missing field %q", field)
			continue
		}
		sa, ok := a.(schema.StringAttribute)
		if !ok {
			t.Errorf("schedule.%s is %T, want schema.StringAttribute", field, a)
			continue
		}
		if !sa.IsRequired() {
			t.Errorf("schedule.%s should be Required", field)
		}
	}
}

func TestCloudSyncSchema_CredentialsRequired(t *testing.T) {
	s := resourceSchema()

	a, ok := s.Attributes["credentials"]
	if !ok {
		t.Fatal("schema missing 'credentials' attribute")
	}
	intAttr, ok := a.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'credentials' attribute is %T, want schema.Int64Attribute", a)
	}
	if !intAttr.IsRequired() {
		t.Error("'credentials' should be Required")
	}
}

func TestCloudSyncSchema_EnabledOptionalComputed(t *testing.T) {
	s := resourceSchema()

	a, ok := s.Attributes["enabled"]
	if !ok {
		t.Fatal("schema missing 'enabled' attribute")
	}
	boolAttr, ok := a.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'enabled' attribute is %T, want schema.BoolAttribute", a)
	}
	if !boolAttr.IsOptional() || !boolAttr.IsComputed() {
		t.Error("'enabled' should be Optional and Computed")
	}
}

func TestCloudSyncSchema_IncludeExcludeOptionalComputed(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"include", "exclude"} {
		a, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		listAttr, ok := a.(schema.ListAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.ListAttribute", name, a)
		}
		if !listAttr.IsOptional() || !listAttr.IsComputed() {
			t.Errorf("%q should be Optional and Computed", name)
		}
	}
}

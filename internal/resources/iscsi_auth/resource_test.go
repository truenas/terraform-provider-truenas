package iscsi_auth

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestISCSIAuthSchema verifies that the resource schema has the expected
// attributes, types, and Required/Optional/Computed/Sensitive flags.
func TestISCSIAuthSchema(t *testing.T) {
	s := resourceSchema()

	// id must be Int64Attribute, Computed-only.
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

	// tag must be Int64Attribute, Required.
	tagAttr, ok := s.Attributes["tag"]
	if !ok {
		t.Fatal("schema missing 'tag' attribute")
	}
	tagInt64, ok := tagAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'tag' attribute is %T, want schema.Int64Attribute", tagAttr)
	}
	if !tagInt64.IsRequired() {
		t.Error("'tag' should be Required")
	}

	// user must be StringAttribute, Required.
	userAttr, ok := s.Attributes["user"]
	if !ok {
		t.Fatal("schema missing 'user' attribute")
	}
	userStr, ok := userAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'user' attribute is %T, want schema.StringAttribute", userAttr)
	}
	if !userStr.IsRequired() {
		t.Error("'user' should be Required")
	}

	// secret must be StringAttribute, Required and Sensitive.
	secretAttr, ok := s.Attributes["secret"]
	if !ok {
		t.Fatal("schema missing 'secret' attribute")
	}
	secretStr, ok := secretAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'secret' attribute is %T, want schema.StringAttribute", secretAttr)
	}
	if !secretStr.IsRequired() {
		t.Error("'secret' should be Required")
	}
	if !secretStr.IsSensitive() {
		t.Error("'secret' should be Sensitive")
	}
	if secretStr.IsComputed() {
		t.Error("'secret' should not be Computed")
	}

	// peeruser must be StringAttribute, Optional+Computed.
	peerUserAttr, ok := s.Attributes["peeruser"]
	if !ok {
		t.Fatal("schema missing 'peeruser' attribute")
	}
	peerUserStr, ok := peerUserAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'peeruser' attribute is %T, want schema.StringAttribute", peerUserAttr)
	}
	if !peerUserStr.IsOptional() {
		t.Error("'peeruser' should be Optional")
	}
	if !peerUserStr.IsComputed() {
		t.Error("'peeruser' should be Computed")
	}

	// peersecret must be StringAttribute, Optional, Sensitive, NOT Computed.
	peerSecretAttr, ok := s.Attributes["peersecret"]
	if !ok {
		t.Fatal("schema missing 'peersecret' attribute")
	}
	peerSecretStr, ok := peerSecretAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'peersecret' attribute is %T, want schema.StringAttribute", peerSecretAttr)
	}
	if !peerSecretStr.IsOptional() {
		t.Error("'peersecret' should be Optional")
	}
	if !peerSecretStr.IsSensitive() {
		t.Error("'peersecret' should be Sensitive")
	}
	if peerSecretStr.IsComputed() {
		t.Error("'peersecret' should NOT be Computed")
	}
	if peerSecretStr.IsRequired() {
		t.Error("'peersecret' should not be Required")
	}

	// discovery_auth must be StringAttribute, Optional+Computed.
	discAuthAttr, ok := s.Attributes["discovery_auth"]
	if !ok {
		t.Fatal("schema missing 'discovery_auth' attribute")
	}
	discAuthStr, ok := discAuthAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'discovery_auth' attribute is %T, want schema.StringAttribute", discAuthAttr)
	}
	if !discAuthStr.IsOptional() {
		t.Error("'discovery_auth' should be Optional")
	}
	if !discAuthStr.IsComputed() {
		t.Error("'discovery_auth' should be Computed")
	}
}

// TestISCSIAuthApiPayload_AlwaysIncludesRequired verifies that tag/user/secret
// are always present in the payload, and that optional fields are omitted
// when null/unknown.
func TestISCSIAuthApiPayload_AlwaysIncludesRequired(t *testing.T) {
	m := ISCSIAuthModel{
		Tag:           types.Int64Value(5),
		User:          types.StringValue("chapuser"),
		Secret:        types.StringValue("chapsecret123"),
		PeerUser:      types.StringNull(),
		PeerSecret:    types.StringNull(),
		DiscoveryAuth: types.StringNull(),
	}

	payload := m.apiPayload()

	if payload["tag"] != int64(5) {
		t.Errorf("payload[tag] = %v, want 5", payload["tag"])
	}
	if payload["user"] != "chapuser" {
		t.Errorf("payload[user] = %v, want 'chapuser'", payload["user"])
	}
	if payload["secret"] != "chapsecret123" {
		t.Errorf("payload[secret] = %v, want 'chapsecret123'", payload["secret"])
	}

	if _, ok := payload["peeruser"]; ok {
		t.Error("payload should not contain 'peeruser' when null")
	}
	if _, ok := payload["peersecret"]; ok {
		t.Error("payload should not contain 'peersecret' when null")
	}
	if _, ok := payload["discovery_auth"]; ok {
		t.Error("payload should not contain 'discovery_auth' when null")
	}
	if _, ok := payload["id"]; ok {
		t.Error("payload should not contain 'id'")
	}
}

// TestISCSIAuthApiPayload_OptionalFieldsIncludedWhenKnown verifies that
// peeruser/discovery_auth are included when known, and peersecret is
// included when known and non-empty.
func TestISCSIAuthApiPayload_OptionalFieldsIncludedWhenKnown(t *testing.T) {
	m := ISCSIAuthModel{
		Tag:           types.Int64Value(1),
		User:          types.StringValue("u"),
		Secret:        types.StringValue("s"),
		PeerUser:      types.StringValue("peer"),
		PeerSecret:    types.StringValue("peersecretvalue"),
		DiscoveryAuth: types.StringValue("CHAP_MUTUAL"),
	}

	payload := m.apiPayload()

	if payload["peeruser"] != "peer" {
		t.Errorf("payload[peeruser] = %v, want 'peer'", payload["peeruser"])
	}
	if payload["peersecret"] != "peersecretvalue" {
		t.Errorf("payload[peersecret] = %v, want 'peersecretvalue'", payload["peersecret"])
	}
	if payload["discovery_auth"] != "CHAP_MUTUAL" {
		t.Errorf("payload[discovery_auth] = %v, want 'CHAP_MUTUAL'", payload["discovery_auth"])
	}
}

// TestISCSIAuthApiPayload_EmptyPeerSecretOmitted verifies that a known but
// empty peersecret is NOT included in the payload (guards against clearing
// mutual CHAP on the server unintentionally / matches "only when non-empty").
func TestISCSIAuthApiPayload_EmptyPeerSecretOmitted(t *testing.T) {
	m := ISCSIAuthModel{
		Tag:        types.Int64Value(1),
		User:       types.StringValue("u"),
		Secret:     types.StringValue("s"),
		PeerSecret: types.StringValue(""),
	}

	payload := m.apiPayload()

	if _, ok := payload["peersecret"]; ok {
		t.Error("payload should not contain 'peersecret' when it is an empty string")
	}
}

// TestISCSIAuthApiPayload_UnknownOptionalFieldsOmitted verifies that unknown
// (not yet resolved) optional fields are omitted from the payload.
func TestISCSIAuthApiPayload_UnknownOptionalFieldsOmitted(t *testing.T) {
	m := ISCSIAuthModel{
		Tag:           types.Int64Value(1),
		User:          types.StringValue("u"),
		Secret:        types.StringValue("s"),
		PeerUser:      types.StringUnknown(),
		PeerSecret:    types.StringUnknown(),
		DiscoveryAuth: types.StringUnknown(),
	}

	payload := m.apiPayload()

	if _, ok := payload["peeruser"]; ok {
		t.Error("payload should not contain 'peeruser' when unknown")
	}
	if _, ok := payload["peersecret"]; ok {
		t.Error("payload should not contain 'peersecret' when unknown")
	}
	if _, ok := payload["discovery_auth"]; ok {
		t.Error("payload should not contain 'discovery_auth' when unknown")
	}
}

// TestISCSIAuthResponseToModel verifies field mapping when the API returns
// non-empty values for everything.
func TestISCSIAuthResponseToModel(t *testing.T) {
	api := &iscsiAuthAPI{
		ID:            7,
		Tag:           3,
		User:          "chapuser",
		Secret:        "apisecret",
		PeerUser:      "peeruser",
		PeerSecret:    "apipeersecret",
		DiscoveryAuth: "CHAP",
	}

	var m ISCSIAuthModel
	diags := responseToModel(api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if m.ID.ValueInt64() != 7 {
		t.Errorf("ID = %v, want 7", m.ID.ValueInt64())
	}
	if m.Tag.ValueInt64() != 3 {
		t.Errorf("Tag = %v, want 3", m.Tag.ValueInt64())
	}
	if m.User.ValueString() != "chapuser" {
		t.Errorf("User = %v, want 'chapuser'", m.User.ValueString())
	}
	if m.Secret.ValueString() != "apisecret" {
		t.Errorf("Secret = %v, want 'apisecret' (API returned non-empty, should take API value)", m.Secret.ValueString())
	}
	if m.PeerUser.ValueString() != "peeruser" {
		t.Errorf("PeerUser = %v, want 'peeruser'", m.PeerUser.ValueString())
	}
	if m.PeerSecret.ValueString() != "apipeersecret" {
		t.Errorf("PeerSecret = %v, want 'apipeersecret' (API returned non-empty, should take API value)", m.PeerSecret.ValueString())
	}
	if m.DiscoveryAuth.ValueString() != "CHAP" {
		t.Errorf("DiscoveryAuth = %v, want 'CHAP'", m.DiscoveryAuth.ValueString())
	}
}

// TestISCSIAuthResponseToModel_SecretPreservation verifies that when the API
// returns an empty secret/peersecret but the model already holds a non-empty
// value (e.g. set by the user in a prior plan), the model's value is
// preserved rather than being wiped out.
func TestISCSIAuthResponseToModel_SecretPreservation(t *testing.T) {
	api := &iscsiAuthAPI{
		ID:            1,
		Tag:           1,
		User:          "u",
		Secret:        "", // API masked/omitted the secret
		PeerUser:      "peer",
		PeerSecret:    "", // API masked/omitted the peer secret
		DiscoveryAuth: "NONE",
	}

	m := ISCSIAuthModel{
		Secret:     types.StringValue("user-set-secret"),
		PeerSecret: types.StringValue("user-set-peersecret"),
	}

	diags := responseToModel(api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if m.Secret.ValueString() != "user-set-secret" {
		t.Errorf("Secret = %v, want 'user-set-secret' (API returned empty, should keep model value)", m.Secret.ValueString())
	}
	if m.PeerSecret.ValueString() != "user-set-peersecret" {
		t.Errorf("PeerSecret = %v, want 'user-set-peersecret' (API returned empty, should keep model value)", m.PeerSecret.ValueString())
	}
}

// TestISCSIAuthResponseToModel_EmptyApiAndEmptyModel verifies that when both
// the API and the model have no secret value, the result is an empty (but
// known, non-null) string rather than null/unknown.
func TestISCSIAuthResponseToModel_EmptyApiAndEmptyModel(t *testing.T) {
	api := &iscsiAuthAPI{
		ID:            1,
		Tag:           1,
		User:          "u",
		Secret:        "",
		PeerUser:      "",
		PeerSecret:    "",
		DiscoveryAuth: "NONE",
	}

	var m ISCSIAuthModel // Secret/PeerSecret start as null zero values

	diags := responseToModel(api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if m.Secret.IsNull() || m.Secret.IsUnknown() {
		t.Error("Secret should be a known, non-null empty string, not null/unknown")
	}
	if m.Secret.ValueString() != "" {
		t.Errorf("Secret = %v, want ''", m.Secret.ValueString())
	}
	if m.PeerSecret.IsNull() || m.PeerSecret.IsUnknown() {
		t.Error("PeerSecret should be a known, non-null empty string, not null/unknown")
	}
}

// TestISCSIAuthDataSourceModel_NoSecretFields verifies (via reflection) that
// ISCSIAuthDataSourceModel has no "secret" or "peersecret" fields, so CHAP
// secrets can never be exposed through the datasource.
func TestISCSIAuthDataSourceModel_NoSecretFields(t *testing.T) {
	modelType := reflect.TypeOf(ISCSIAuthDataSourceModel{})
	for i := 0; i < modelType.NumField(); i++ {
		tag := modelType.Field(i).Tag.Get("tfsdk")
		if tag == "secret" || tag == "peersecret" {
			t.Errorf("ISCSIAuthDataSourceModel must not have a %q field (secrets must never be exposed via datasource)", tag)
		}
	}
}

// TestISCSIAuthDataSourceModel_MatchesSchema verifies that every tfsdk tag on
// ISCSIAuthDataSourceModel has a corresponding attribute in the datasource
// schema, and vice versa.
func TestISCSIAuthDataSourceModel_MatchesSchema(t *testing.T) {
	d := &ISCSIAuthDataSource{}
	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	schemaAttrs := make([]string, 0, len(resp.Schema.Attributes))
	for name := range resp.Schema.Attributes {
		schemaAttrs = append(schemaAttrs, name)
	}
	sort.Strings(schemaAttrs)

	modelType := reflect.TypeOf(ISCSIAuthDataSourceModel{})
	modelFields := make([]string, 0, modelType.NumField())
	for i := 0; i < modelType.NumField(); i++ {
		tag := modelType.Field(i).Tag.Get("tfsdk")
		if tag == "" {
			t.Fatalf("field %q has no tfsdk tag", modelType.Field(i).Name)
		}
		modelFields = append(modelFields, tag)
	}
	sort.Strings(modelFields)

	if !reflect.DeepEqual(schemaAttrs, modelFields) {
		t.Fatalf("ISCSIAuthDataSourceModel tfsdk tags %v do not match datasource schema attributes %v", modelFields, schemaAttrs)
	}
}

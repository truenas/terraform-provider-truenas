// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_host

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestNVMetHostSchema verifies that the resource schema has the expected
// attributes, types, and Required/Optional/Computed/Sensitive flags.
func TestNVMetHostSchema(t *testing.T) {
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

	// hostnqn must be StringAttribute, Required.
	hostnqnAttr, ok := s.Attributes["hostnqn"]
	if !ok {
		t.Fatal("schema missing 'hostnqn' attribute")
	}
	hostnqnStr, ok := hostnqnAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'hostnqn' attribute is %T, want schema.StringAttribute", hostnqnAttr)
	}
	if !hostnqnStr.IsRequired() {
		t.Error("'hostnqn' should be Required")
	}

	// description must be StringAttribute, Optional+Computed.
	descAttr, ok := s.Attributes["description"]
	if !ok {
		t.Fatal("schema missing 'description' attribute")
	}
	descStr, ok := descAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'description' attribute is %T, want schema.StringAttribute", descAttr)
	}
	if !descStr.IsOptional() || !descStr.IsComputed() {
		t.Error("'description' should be Optional and Computed")
	}

	// dhchap_key must be StringAttribute, Optional, Sensitive, NOT Computed.
	keyAttr, ok := s.Attributes["dhchap_key"]
	if !ok {
		t.Fatal("schema missing 'dhchap_key' attribute")
	}
	keyStr, ok := keyAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'dhchap_key' attribute is %T, want schema.StringAttribute", keyAttr)
	}
	if !keyStr.IsOptional() {
		t.Error("'dhchap_key' should be Optional")
	}
	if !keyStr.IsSensitive() {
		t.Error("'dhchap_key' should be Sensitive")
	}
	if keyStr.IsComputed() {
		t.Error("'dhchap_key' should NOT be Computed")
	}
	if !keyStr.IsWriteOnly() {
		t.Error("'dhchap_key' should be WriteOnly")
	}
	if keyStr.IsRequired() {
		t.Error("'dhchap_key' should not be Required")
	}

	// dhchap_ctrl_key must be StringAttribute, Optional, Sensitive, NOT Computed.
	ctrlKeyAttr, ok := s.Attributes["dhchap_ctrl_key"]
	if !ok {
		t.Fatal("schema missing 'dhchap_ctrl_key' attribute")
	}
	ctrlKeyStr, ok := ctrlKeyAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'dhchap_ctrl_key' attribute is %T, want schema.StringAttribute", ctrlKeyAttr)
	}
	if !ctrlKeyStr.IsOptional() {
		t.Error("'dhchap_ctrl_key' should be Optional")
	}
	if !ctrlKeyStr.IsSensitive() {
		t.Error("'dhchap_ctrl_key' should be Sensitive")
	}
	if ctrlKeyStr.IsComputed() {
		t.Error("'dhchap_ctrl_key' should NOT be Computed")
	}
	if !ctrlKeyStr.IsWriteOnly() {
		t.Error("'dhchap_ctrl_key' should be WriteOnly")
	}
	if ctrlKeyStr.IsRequired() {
		t.Error("'dhchap_ctrl_key' should not be Required")
	}

	// dhchap_dhgroup must be StringAttribute, Optional+Computed.
	dhgroupAttr, ok := s.Attributes["dhchap_dhgroup"]
	if !ok {
		t.Fatal("schema missing 'dhchap_dhgroup' attribute")
	}
	dhgroupStr, ok := dhgroupAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'dhchap_dhgroup' attribute is %T, want schema.StringAttribute", dhgroupAttr)
	}
	if !dhgroupStr.IsOptional() || !dhgroupStr.IsComputed() {
		t.Error("'dhchap_dhgroup' should be Optional and Computed")
	}

	// dhchap_hash must be StringAttribute, Optional+Computed.
	hashAttr, ok := s.Attributes["dhchap_hash"]
	if !ok {
		t.Fatal("schema missing 'dhchap_hash' attribute")
	}
	hashStr, ok := hashAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'dhchap_hash' attribute is %T, want schema.StringAttribute", hashAttr)
	}
	if !hashStr.IsOptional() || !hashStr.IsComputed() {
		t.Error("'dhchap_hash' should be Optional and Computed")
	}
}

// TestNVMetHostCreatePayload_AlwaysIncludesHostNQN verifies that hostnqn is
// always present in the create payload, and that optional/secret fields are
// omitted when null.
func TestNVMetHostCreatePayload_AlwaysIncludesHostNQN(t *testing.T) {
	m := NVMetHostModel{
		HostNQN:       types.StringValue("nqn.2014-08.org.nvmexpress:uuid:test"),
		Description:   types.StringNull(),
		DHChapKey:     types.StringNull(),
		DHChapCtrlKey: types.StringNull(),
		DHChapDHGroup: types.StringNull(),
		DHChapHash:    types.StringNull(),
	}

	payload, diags := m.createPayload(context.Background())
	if diags.HasError() {
		t.Fatalf("createPayload returned diagnostic errors: %v", diags)
	}

	if payload["hostnqn"] != "nqn.2014-08.org.nvmexpress:uuid:test" {
		t.Errorf("payload[hostnqn] = %v, want the configured NQN", payload["hostnqn"])
	}
	if _, ok := payload["description"]; ok {
		t.Error("payload should not contain 'description' when null")
	}
	if _, ok := payload["dhchap_key"]; ok {
		t.Error("payload should not contain 'dhchap_key' when null")
	}
	if _, ok := payload["dhchap_ctrl_key"]; ok {
		t.Error("payload should not contain 'dhchap_ctrl_key' when null")
	}
	if _, ok := payload["dhchap_dhgroup"]; ok {
		t.Error("payload should not contain 'dhchap_dhgroup' when null")
	}
	if _, ok := payload["dhchap_hash"]; ok {
		t.Error("payload should not contain 'dhchap_hash' when null")
	}
	if _, ok := payload["id"]; ok {
		t.Error("payload should not contain 'id'")
	}
}

// TestNVMetHostCreatePayload_OptionalFieldsIncludedWhenKnown verifies that
// description/dhchap_dhgroup/dhchap_hash and the secrets are included when
// known and non-empty.
func TestNVMetHostCreatePayload_OptionalFieldsIncludedWhenKnown(t *testing.T) {
	m := NVMetHostModel{
		HostNQN:       types.StringValue("nqn.2014-08.org.nvmexpress:uuid:test"),
		Description:   types.StringValue("test host"),
		DHChapKey:     types.StringValue("DHHC-1:00:abcd"),
		DHChapCtrlKey: types.StringValue("DHHC-1:00:efgh"),
		DHChapDHGroup: types.StringValue("2048-bit"),
		DHChapHash:    types.StringValue("SHA-256"),
	}

	payload, diags := m.createPayload(context.Background())
	if diags.HasError() {
		t.Fatalf("createPayload returned diagnostic errors: %v", diags)
	}

	if payload["description"] != "test host" {
		t.Errorf("payload[description] = %v, want 'test host'", payload["description"])
	}
	if payload["dhchap_key"] != "DHHC-1:00:abcd" {
		t.Errorf("payload[dhchap_key] = %v, want 'DHHC-1:00:abcd'", payload["dhchap_key"])
	}
	if payload["dhchap_ctrl_key"] != "DHHC-1:00:efgh" {
		t.Errorf("payload[dhchap_ctrl_key] = %v, want 'DHHC-1:00:efgh'", payload["dhchap_ctrl_key"])
	}
	if payload["dhchap_dhgroup"] != "2048-bit" {
		t.Errorf("payload[dhchap_dhgroup] = %v, want '2048-bit'", payload["dhchap_dhgroup"])
	}
	if payload["dhchap_hash"] != "SHA-256" {
		t.Errorf("payload[dhchap_hash] = %v, want 'SHA-256'", payload["dhchap_hash"])
	}
}

// TestNVMetHostCreatePayload_EmptySecretsOmitted verifies that known but
// empty dhchap_key/dhchap_ctrl_key are NOT included in the payload (guards
// against clearing/rewriting an existing secret unintentionally).
func TestNVMetHostCreatePayload_EmptySecretsOmitted(t *testing.T) {
	m := NVMetHostModel{
		HostNQN:       types.StringValue("nqn.2014-08.org.nvmexpress:uuid:test"),
		DHChapKey:     types.StringValue(""),
		DHChapCtrlKey: types.StringValue(""),
	}

	payload, diags := m.createPayload(context.Background())
	if diags.HasError() {
		t.Fatalf("createPayload returned diagnostic errors: %v", diags)
	}

	if _, ok := payload["dhchap_key"]; ok {
		t.Error("payload should not contain 'dhchap_key' when it is an empty string")
	}
	if _, ok := payload["dhchap_ctrl_key"]; ok {
		t.Error("payload should not contain 'dhchap_ctrl_key' when it is an empty string")
	}
}

// TestNVMetHostCreatePayload_EmptyDHGroupOmitted verifies that a known but
// empty dhchap_dhgroup is NOT included in the payload.
func TestNVMetHostCreatePayload_EmptyDHGroupOmitted(t *testing.T) {
	m := NVMetHostModel{
		HostNQN:       types.StringValue("nqn.2014-08.org.nvmexpress:uuid:test"),
		DHChapDHGroup: types.StringValue(""),
	}

	payload, diags := m.createPayload(context.Background())
	if diags.HasError() {
		t.Fatalf("createPayload returned diagnostic errors: %v", diags)
	}

	if _, ok := payload["dhchap_dhgroup"]; ok {
		t.Error("payload should not contain 'dhchap_dhgroup' when it is an empty string")
	}
}

// TestNVMetHostCreatePayload_UnknownOptionalFieldsOmitted verifies that
// unknown (not yet resolved) optional fields are omitted from the payload.
func TestNVMetHostCreatePayload_UnknownOptionalFieldsOmitted(t *testing.T) {
	m := NVMetHostModel{
		HostNQN:       types.StringValue("nqn.2014-08.org.nvmexpress:uuid:test"),
		Description:   types.StringUnknown(),
		DHChapKey:     types.StringUnknown(),
		DHChapCtrlKey: types.StringUnknown(),
		DHChapDHGroup: types.StringUnknown(),
		DHChapHash:    types.StringUnknown(),
	}

	payload, diags := m.createPayload(context.Background())
	if diags.HasError() {
		t.Fatalf("createPayload returned diagnostic errors: %v", diags)
	}

	for _, key := range []string{"description", "dhchap_key", "dhchap_ctrl_key", "dhchap_dhgroup", "dhchap_hash"} {
		if _, ok := payload[key]; ok {
			t.Errorf("payload should not contain %q when unknown", key)
		}
	}
}

// TestNVMetHostUpdatePayload_AlwaysIncludesHostNQN verifies that hostnqn is
// included in the update payload (it is updatable per the API and Required,
// so it is always known).
func TestNVMetHostUpdatePayload_AlwaysIncludesHostNQN(t *testing.T) {
	m := NVMetHostModel{
		HostNQN: types.StringValue("nqn.2014-08.org.nvmexpress:uuid:updated"),
	}

	payload, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("updatePayload returned diagnostic errors: %v", diags)
	}

	if payload["hostnqn"] != "nqn.2014-08.org.nvmexpress:uuid:updated" {
		t.Errorf("payload[hostnqn] = %v, want the updated NQN", payload["hostnqn"])
	}
}

// TestNVMetHostResponseToModel verifies field mapping when the API returns
// non-empty values for everything. DHChapKey/DHChapCtrlKey are write-only
// and must never be assigned from the API response.
func TestNVMetHostResponseToModel(t *testing.T) {
	dhgroup := "2048-bit"
	api := &nvmetHostAPI{
		ID:            7,
		HostNQN:       "nqn.2014-08.org.nvmexpress:uuid:test",
		Description:   "test host",
		DHChapDHGroup: &dhgroup,
		DHChapHash:    "SHA-256",
	}

	var m NVMetHostModel
	diags := responseToModel(context.Background(), api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if m.ID.ValueInt64() != 7 {
		t.Errorf("ID = %v, want 7", m.ID.ValueInt64())
	}
	if m.HostNQN.ValueString() != "nqn.2014-08.org.nvmexpress:uuid:test" {
		t.Errorf("HostNQN = %v, want the configured NQN", m.HostNQN.ValueString())
	}
	if m.Description.ValueString() != "test host" {
		t.Errorf("Description = %v, want 'test host'", m.Description.ValueString())
	}
	if m.DHChapDHGroup.ValueString() != "2048-bit" {
		t.Errorf("DHChapDHGroup = %v, want '2048-bit'", m.DHChapDHGroup.ValueString())
	}
	if m.DHChapHash.ValueString() != "SHA-256" {
		t.Errorf("DHChapHash = %v, want 'SHA-256'", m.DHChapHash.ValueString())
	}
	if !m.DHChapKey.IsNull() {
		t.Errorf("DHChapKey = %v, want null (write-only, never read back from API)", m.DHChapKey)
	}
	if !m.DHChapCtrlKey.IsNull() {
		t.Errorf("DHChapCtrlKey = %v, want null (write-only, never read back from API)", m.DHChapCtrlKey)
	}
}

// TestNVMetHostResponseToModel_NilDHGroupMapsToEmpty verifies that a nil
// dhchap_dhgroup from the API maps to an empty string rather than leaving
// the model attribute null/unknown.
func TestNVMetHostResponseToModel_NilDHGroupMapsToEmpty(t *testing.T) {
	api := &nvmetHostAPI{
		ID:            1,
		HostNQN:       "nqn.2014-08.org.nvmexpress:uuid:test",
		Description:   "",
		DHChapDHGroup: nil,
		DHChapHash:    "SHA-256",
	}

	var m NVMetHostModel
	diags := responseToModel(context.Background(), api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if m.DHChapDHGroup.ValueString() != "" {
		t.Errorf("DHChapDHGroup = %v, want empty string when API returns nil", m.DHChapDHGroup.ValueString())
	}
	if m.DHChapDHGroup.IsNull() {
		t.Error("DHChapDHGroup should be a known empty string, not null")
	}
}

// TestNVMetHostResponseToModel_SecretsWriteOnly verifies that
// responseToModel never assigns to DHChapKey/DHChapCtrlKey regardless of
// what the caller already had in the model: set values are left unchanged,
// and null values stay null (never coerced to "").
func TestNVMetHostResponseToModel_SecretsWriteOnly(t *testing.T) {
	dhgroup := "2048-bit"
	api := &nvmetHostAPI{
		ID:            1,
		HostNQN:       "nqn.2014-08.org.nvmexpress:uuid:test",
		Description:   "d",
		DHChapDHGroup: &dhgroup,
		DHChapHash:    "SHA-256",
	}

	// Case 1: model already holds values set by the user in the plan -> left
	// exactly as-is.
	set := NVMetHostModel{
		DHChapKey:     types.StringValue("user-set-key"),
		DHChapCtrlKey: types.StringValue("user-set-ctrl-key"),
	}
	diags := responseToModel(context.Background(), api, &set)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}
	if set.DHChapKey.ValueString() != "user-set-key" {
		t.Errorf("DHChapKey = %v, want unchanged 'user-set-key'", set.DHChapKey.ValueString())
	}
	if set.DHChapCtrlKey.ValueString() != "user-set-ctrl-key" {
		t.Errorf("DHChapCtrlKey = %v, want unchanged 'user-set-ctrl-key'", set.DHChapCtrlKey.ValueString())
	}

	// Case 2: model has null DHChapKey/DHChapCtrlKey (e.g. omitted by the
	// user, or immediately after import) -> stays null, never coerced to "".
	var null NVMetHostModel
	diags = responseToModel(context.Background(), api, &null)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}
	if !null.DHChapKey.IsNull() {
		t.Errorf("DHChapKey = %v, want null (must stay null, not be coerced to \"\")", null.DHChapKey)
	}
	if !null.DHChapCtrlKey.IsNull() {
		t.Errorf("DHChapCtrlKey = %v, want null (must stay null, not be coerced to \"\")", null.DHChapCtrlKey)
	}
}

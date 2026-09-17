// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package keychain_ssh_keypair

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// --- createPayload ---

// TestCreatePayload_GeneratedIncludesBothKeys verifies the generated-key
// path (both private_key and public_key known from
// generate_ssh_key_pair's result) sends both fields.
func TestCreatePayload_GeneratedIncludesBothKeys(t *testing.T) {
	p := createPayload("tf-test", "PRIVATE-PEM", "PUBLIC-LINE")

	if p["name"] != "tf-test" {
		t.Errorf("name = %v, want tf-test", p["name"])
	}
	if p["type"] != keychainCredentialType {
		t.Errorf("type = %v, want %v", p["type"], keychainCredentialType)
	}
	attrs, ok := p["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("attributes = %T, want map[string]any", p["attributes"])
	}
	if attrs["private_key"] != "PRIVATE-PEM" {
		t.Errorf("attributes[private_key] = %v, want PRIVATE-PEM", attrs["private_key"])
	}
	if attrs["public_key"] != "PUBLIC-LINE" {
		t.Errorf("attributes[public_key] = %v, want PUBLIC-LINE", attrs["public_key"])
	}
}

// TestCreatePayload_SuppliedOmitsPublicKey verifies the user-supplied-key
// path (empty publicKey — the caller only has the private key) omits
// "public_key" entirely, letting TrueNAS derive it server-side (probed
// live).
func TestCreatePayload_SuppliedOmitsPublicKey(t *testing.T) {
	p := createPayload("tf-test", "PRIVATE-PEM", "")

	attrs, ok := p["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("attributes = %T, want map[string]any", p["attributes"])
	}
	if _, present := attrs["public_key"]; present {
		t.Error("attributes[public_key] should be omitted when publicKey is empty")
	}
	if attrs["private_key"] != "PRIVATE-PEM" {
		t.Errorf("attributes[private_key] = %v, want PRIVATE-PEM", attrs["private_key"])
	}
}

// --- updatePayload ---

// TestUpdatePayload_NameOnly verifies the update payload sends only
// "name" — private_key/public_key both carry RequiresReplace, so Update is
// never called across a change to either (probed live: a name-only update
// leaves the existing attributes untouched).
func TestUpdatePayload_NameOnly(t *testing.T) {
	m := &KeychainSSHKeyPairModel{Name: types.StringValue("renamed")}
	p := m.updatePayload()

	if len(p) != 1 {
		t.Fatalf("updatePayload() = %v, want exactly 1 key (name)", p)
	}
	if p["name"] != "renamed" {
		t.Errorf("name = %v, want renamed", p["name"])
	}
}

// --- responseToModel ---

// TestResponseToModel_PreservesGenerate verifies responseToModel never
// touches Generate (a provider-side-only field with no wire counterpart) —
// whatever the caller set beforehand survives unchanged.
func TestResponseToModel_PreservesGenerate(t *testing.T) {
	m := &KeychainSSHKeyPairModel{Generate: types.BoolValue(true)}
	api := &keychainSSHKeyPairAPI{
		ID:   7,
		Name: "tf-test",
		Type: keychainCredentialType,
		Attributes: keychainSSHKeyPairAttributesAPI{
			PrivateKey: strPtr("PRIVATE-PEM"),
			PublicKey:  strPtr("PUBLIC-LINE"),
		},
	}

	diags := responseToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if !m.Generate.ValueBool() {
		t.Error("Generate should still be true after responseToModel")
	}
	if m.ID.ValueInt64() != 7 {
		t.Errorf("ID = %v, want 7", m.ID)
	}
	if m.PrivateKey.ValueString() != "PRIVATE-PEM" {
		t.Errorf("PrivateKey = %v, want PRIVATE-PEM", m.PrivateKey)
	}
	if m.PublicKey.ValueString() != "PUBLIC-LINE" {
		t.Errorf("PublicKey = %v, want PUBLIC-LINE", m.PublicKey)
	}
}

func strPtr(s string) *string { return &s }

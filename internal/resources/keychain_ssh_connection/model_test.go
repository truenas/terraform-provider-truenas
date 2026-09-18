// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package keychain_ssh_connection

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func testModel() *KeychainSSHConnectionModel {
	return &KeychainSSHConnectionModel{
		ID:             types.Int64Value(9),
		Name:           types.StringValue("tf-test"),
		Host:           types.StringValue("192.168.1.249"),
		Port:           types.Int64Value(22),
		Username:       types.StringValue("truenas_admin"),
		PrivateKeyID:   types.Int64Value(4),
		RemoteHostKey:  types.StringValue("ssh-ed25519 AAAA..."),
		ConnectTimeout: types.Int64Value(10),
	}
}

// TestCreatePayload verifies createPayload sends the full attributes
// object plus name+type — every SSH_CREDENTIALS field always included,
// since keychaincredential.create's "attributes" is a single required
// object (probed live, no partial variant).
func TestCreatePayload(t *testing.T) {
	p := testModel().createPayload()

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
	wantAttrs := map[string]any{
		"host":            "192.168.1.249",
		"port":            int64(22),
		"username":        "truenas_admin",
		"private_key":     int64(4),
		"remote_host_key": "ssh-ed25519 AAAA...",
		"connect_timeout": int64(10),
	}
	for k, want := range wantAttrs {
		if attrs[k] != want {
			t.Errorf("attributes[%s] = %v, want %v", k, attrs[k], want)
		}
	}
	if len(attrs) != len(wantAttrs) {
		t.Errorf("attributes has %d keys, want %d: %v", len(attrs), len(wantAttrs), attrs)
	}
}

// TestUpdatePayload verifies updatePayload sends the same complete
// attributes object as createPayload (plus name, no "type") —
// keychaincredential.update requires the full attributes value, never a
// partial merge (probed live and per the API's own description).
func TestUpdatePayload(t *testing.T) {
	p := testModel().updatePayload()

	if _, present := p["type"]; present {
		t.Error("updatePayload should not include \"type\" — keychaincredential.update rejects changing it")
	}
	if p["name"] != "tf-test" {
		t.Errorf("name = %v, want tf-test", p["name"])
	}
	attrs, ok := p["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("attributes = %T, want map[string]any", p["attributes"])
	}
	if attrs["host"] != "192.168.1.249" {
		t.Errorf("attributes[host] = %v, want 192.168.1.249", attrs["host"])
	}
	if attrs["private_key"] != int64(4) {
		t.Errorf("attributes[private_key] = %v, want 4", attrs["private_key"])
	}
}

// TestResponseToModel verifies responseToModel maps every API field onto
// the Terraform model, including renaming the wire's "private_key" (a
// keychain credential id, not key material) to PrivateKeyID.
func TestResponseToModel(t *testing.T) {
	api := &keychainSSHConnectionAPI{
		ID:   9,
		Name: "tf-test",
		Type: keychainCredentialType,
		Attributes: keychainSSHConnectionAttributesAPI{
			Host:           "192.168.1.249",
			Port:           2222,
			Username:       "truenas_admin",
			PrivateKey:     4,
			RemoteHostKey:  "ssh-ed25519 AAAA...",
			ConnectTimeout: 15,
		},
	}

	m := &KeychainSSHConnectionModel{}
	diags := responseToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueInt64() != 9 {
		t.Errorf("ID = %v, want 9", m.ID)
	}
	if m.Port.ValueInt64() != 2222 {
		t.Errorf("Port = %v, want 2222", m.Port)
	}
	if m.PrivateKeyID.ValueInt64() != 4 {
		t.Errorf("PrivateKeyID = %v, want 4", m.PrivateKeyID)
	}
	if m.ConnectTimeout.ValueInt64() != 15 {
		t.Errorf("ConnectTimeout = %v, want 15", m.ConnectTimeout)
	}
}

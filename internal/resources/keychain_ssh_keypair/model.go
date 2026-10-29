// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package keychain_ssh_keypair

import (
	"bytes"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

// keychainCredentialType is the fixed "type" discriminator this package's
// resource/datasource always send/expect from keychaincredential.* — probed
// live: the field is immutable ("Please note that you can't change `type`",
// per keychaincredential.update's own description) and this resource never
// manages the other value (SSH_CREDENTIALS — see the sibling
// keychain_ssh_connection package).
const keychainCredentialType = "SSH_KEY_PAIR"

// KeychainSSHKeyPairModel is the Terraform state/plan model for
// truenas_keychain_ssh_keypair.
//
// Generate is a provider-side-only switch (see schema.go's doc comment):
// it selects between two keychaincredential.create paths but has no
// corresponding field in the wire attributes object, so responseToModel
// never touches it.
type KeychainSSHKeyPairModel struct {
	ID         types.Int64       `tfsdk:"id"`
	Name       types.String      `tfsdk:"name"`
	Generate   types.Bool        `tfsdk:"generate"`
	PrivateKey normalize.Trimmed `tfsdk:"private_key"`
	PublicKey  types.String      `tfsdk:"public_key"`
}

// KeychainSSHKeyPairDataSourceModel is the read-only lookup model for the
// truenas_keychain_ssh_keypair data source, looked up by "name"
// (keychaincredential names must be unique across the whole keychain,
// confirmed by the "Distinguishes this Keychain Credential from others"
// schema description — not scoped per type).
type KeychainSSHKeyPairDataSourceModel struct {
	ID         types.Int64  `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	PrivateKey types.String `tfsdk:"private_key"`
	PublicKey  types.String `tfsdk:"public_key"`
}

// keychainSSHKeyPairAttributesAPI mirrors keychaincredential.*'s
// "attributes" object for type=SSH_KEY_PAIR. Probed live against TrueNAS
// TrueNAS 25.10: both fields are nullable on the wire ("at least one of the
// two keys must be provided on creation"), but in every response this
// provider ever reads back (create/get_instance/query/update, whichever
// path was taken — user-supplied private_key or a
// generate_ssh_key_pair-sourced one), both come back populated: TrueNAS
// always derives and echoes "public_key" from "private_key" when the
// caller omits it (confirmed live: creating with only private_key set
// echoed back the exact matching OpenSSH public key line). The individual
// fields are not masked for a privileged session, but the WHOLE attributes
// object is redacted to the string "********" for a session without the
// KEYCHAIN_CREDENTIAL write role — see UnmarshalJSON / Redacted. (#57)
type keychainSSHKeyPairAttributesAPI struct {
	PrivateKey *string `json:"private_key"`
	PublicKey  *string `json:"public_key"`

	// Redacted is set (not an API field) when the middleware returned the whole
	// attributes value as the string "********" instead of an object. (#57)
	Redacted bool `json:"-"`
}

// UnmarshalJSON tolerates the redacted form of "attributes": a session without
// the KEYCHAIN_CREDENTIAL write role gets the whole attributes object as the
// JSON string "********" instead of an object. Detect that (a string where an
// object is expected) and flag it, rather than failing with an opaque
// "cannot unmarshal string into ... attributes"; callers turn Redacted into an
// actionable diagnostic. (#57)
func (a *keychainSSHKeyPairAttributesAPI) UnmarshalJSON(b []byte) error {
	if t := bytes.TrimSpace(b); len(t) > 0 && t[0] == '"' {
		a.Redacted = true
		return nil
	}
	type alias keychainSSHKeyPairAttributesAPI
	var x alias
	if err := json.Unmarshal(b, &x); err != nil {
		return err
	}
	*a = keychainSSHKeyPairAttributesAPI(x)
	return nil
}

// redactedAttributesError is the diagnostic returned when a read sees redacted
// attributes: the session cannot reveal credential attributes. (#57)
func redactedAttributesError() diag.Diagnostics {
	var d diag.Diagnostics
	d.AddError(
		"Keychain credential attributes are redacted",
		"TrueNAS returned this credential's \"attributes\" as \"********\": the session used for the "+
			"read lacks the KEYCHAIN_CREDENTIAL write role, which the middleware requires to reveal "+
			"credential attributes. Re-run with a full-admin session, or an API key whose user holds "+
			"KEYCHAIN_CREDENTIAL_WRITE — KEYCHAIN_CREDENTIAL_READ is not sufficient, only the write role unredacts.",
	)
	return d
}

// keychainSSHKeyPairAPI mirrors the JSON object returned by
// keychaincredential.create/update/get_instance/query for a SSH_KEY_PAIR
// entry.
type keychainSSHKeyPairAPI struct {
	ID         int64                           `json:"id"`
	Name       string                          `json:"name"`
	Type       string                          `json:"type"`
	Attributes keychainSSHKeyPairAttributesAPI `json:"attributes"`
}

// generatedKeyPair mirrors keychaincredential.generate_ssh_key_pair's
// result (probed live: {"private_key": "<PEM>", "public_key": "<OpenSSH
// line>"}, no other fields).
type generatedKeyPair struct {
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
}

// createPayload builds the map expected by keychaincredential.create for
// type=SSH_KEY_PAIR. privateKey/publicKey come from whichever path Create
// resolved (user-supplied m.PrivateKey, or a prior
// keychaincredential.generate_ssh_key_pair call's result) — see resource.go.
// publicKey is passed through only when non-empty: when the caller supplied
// their own private_key with no matching public_key value at hand, omitting
// it lets TrueNAS derive it server-side (probed live, see
// keychainSSHKeyPairAttributesAPI's doc comment).
func createPayload(name, privateKey, publicKey string) map[string]any {
	attrs := map[string]any{"private_key": privateKey}
	if publicKey != "" {
		attrs["public_key"] = publicKey
	}
	return map[string]any{
		"name":       name,
		"type":       keychainCredentialType,
		"attributes": attrs,
	}
}

// updatePayload builds the second arg of keychaincredential.update. Only
// "name" is ever sent — "private_key"/"public_key" both carry
// RequiresReplace in the schema, so Update is never called across a change
// to either, and probed live: a name-only update (omitting "attributes"
// entirely) leaves the existing key pair attributes untouched.
func (m *KeychainSSHKeyPairModel) updatePayload() map[string]any {
	return map[string]any{"name": m.Name.ValueString()}
}

// responseToModel maps a keychainSSHKeyPairAPI response onto a
// KeychainSSHKeyPairModel. Generate is intentionally left untouched (see
// KeychainSSHKeyPairModel's doc comment).
func responseToModel(api *keychainSSHKeyPairAPI, m *KeychainSSHKeyPairModel) diag.Diagnostics {
	var diags diag.Diagnostics

	if api.Attributes.Redacted {
		return redactedAttributesError()
	}

	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	// The server appends a final "\n" to a key supplied without one
	// (trimspace(file(...))); Trimmed's semantic equality keeps the configured
	// spelling. (B-054)
	m.PrivateKey = normalize.NewTrimmedPointerValue(api.Attributes.PrivateKey)
	m.PublicKey = types.StringPointerValue(api.Attributes.PublicKey)

	return diags
}

// responseToDataSourceModel maps a keychainSSHKeyPairAPI response onto a
// KeychainSSHKeyPairDataSourceModel.
func responseToDataSourceModel(api *keychainSSHKeyPairAPI, m *KeychainSSHKeyPairDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	if api.Attributes.Redacted {
		return redactedAttributesError()
	}

	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.PrivateKey = types.StringPointerValue(api.Attributes.PrivateKey)
	m.PublicKey = types.StringPointerValue(api.Attributes.PublicKey)

	return diags
}

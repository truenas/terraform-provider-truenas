// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_keytab

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// KerberosKeytabModel is the Terraform state/plan model for
// truenas_kerberos_keytab.
type KerberosKeytabModel struct {
	ID   types.Int64  `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	File types.String `tfsdk:"file"`
	// Write-only alternative to File: the base64 keytab read from config and
	// never stored in state, and not read back on refresh. Version triggers
	// re-send. Exactly one of file / file_wo is set. (secrets-in-state)
	FileWO        types.String `tfsdk:"file_wo"`
	FileWOVersion types.Int64  `tfsdk:"file_wo_version"`
}

// usesWriteOnlyFile reports whether the write-only keytab path is in use.
func (m *KerberosKeytabModel) usesWriteOnlyFile() bool {
	return !m.FileWOVersion.IsNull() && !m.FileWOVersion.IsUnknown()
}

// KerberosKeytabDataSourceModel is the read-only model for the
// truenas_kerberos_keytab datasource, looked up by "name".
type KerberosKeytabDataSourceModel struct {
	ID   types.Int64  `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	File types.String `tfsdk:"file"`
}

// kerberosKeytabAPI mirrors the JSON object returned by
// kerberos.keytab.create, kerberos.keytab.update, kerberos.keytab.get_instance,
// and kerberos.keytab.query.
//
// Probed against a live TrueNAS 25.10 box (`core.get_methods` for
// kerberos.keytab.create/update/get_instance/query/delete — all four report
// "job": false — plus a full create -> get_instance -> query -> update
// (name-only) -> delete round trip using a real keytab exported from a
// Samba AD DC via `samba-tool domain exportkeytab`): "file" is returned
// byte-for-byte identical to what was sent on every read path (create
// response, get_instance, query, and even the response to a name-only
// update that never re-sent file) — it is NOT redacted or omitted. That is
// the deciding evidence for modeling "file" as a normal Sensitive
// attribute rather than WriteOnly (see schema.go). The API's own schema
// declares "file" as anyOf(string minLength>=1, null), but null was never
// observed in practice; File is a plain string here, so an actual
// server-side null would decode to "" rather than erroring.
type kerberosKeytabAPI struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	File string `json:"file"`
}

// redactedFile is the value kerberos.keytab.get_instance/query return for
// "file" to a session without full-admin rights (a READONLY_ADMIN API key).
const redactedFile = "********"

// responseToModel maps a kerberosKeytabAPI response onto a
// KerberosKeytabModel. The keytab is a secret, so "file" is read back only
// when the prior plan/state already holds it (the resource manages it via
// file):
//   - file_wo in use: always null (the write-only path keeps it out of state).
//   - prior file null: left null. This is the import case. Import cannot see
//     the configuration, so it cannot tell a file-managed keytab from a
//     file_wo-managed one; reading the keytab here put the secret into state
//     for file_wo users (B-172). A file-managed keytab instead plans a
//     one-time in-place update after import that records (re-sends) file.
//   - the server returned the redaction mask (a READONLY_ADMIN key): the
//     prior value is kept, so a read-only refresh does not plan writing the
//     mask back (B-028).
func responseToModel(api *kerberosKeytabAPI, m *KerberosKeytabModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	switch {
	case m.usesWriteOnlyFile(), m.File.IsNull(), m.File.IsUnknown():
		m.File = types.StringNull()
	case api.File != redactedFile:
		m.File = types.StringValue(api.File)
	}
	return diags
}

// responseToDataSourceModel maps a kerberosKeytabAPI response onto a
// KerberosKeytabDataSourceModel.
func responseToDataSourceModel(api *kerberosKeytabAPI, m *KerberosKeytabDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	// A READONLY_ADMIN session gets the redaction mask, not the keytab: report
	// null rather than the mask as if it were data (B-028).
	if api.File == redactedFile {
		m.File = types.StringNull()
	} else {
		m.File = types.StringValue(api.File)
	}
	return diags
}

// apiPayload builds the map expected by kerberos.keytab.create and
// kerberos.keytab.update. Both "name" and "file" are Required in this
// resource's schema (see schema.go), so both are always known and always
// included — there is no optional-field omission logic needed here, unlike
// resources with Optional+Computed fields.
//
// "file" is passed through exactly as configured: the value is already the
// base64 text TrueNAS expects, so no additional encoding/decoding happens
// here. Double-encoding (e.g. accidentally base64-encoding an
// already-base64 string) would corrupt the keytab and is exercised against
// in model_test.go.
func (m *KerberosKeytabModel) apiPayload() map[string]any {
	p := map[string]any{"name": m.Name.ValueString()}
	// file comes from File here; the write-only file_wo (not in the model) is
	// injected from config by the resource when used.
	if !m.File.IsNull() && !m.File.IsUnknown() {
		p["file"] = m.File.ValueString()
	}
	return p
}

// injectWriteOnlyFile sets the payload "file" from the write-only file_wo read
// from config, when present.
func injectWriteOnlyFile(payload map[string]any, cfg *KerberosKeytabModel) {
	if !cfg.FileWO.IsNull() && !cfg.FileWO.IsUnknown() && cfg.FileWO.ValueString() != "" {
		payload["file"] = cfg.FileWO.ValueString()
	}
}

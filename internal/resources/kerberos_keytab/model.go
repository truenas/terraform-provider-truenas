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

// responseToModel maps a kerberosKeytabAPI response onto a
// KerberosKeytabModel.
func responseToModel(api *kerberosKeytabAPI, m *KerberosKeytabModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.File = types.StringValue(api.File)
	return diags
}

// responseToDataSourceModel maps a kerberosKeytabAPI response onto a
// KerberosKeytabDataSourceModel.
func responseToDataSourceModel(api *kerberosKeytabAPI, m *KerberosKeytabDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.File = types.StringValue(api.File)
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
	return map[string]any{
		"name": m.Name.ValueString(),
		"file": m.File.ValueString(),
	}
}

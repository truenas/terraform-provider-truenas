// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_keytab

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestApiPayload_PassesFieldsThrough verifies apiPayload includes exactly
// "name" and "file" with the exact keys observed in the
// kerberos.keytab.create/update probe, and no others.
func TestApiPayload_PassesFieldsThrough(t *testing.T) {
	m := &KerberosKeytabModel{
		Name: types.StringValue("tf-test-keytab"),
		File: types.StringValue("QkFTRTY0REFUQQ=="),
	}

	p := m.apiPayload()

	if len(p) != 2 {
		t.Fatalf("payload has %d keys (%v), want 2 (name, file)", len(p), p)
	}
	if p["name"] != "tf-test-keytab" {
		t.Errorf("payload[name] = %v, want \"tf-test-keytab\"", p["name"])
	}
	if p["file"] != "QkFTRTY0REFUQQ==" {
		t.Errorf("payload[file] = %v, want \"QkFTRTY0REFUQQ==\"", p["file"])
	}
}

// TestApiPayload_NoDoubleEncode verifies that apiPayload passes the
// configured "file" value through byte-for-byte — it must never re-encode
// (e.g. base64-encode an already-base64 string) or otherwise transform it,
// since the value is already the exact base64 text TrueNAS expects.
func TestApiPayload_NoDoubleEncode(t *testing.T) {
	// Synthetic, structurally-plausible base64 payload (starts with the
	// standard keytab magic bytes 0x05 0x02, followed by arbitrary filler
	// — NOT derived from any real keytab), containing '/' and '+' —
	// characters that would visibly mutate under an accidental re-encode.
	rawB64 := "BQL//77vAAA+Pj77/78BAgMEBQYHCAkKCwyqu8zd7v8RIjNEVWZ3iJkA/v38+/r5+Pc="

	m := &KerberosKeytabModel{
		Name: types.StringValue("tf-test-keytab"),
		File: types.StringValue(rawB64),
	}

	p := m.apiPayload()

	got, ok := p["file"].(string)
	if !ok {
		t.Fatalf("payload[file] is %T, want string", p["file"])
	}
	if got != rawB64 {
		t.Errorf("payload[file] was transformed:\n got  %q\n want %q", got, rawB64)
	}
	if len(got) != len(rawB64) {
		t.Errorf("payload[file] length = %d, want %d (double-encoding would change length)", len(got), len(rawB64))
	}
}

// TestResponseToModel verifies responseToModel against the exact shape
// observed from a live kerberos.keytab.create/get_instance/query call: id,
// name, and file all present, file byte-for-byte identical to what was
// sent (see model.go's kerberosKeytabAPI doc comment for the full probe
// writeup).
func TestResponseToModel(t *testing.T) {
	api := &kerberosKeytabAPI{
		ID:   1,
		Name: "tf-test-keytab",
		File: "QkFTRTY0REFUQQ==",
	}

	m := &KerberosKeytabModel{}
	diags := responseToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueInt64() != 1 {
		t.Errorf("ID = %v, want 1", m.ID)
	}
	if m.Name.ValueString() != "tf-test-keytab" {
		t.Errorf("Name = %q, want \"tf-test-keytab\"", m.Name.ValueString())
	}
	if m.File.ValueString() != "QkFTRTY0REFUQQ==" {
		t.Errorf("File = %q, want \"QkFTRTY0REFUQQ==\" (file should round-trip intact, not be redacted)", m.File.ValueString())
	}
}

// TestResponseToDataSourceModel mirrors TestResponseToModel for the
// datasource model.
func TestResponseToDataSourceModel(t *testing.T) {
	api := &kerberosKeytabAPI{
		ID:   2,
		Name: "tf-test-keytab-ds",
		File: "QkFTRTY0REFUQQ==",
	}

	m := &KerberosKeytabDataSourceModel{}
	diags := responseToDataSourceModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.Name.ValueString() != "tf-test-keytab-ds" {
		t.Errorf("Name = %q, want \"tf-test-keytab-ds\"", m.Name.ValueString())
	}
	if m.File.ValueString() != "QkFTRTY0REFUQQ==" {
		t.Errorf("File = %q, want \"QkFTRTY0REFUQQ==\"", m.File.ValueString())
	}
}

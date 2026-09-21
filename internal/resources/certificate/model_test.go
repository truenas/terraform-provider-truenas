// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package certificate

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func strPtr(s string) *string { return &s }
func i64Ptr(i int64) *int64   { return &i }
func boolPtr(b bool) *bool    { return &b }

// TestCreatePayload_Imported verifies createPayload includes name,
// create_type, certificate, and privatekey for the IMPORTED path, and omits
// unset optionals.
func TestCreatePayload_Imported(t *testing.T) {
	ctx := context.Background()
	m := &CertificateModel{
		Name:               types.StringValue("tf-cert"),
		CreateType:         types.StringValue(CreateTypeImported),
		Certificate:        types.StringValue("CERT PEM"),
		Privatekey:         types.StringValue("KEY PEM"),
		CSR:                types.StringNull(),
		KeyType:            types.StringNull(),
		KeyLength:          types.Int64Null(),
		ECCurve:            types.StringNull(),
		Passphrase:         types.StringNull(),
		City:               types.StringNull(),
		Common:             types.StringNull(),
		Country:            types.StringNull(),
		Email:              types.StringNull(),
		Organization:       types.StringNull(),
		OrganizationalUnit: types.StringNull(),
		State:              types.StringNull(),
		DigestAlgorithm:    types.StringNull(),
		San:                types.ListNull(types.StringType),
		AddToTrustedStore:  types.BoolNull(),
		RenewDays:          types.Int64Null(),
		AcmeDirectoryURI:   types.StringNull(),
		CsrID:              types.Int64Null(),
		Tos:                types.BoolNull(),
		DNSMapping:         types.MapNull(types.Int64Type),
	}

	p, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload returned diagnostics errors: %v", diags)
	}

	if p["name"] != "tf-cert" {
		t.Errorf("payload[name] = %v, want tf-cert", p["name"])
	}
	if p["create_type"] != CreateTypeImported {
		t.Errorf("payload[create_type] = %v, want %v", p["create_type"], CreateTypeImported)
	}
	if p["certificate"] != "CERT PEM" {
		t.Errorf("payload[certificate] = %v, want CERT PEM", p["certificate"])
	}
	if p["privatekey"] != "KEY PEM" {
		t.Errorf("payload[privatekey] = %v, want KEY PEM", p["privatekey"])
	}
	for _, unsetKey := range []string{"CSR", "key_type", "key_length", "ec_curve", "passphrase", "san", "acme_directory_uri", "csr_id", "tos", "dns_mapping"} {
		if _, present := p[unsetKey]; present {
			t.Errorf("payload[%s] should be omitted when unset, got %v", unsetKey, p[unsetKey])
		}
	}
}

// TestUpdatePayload_OnlyUpdatableFields verifies updatePayload includes
// name/renew_days/add_to_trusted_store — the top-level set
// certificate.update's schema accepts (probed live) — but ALSO gates
// "add_to_trusted_store" and "renew_days" by create_type, since
// certificate.update's own service-level validation rejects both outside
// specific create_types (probed live; see updatePayload's doc comment):
// "add_to_trusted_store" only for CERTIFICATE_CREATE_IMPORTED/_ACME (never
// for an unsigned CSR entry), "renew_days" only for CERTIFICATE_CREATE_ACME.
func TestUpdatePayload_OnlyUpdatableFields(t *testing.T) {
	m := &CertificateModel{
		CreateType:        types.StringValue(CreateTypeACME),
		Name:              types.StringValue("tf-cert-renamed"),
		AddToTrustedStore: types.BoolValue(true),
		RenewDays:         types.Int64Value(15),
	}

	p := m.updatePayload()

	if len(p) != 3 {
		t.Fatalf("updatePayload has %d keys (%v), want 3 (name, add_to_trusted_store, renew_days)", len(p), p)
	}
	if p["name"] != "tf-cert-renamed" {
		t.Errorf("payload[name] = %v, want tf-cert-renamed", p["name"])
	}
	if p["add_to_trusted_store"] != true {
		t.Errorf("payload[add_to_trusted_store] = %v, want true", p["add_to_trusted_store"])
	}
	if p["renew_days"] != int64(15) {
		t.Errorf("payload[renew_days] = %v, want 15", p["renew_days"])
	}
}

// TestUpdatePayload_ImportedOmitsRenewDays verifies that for
// CERTIFICATE_CREATE_IMPORTED, updatePayload includes "add_to_trusted_store"
// but omits "renew_days" entirely — sending it would fail live with
// "[EINVAL] certificate_update.renew_days: Certificate renewal days is only
// supported for ACME certificates", even though renew_days is Optional+
// Computed and therefore always known once a prior apply has populated it.
func TestUpdatePayload_ImportedOmitsRenewDays(t *testing.T) {
	m := &CertificateModel{
		CreateType:        types.StringValue(CreateTypeImported),
		Name:              types.StringValue("tf-cert-renamed"),
		AddToTrustedStore: types.BoolValue(true),
		RenewDays:         types.Int64Value(10), // carried forward by UseStateForUnknown
	}

	p := m.updatePayload()

	if _, present := p["renew_days"]; present {
		t.Errorf("updatePayload for %s should omit renew_days, got %v", CreateTypeImported, p)
	}
	if p["add_to_trusted_store"] != true {
		t.Errorf("updatePayload for %s should include add_to_trusted_store, got %v", CreateTypeImported, p)
	}
}

// TestUpdatePayload_CSROmitsAddToTrustedStoreAndRenewDays verifies that for
// CERTIFICATE_CREATE_CSR (an unsigned CSR entry), updatePayload omits both
// "add_to_trusted_store" (rejected live: "A CSR cannot be added to the
// system's trusted store") and "renew_days" — only "name" survives.
func TestUpdatePayload_CSROmitsAddToTrustedStoreAndRenewDays(t *testing.T) {
	m := &CertificateModel{
		CreateType:        types.StringValue(CreateTypeCSR),
		Name:              types.StringValue("tf-cert-renamed"),
		AddToTrustedStore: types.BoolValue(false),
		RenewDays:         types.Int64Value(10),
	}

	p := m.updatePayload()

	if len(p) != 1 {
		t.Fatalf("updatePayload for %s has %d keys (%v), want 1 (name only)", CreateTypeCSR, len(p), p)
	}
	if p["name"] != "tf-cert-renamed" {
		t.Errorf("payload[name] = %v, want tf-cert-renamed", p["name"])
	}
}

// TestResponseToModel_ProbedShape verifies responseToModel against the
// exact shape observed from a live certificate.create (IMPORTED)/
// get_instance call on TrueNAS 25.10, including the masked-then-
// re-read privatekey and pointer-nullable parsed fields.
func TestResponseToModel_ProbedShape(t *testing.T) {
	ctx := context.Background()
	api := &certificateAPI{
		ID:                 3,
		Name:               "tf_probe_imported_cert",
		Certificate:        strPtr("-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n"),
		Privatekey:         strPtr("-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----\n"),
		CSR:                nil,
		RenewDays:          i64Ptr(10),
		AddToTrustedStore:  false,
		RootPath:           "/etc/certificates",
		CertificatePath:    strPtr("/etc/certificates/tf_probe_imported_cert.crt"),
		PrivatekeyPath:     strPtr("/etc/certificates/tf_probe_imported_cert.key"),
		CsrPath:            nil,
		CertType:           "CERTIFICATE",
		KeyLength:          i64Ptr(2048),
		KeyType:            strPtr("RSA"),
		Country:            nil,
		State:              nil,
		City:               nil,
		Organization:       nil,
		OrganizationalUnit: nil,
		Common:             strPtr("tf-probe-import.example.com"),
		San:                []string{"DNS:tf-probe-import.example.com"},
		Email:              nil,
		DigestAlgorithm:    strPtr("SHA256"),
		Fingerprint:        strPtr("A5:C0:37:98:F0:B8:6B:34:B2:50:51:D1:3F:2D:4F:01:E2:2C:2D:02"),
		Expired:            boolPtr(false),
	}

	m := &CertificateModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueInt64() != 3 {
		t.Errorf("ID = %v, want 3", m.ID)
	}
	if m.Name.ValueString() != "tf_probe_imported_cert" {
		t.Errorf("Name = %q, want tf_probe_imported_cert", m.Name.ValueString())
	}
	if m.Privatekey.IsNull() || m.Privatekey.ValueString() == "" {
		t.Error("Privatekey should be populated intact from get_instance, not null/empty")
	}
	if m.CSR.IsNull() != true {
		t.Error("CSR should be null for an IMPORTED certificate")
	}
	if m.KeyLength.ValueInt64() != 2048 {
		t.Errorf("KeyLength = %v, want 2048", m.KeyLength)
	}
	if m.Fingerprint.ValueString() == "" {
		t.Error("Fingerprint should be populated")
	}
	if m.Expired.ValueBool() {
		t.Error("Expired = true, want false")
	}

	var san []string
	diags = m.San.ElementsAs(ctx, &san, false)
	if diags.HasError() {
		t.Fatalf("reading back san: %v", diags)
	}
	if len(san) != 1 || san[0] != "tf-probe-import.example.com" {
		t.Errorf("San = %v, want [tf-probe-import.example.com] (DNS: prefix stripped)", san)
	}
}

// TestResponseToModel_CSRTypeNullFields verifies that CSR-type responses
// (certificate=nil, many parsed fields nil per live probe) map to null
// rather than panicking or zero-valuing.
func TestResponseToModel_CSRTypeNullFields(t *testing.T) {
	ctx := context.Background()
	api := &certificateAPI{
		ID:              4,
		Name:            "tf_probe_csr_cert",
		Certificate:     nil,
		Privatekey:      strPtr("-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----\n"),
		CSR:             strPtr("-----BEGIN CERTIFICATE REQUEST-----\n...\n-----END CERTIFICATE REQUEST-----\n"),
		RenewDays:       i64Ptr(10),
		RootPath:        "/etc/certificates",
		CertificatePath: nil,
		PrivatekeyPath:  strPtr("/etc/certificates/tf_probe_csr_cert.key"),
		CsrPath:         strPtr("/etc/certificates/tf_probe_csr_cert.csr"),
		CertType:        "CERTIFICATE",
		KeyLength:       i64Ptr(2048),
		KeyType:         strPtr("RSA"),
		Common:          strPtr("tf-probe-csr.example.com"),
		San:             []string{"DNS:tf-probe-csr.example.com"},
		DigestAlgorithm: nil,
		Fingerprint:     nil,
		Expired:         nil,
	}

	m := &CertificateModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if !m.Certificate.IsNull() {
		t.Error("Certificate should be null for a CSR-type entry")
	}
	if m.CSR.IsNull() || m.CSR.ValueString() == "" {
		t.Error("CSR should be populated (non-null, non-empty) for a CSR-type entry")
	}
	if !m.Fingerprint.IsNull() {
		t.Error("Fingerprint should be null for an unsigned CSR")
	}
	if !m.Expired.IsNull() {
		t.Error("Expired should be null for an unsigned CSR")
	}
}

// TestPreflight_ImportedRequiresCertAndKey verifies IMPORTED requires both
// "certificate" and "privatekey" (probed live: a bare
// CERTIFICATE_CREATE_IMPORTED payload with neither field set fails schema
// validation for both).
func TestPreflight_ImportedRequiresCertAndKey(t *testing.T) {
	m := &CertificateModel{
		CreateType:  types.StringValue(CreateTypeImported),
		Certificate: types.StringNull(),
		Privatekey:  types.StringNull(),
	}
	diags := m.preflight()
	if !diags.HasError() {
		t.Fatal("expected preflight errors for IMPORTED with no certificate/privatekey")
	}
	if len(diags.Errors()) != 2 {
		t.Errorf("got %d preflight errors, want 2 (certificate, privatekey): %v", len(diags.Errors()), diags)
	}
}

// TestPreflight_ImportedCSRRequiresCSRAndKey mirrors
// TestPreflight_ImportedRequiresCertAndKey for IMPORTED_CSR.
func TestPreflight_ImportedCSRRequiresCSRAndKey(t *testing.T) {
	m := &CertificateModel{
		CreateType: types.StringValue(CreateTypeImportedCSR),
		CSR:        types.StringNull(),
		Privatekey: types.StringNull(),
	}
	diags := m.preflight()
	if !diags.HasError() {
		t.Fatal("expected preflight errors for IMPORTED_CSR with no csr/privatekey")
	}
	if len(diags.Errors()) != 2 {
		t.Errorf("got %d preflight errors, want 2 (csr, privatekey): %v", len(diags.Errors()), diags)
	}
}

// TestPreflight_CSRRequiresSan verifies CSR requires a non-empty "san" —
// probed live: omitting it fails "List should have at least 1 item", even
// with "common" set.
func TestPreflight_CSRRequiresSan(t *testing.T) {
	ctx := context.Background()
	emptySan, diags := types.ListValueFrom(ctx, types.StringType, []string{})
	if diags.HasError() {
		t.Fatalf("building empty san list: %v", diags)
	}

	m := &CertificateModel{
		CreateType: types.StringValue(CreateTypeCSR),
		Common:     types.StringValue("tf-cert.example.com"),
		San:        emptySan,
		KeyType:    types.StringValue("EC"), // avoid tripping the key_length check too
	}
	diags = m.preflight()
	if !diags.HasError() {
		t.Fatal("expected a preflight error for CSR with empty san")
	}
}

// TestPreflight_CSRRequiresKeyLengthForRSA verifies CSR requires
// "key_length" when key_type is RSA (the default) — probed live: TrueNAS
// rejects an RSA CSR with no key_length ("RSA-based keys require an entry
// in this field"). EC does NOT require an explicit ec_curve (schema
// default SECP384R1 is accepted, probed live).
func TestPreflight_CSRRequiresKeyLengthForRSA(t *testing.T) {
	ctx := context.Background()
	san, diags := types.ListValueFrom(ctx, types.StringType, []string{"tf-cert.example.com"})
	if diags.HasError() {
		t.Fatalf("building san list: %v", diags)
	}

	rsaNoLength := &CertificateModel{
		CreateType: types.StringValue(CreateTypeCSR),
		San:        san,
		KeyType:    types.StringNull(), // defaults to RSA
		KeyLength:  types.Int64Null(),
	}
	if diags := rsaNoLength.preflight(); !diags.HasError() {
		t.Error("expected a preflight error for RSA CSR with no key_length")
	}

	ecNoCurve := &CertificateModel{
		CreateType: types.StringValue(CreateTypeCSR),
		San:        san,
		KeyType:    types.StringValue("EC"),
		KeyLength:  types.Int64Null(),
	}
	if diags := ecNoCurve.preflight(); diags.HasError() {
		t.Errorf("did not expect a preflight error for EC CSR with no key_length: %v", diags)
	}
}

// TestPreflight_AcmeRequiresAllFour verifies ACME requires
// acme_directory_uri, csr_id, tos (true), and dns_mapping (non-empty), per
// the task brief.
func TestPreflight_AcmeRequiresAllFour(t *testing.T) {
	m := &CertificateModel{
		CreateType:       types.StringValue(CreateTypeACME),
		AcmeDirectoryURI: types.StringNull(),
		CsrID:            types.Int64Null(),
		Tos:              types.BoolNull(),
		DNSMapping:       types.MapNull(types.Int64Type),
	}
	diags := m.preflight()
	if len(diags.Errors()) != 4 {
		t.Errorf("got %d preflight errors, want 4 (acme_directory_uri, csr_id, tos, dns_mapping): %v", len(diags.Errors()), diags)
	}
}

// TestPreflight_AcmeSatisfied verifies a fully-populated ACME model passes
// preflight cleanly.
func TestPreflight_AcmeSatisfied(t *testing.T) {
	ctx := context.Background()
	mapping, diags := types.MapValueFrom(ctx, types.Int64Type, map[string]int64{"example.com": 1})
	if diags.HasError() {
		t.Fatalf("building dns_mapping: %v", diags)
	}

	m := &CertificateModel{
		CreateType:       types.StringValue(CreateTypeACME),
		AcmeDirectoryURI: types.StringValue("https://acme-staging-v02.api.letsencrypt.org/directory"),
		CsrID:            types.Int64Value(1),
		Tos:              types.BoolValue(true),
		DNSMapping:       mapping,
	}
	if diags := m.preflight(); diags.HasError() {
		t.Errorf("did not expect preflight errors: %v", diags)
	}
}

// TestValidateRenewDays_RejectedForNonAcmeConfig verifies renew_days is
// rejected client-side for any non-ACME create_type when explicitly set in
// Config — probed live: certificate.update rejects it outright ("Certificate
// renewal days is only supported for ACME certificates") and
// certificate.create silently ignores/overrides it, either of which would
// otherwise surface as a confusing provider-inconsistency error rather than
// a clear preflight one.
func TestValidateRenewDays_RejectedForNonAcmeConfig(t *testing.T) {
	diags := validateRenewDays(CreateTypeCSR, types.Int64Value(5))
	if !diags.HasError() {
		t.Fatal("expected an error for renew_days explicitly set in config on a non-ACME create_type")
	}
}

// TestValidateRenewDays_AllowedWhenUnsetInConfig verifies a non-ACME
// create_type with renew_days left null in Config (the caller never wrote
// it in HCL) passes cleanly, even though the same field could be a known,
// non-null value in Plan once a prior apply has populated it.
func TestValidateRenewDays_AllowedWhenUnsetInConfig(t *testing.T) {
	diags := validateRenewDays(CreateTypeCSR, types.Int64Null())
	if diags.HasError() {
		t.Errorf("did not expect an error for renew_days left unset in config: %v", diags)
	}
}

// TestValidateRenewDays_AllowedForAcme verifies ACME may set renew_days
// freely.
func TestValidateRenewDays_AllowedForAcme(t *testing.T) {
	diags := validateRenewDays(CreateTypeACME, types.Int64Value(5))
	if diags.HasError() {
		t.Errorf("did not expect an error for renew_days set on an ACME create_type: %v", diags)
	}
}

// TestSanListValue_StripsDNSPrefix verifies sanListValue strips the "DNS:"
// prefix the API adds to every "san" entry it returns — probed live:
// certificate.create accepts a plain hostname but certificate.query/
// get_instance echo it back prefixed.
func TestSanListValue_StripsDNSPrefix(t *testing.T) {
	ctx := context.Background()
	got, diags := sanListValue(ctx, []string{"DNS:tf-acc-csr.example.com", "DNS:tf-acc-csr-2.example.com"})
	if diags.HasError() {
		t.Fatalf("sanListValue returned diagnostics errors: %v", diags)
	}

	var san []string
	diags = got.ElementsAs(ctx, &san, false)
	if diags.HasError() {
		t.Fatalf("reading back san: %v", diags)
	}
	want := []string{"tf-acc-csr.example.com", "tf-acc-csr-2.example.com"}
	if len(san) != len(want) {
		t.Fatalf("San = %v, want %v", san, want)
	}
	for i := range want {
		if san[i] != want[i] {
			t.Errorf("San[%d] = %q, want %q", i, san[i], want[i])
		}
	}
}

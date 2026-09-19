// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package certificate

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// CertificateModel is the Terraform state/plan model for truenas_certificate.
//
// certificate.create's "accepts" schema (probed live and identical, field
// for field, on both TrueNAS 25.10 and 26.0 — no version gating
// needed) is a single flat object covering all four create_type variants
// (CERTIFICATE_CREATE_IMPORTED / _CSR / _IMPORTED_CSR / _ACME); the service
// validates only the subset relevant to the chosen create_type. This model
// mirrors that flat shape rather than splitting into per-type nested
// blocks, matching how the upstream API itself is one method with one
// payload shape. "cert_extensions" (a three-way nested X.509 extension
// object: BasicConstraints/ExtendedKeyUsage/KeyUsage) is intentionally
// omitted from this model to keep the schema a manageable size — it is
// optional on every create_type and not required for any of this task's
// Tier 1 acceptance paths (IMPORTED, CSR). Likewise a number of
// certificate.query's purely-derived/parsed-from-PEM fields (DN,
// subject_name_hash, extensions, lifetime, from, until, serial, chain,
// chain_list, digest_algorithm-as-output, cert_type_existing/CSR/CA, acme,
// acme_uri, domains_authenticators) are not surfaced; only a practical
// subset of computed metadata is (certificate_path, privatekey_path,
// csr_path, root_path, cert_type, fingerprint, expired).
type CertificateModel struct {
	ID                 types.Int64  `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	CreateType         types.String `tfsdk:"create_type"`
	AddToTrustedStore  types.Bool   `tfsdk:"add_to_trusted_store"`
	RenewDays          types.Int64  `tfsdk:"renew_days"`
	Certificate        types.String `tfsdk:"certificate"`
	Privatekey         types.String `tfsdk:"privatekey"`
	CSR                types.String `tfsdk:"csr"`
	KeyType            types.String `tfsdk:"key_type"`
	KeyLength          types.Int64  `tfsdk:"key_length"`
	ECCurve            types.String `tfsdk:"ec_curve"`
	Passphrase         types.String `tfsdk:"passphrase"`
	City               types.String `tfsdk:"city"`
	Common             types.String `tfsdk:"common"`
	Country            types.String `tfsdk:"country"`
	Email              types.String `tfsdk:"email"`
	Organization       types.String `tfsdk:"organization"`
	OrganizationalUnit types.String `tfsdk:"organizational_unit"`
	State              types.String `tfsdk:"state"`
	DigestAlgorithm    types.String `tfsdk:"digest_algorithm"`
	San                types.List   `tfsdk:"san"`
	AcmeDirectoryURI   types.String `tfsdk:"acme_directory_uri"`
	CsrID              types.Int64  `tfsdk:"csr_id"`
	Tos                types.Bool   `tfsdk:"tos"`
	DNSMapping         types.Map    `tfsdk:"dns_mapping"`
	CertificatePath    types.String `tfsdk:"certificate_path"`
	PrivatekeyPath     types.String `tfsdk:"privatekey_path"`
	CsrPath            types.String `tfsdk:"csr_path"`
	RootPath           types.String `tfsdk:"root_path"`
	CertType           types.String `tfsdk:"cert_type"`
	Fingerprint        types.String `tfsdk:"fingerprint"`
	Expired            types.Bool   `tfsdk:"expired"`
}

// CertificateDataSourceModel is the read-only lookup model for the
// truenas_certificate datasource, looked up by "name" (certificate.create
// requires name to be unique).
type CertificateDataSourceModel struct {
	ID                 types.Int64  `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	AddToTrustedStore  types.Bool   `tfsdk:"add_to_trusted_store"`
	RenewDays          types.Int64  `tfsdk:"renew_days"`
	Certificate        types.String `tfsdk:"certificate"`
	Privatekey         types.String `tfsdk:"privatekey"`
	CSR                types.String `tfsdk:"csr"`
	KeyType            types.String `tfsdk:"key_type"`
	KeyLength          types.Int64  `tfsdk:"key_length"`
	ECCurve            types.String `tfsdk:"ec_curve"`
	City               types.String `tfsdk:"city"`
	Common             types.String `tfsdk:"common"`
	Country            types.String `tfsdk:"country"`
	Email              types.String `tfsdk:"email"`
	Organization       types.String `tfsdk:"organization"`
	OrganizationalUnit types.String `tfsdk:"organizational_unit"`
	State              types.String `tfsdk:"state"`
	San                types.List   `tfsdk:"san"`
	CertificatePath    types.String `tfsdk:"certificate_path"`
	PrivatekeyPath     types.String `tfsdk:"privatekey_path"`
	CsrPath            types.String `tfsdk:"csr_path"`
	RootPath           types.String `tfsdk:"root_path"`
	CertType           types.String `tfsdk:"cert_type"`
	Fingerprint        types.String `tfsdk:"fingerprint"`
	Expired            types.Bool   `tfsdk:"expired"`
}

// certificateAPI mirrors the JSON object returned by certificate.create,
// certificate.update, certificate.get_instance, and certificate.query.
// Probed live against TrueNAS 25.10.
//
// Critical read-back finding: the job *result* returned directly by
// certificate.create/certificate.update masks "privatekey" as the literal
// string "********" (confirmed live: create response showed
// "privatekey": "********" while the certificate PEM in the same response
// was NOT masked). A subsequent certificate.get_instance call for the same
// id, however, returns the real PEM-encoded private key byte-for-byte
// intact. This means the resource's Create/Update handlers must NOT trust
// the job result for "privatekey" — they must always re-read via
// certificate.get_instance before populating state (same "re-read after
// write" shape dataset.go uses, but for a different reason: masking, not
// omission). "certificate" and "CSR" are never masked in the job result;
// only "privatekey" is. Because get_instance itself returns privatekey
// intact (matching the kerberos_keytab precedent: create -> get_instance ->
// query round trip showed the value returned byte-for-byte, never
// redacted), "privatekey" is modeled as a normal Sensitive attribute (kept
// out of plan/apply output and logs) rather than WriteOnly.
//
// "passphrase" is accepted on create but never appears anywhere in the
// "returns" schema of create/update/get_instance/query — the API has no way
// to echo it back. It is modeled as write-only-by-convention: Sensitive,
// Optional, deliberately NOT Computed, and responseToModel does not touch
// it (the plan/state value is preserved as-is, mirroring dataset.go's
// ShareType field).
type certificateAPI struct {
	ID                 int64    `json:"id"`
	Name               string   `json:"name"`
	Certificate        *string  `json:"certificate"`
	Privatekey         *string  `json:"privatekey"`
	CSR                *string  `json:"CSR"`
	RenewDays          *int64   `json:"renew_days"`
	AddToTrustedStore  bool     `json:"add_to_trusted_store"`
	RootPath           string   `json:"root_path"`
	CertificatePath    *string  `json:"certificate_path"`
	PrivatekeyPath     *string  `json:"privatekey_path"`
	CsrPath            *string  `json:"csr_path"`
	CertType           string   `json:"cert_type"`
	KeyLength          *int64   `json:"key_length"`
	KeyType            *string  `json:"key_type"`
	Country            *string  `json:"country"`
	State              *string  `json:"state"`
	City               *string  `json:"city"`
	Organization       *string  `json:"organization"`
	OrganizationalUnit *string  `json:"organizational_unit"`
	Common             *string  `json:"common"`
	San                []string `json:"san"`
	Email              *string  `json:"email"`
	DigestAlgorithm    *string  `json:"digest_algorithm"`
	Fingerprint        *string  `json:"fingerprint"`
	Expired            *bool    `json:"expired"`
}

// createTypePayload builds the map expected by certificate.create, applying
// only the fields relevant to the chosen create_type. Fields left
// null/unknown in the plan are omitted so the TrueNAS-side default takes
// effect.
func (m *CertificateModel) createPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	p := map[string]any{
		"name":        m.Name.ValueString(),
		"create_type": m.CreateType.ValueString(),
	}

	if !m.AddToTrustedStore.IsNull() && !m.AddToTrustedStore.IsUnknown() {
		p["add_to_trusted_store"] = m.AddToTrustedStore.ValueBool()
	}
	if !m.RenewDays.IsNull() && !m.RenewDays.IsUnknown() {
		p["renew_days"] = m.RenewDays.ValueInt64()
	}
	if !m.Certificate.IsNull() && !m.Certificate.IsUnknown() {
		p["certificate"] = m.Certificate.ValueString()
	}
	if !m.Privatekey.IsNull() && !m.Privatekey.IsUnknown() {
		p["privatekey"] = m.Privatekey.ValueString()
	}
	if !m.CSR.IsNull() && !m.CSR.IsUnknown() {
		p["CSR"] = m.CSR.ValueString()
	}
	if !m.KeyType.IsNull() && !m.KeyType.IsUnknown() {
		p["key_type"] = m.KeyType.ValueString()
	}
	if !m.KeyLength.IsNull() && !m.KeyLength.IsUnknown() {
		p["key_length"] = m.KeyLength.ValueInt64()
	}
	if !m.ECCurve.IsNull() && !m.ECCurve.IsUnknown() {
		p["ec_curve"] = m.ECCurve.ValueString()
	}
	if !m.Passphrase.IsNull() && !m.Passphrase.IsUnknown() {
		p["passphrase"] = m.Passphrase.ValueString()
	}
	if !m.City.IsNull() && !m.City.IsUnknown() {
		p["city"] = m.City.ValueString()
	}
	if !m.Common.IsNull() && !m.Common.IsUnknown() {
		p["common"] = m.Common.ValueString()
	}
	if !m.Country.IsNull() && !m.Country.IsUnknown() {
		p["country"] = m.Country.ValueString()
	}
	if !m.Email.IsNull() && !m.Email.IsUnknown() {
		p["email"] = m.Email.ValueString()
	}
	if !m.Organization.IsNull() && !m.Organization.IsUnknown() {
		p["organization"] = m.Organization.ValueString()
	}
	if !m.OrganizationalUnit.IsNull() && !m.OrganizationalUnit.IsUnknown() {
		p["organizational_unit"] = m.OrganizationalUnit.ValueString()
	}
	if !m.State.IsNull() && !m.State.IsUnknown() {
		p["state"] = m.State.ValueString()
	}
	if !m.DigestAlgorithm.IsNull() && !m.DigestAlgorithm.IsUnknown() {
		p["digest_algorithm"] = m.DigestAlgorithm.ValueString()
	}
	if !m.San.IsNull() && !m.San.IsUnknown() {
		var san []string
		diags.Append(m.San.ElementsAs(ctx, &san, false)...)
		p["san"] = san
	}
	if !m.AcmeDirectoryURI.IsNull() && !m.AcmeDirectoryURI.IsUnknown() {
		p["acme_directory_uri"] = m.AcmeDirectoryURI.ValueString()
	}
	if !m.CsrID.IsNull() && !m.CsrID.IsUnknown() {
		p["csr_id"] = m.CsrID.ValueInt64()
	}
	if !m.Tos.IsNull() && !m.Tos.IsUnknown() {
		p["tos"] = m.Tos.ValueBool()
	}
	if !m.DNSMapping.IsNull() && !m.DNSMapping.IsUnknown() {
		var mapping map[string]int64
		diags.Append(m.DNSMapping.ElementsAs(ctx, &mapping, false)...)
		p["dns_mapping"] = mapping
	}

	return p, diags
}

// updatePayload builds the second arg of certificate.update. Probed live:
// certificate.update's "accepts" schema only ever exposes "renew_days",
// "add_to_trusted_store", and "name" — attempting to pass any other field
// (e.g. "certificate") fails with "[EINVAL] certificate_update.certificate:
// Extra inputs are not permitted". Every other schema attribute therefore
// carries RequiresReplace.
//
// Both remaining fields are further restricted beyond that top-level schema
// check, confirmed live (a schema-valid payload can still be rejected by
// certificate.update's own service-level validation):
//
//   - "renew_days" is rejected outright for any non-ACME certificate:
//     "[EINVAL] certificate_update.renew_days: Certificate renewal days is
//     only supported for ACME certificates" — even though the schema itself
//     shows no such restriction and the field carries an ordinary
//     server-side default (10) on every create_type. So it is only ever
//     included here when create_type is CERTIFICATE_CREATE_ACME.
//   - "add_to_trusted_store" is rejected for any certificate whose entry is
//     still just a CSR (cert_type_CSR=true — i.e. CERTIFICATE_CREATE_CSR or
//     CERTIFICATE_CREATE_IMPORTED_CSR, neither of which is a signed
//     certificate yet): "[EINVAL] certificate_update.add_to_trusted_store:
//     A CSR cannot be added to the system's trusted store". Confirmed
//     accepted for CERTIFICATE_CREATE_IMPORTED (a live create -> update
//     add_to_trusted_store=true -> get_instance round trip showed
//     "add_to_trusted_store": true persisted). So it is only included here
//     for CERTIFICATE_CREATE_IMPORTED and CERTIFICATE_CREATE_ACME (the two
//     create_types that produce an actual signed certificate, immediately
//     or eventually).
func (m *CertificateModel) updatePayload() map[string]any {
	p := map[string]any{
		"name": m.Name.ValueString(),
	}

	createType := m.CreateType.ValueString()

	if createType == CreateTypeImported || createType == CreateTypeACME {
		if !m.AddToTrustedStore.IsNull() && !m.AddToTrustedStore.IsUnknown() {
			p["add_to_trusted_store"] = m.AddToTrustedStore.ValueBool()
		}
	}
	if createType == CreateTypeACME {
		if !m.RenewDays.IsNull() && !m.RenewDays.IsUnknown() {
			p["renew_days"] = m.RenewDays.ValueInt64()
		}
	}

	return p
}

// responseToModel maps a certificateAPI response onto a CertificateModel.
// "create_type", "passphrase", "acme_directory_uri", "csr_id", "tos", and
// "dns_mapping" are never returned by the API, so they are left untouched
// (preserving the plan/state value the caller already set).
func responseToModel(ctx context.Context, api *certificateAPI, m *CertificateModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.AddToTrustedStore = types.BoolValue(api.AddToTrustedStore)
	m.RenewDays = types.Int64PointerValue(api.RenewDays)
	m.Certificate = types.StringPointerValue(api.Certificate)
	m.Privatekey = types.StringPointerValue(api.Privatekey)
	m.CSR = types.StringPointerValue(api.CSR)
	m.KeyType = types.StringPointerValue(api.KeyType)
	m.KeyLength = types.Int64PointerValue(api.KeyLength)
	m.City = types.StringPointerValue(api.City)
	m.Common = types.StringPointerValue(api.Common)
	m.Country = types.StringPointerValue(api.Country)
	m.Email = types.StringPointerValue(api.Email)
	m.Organization = types.StringPointerValue(api.Organization)
	m.OrganizationalUnit = types.StringPointerValue(api.OrganizationalUnit)
	m.State = types.StringPointerValue(api.State)
	m.DigestAlgorithm = types.StringPointerValue(api.DigestAlgorithm)
	m.CertificatePath = types.StringPointerValue(api.CertificatePath)
	m.PrivatekeyPath = types.StringPointerValue(api.PrivatekeyPath)
	m.CsrPath = types.StringPointerValue(api.CsrPath)
	m.RootPath = types.StringValue(api.RootPath)
	m.CertType = types.StringValue(api.CertType)
	m.Fingerprint = types.StringPointerValue(api.Fingerprint)
	m.Expired = types.BoolPointerValue(api.Expired)

	san, d := sanListValue(ctx, api.San)
	diags.Append(d...)
	m.San = san

	// ec_curve is never echoed back by the API (key_length/key_type are, but
	// there is no "ec_curve" field in the returns schema); preserve the
	// plan/state value the caller already set.

	return diags
}

// sanListValue builds a types.List for "san" from the API's []string,
// stripping the "DNS:" RFC 822-style type prefix the API adds to every
// entry it returns (probed live: certificate.create accepts a plain
// hostname in "san", e.g. "tf-acc.example.com", but certificate.query/
// get_instance echo it back as "DNS:tf-acc.example.com" — without this
// normalization, a config that sets "san" would fail Terraform's plan-
// consistency check with "produced an inconsistent result after apply").
// A nil/empty slice is treated as an empty (non-null) list to match
// certificate.query's own "san" default shape for non-CSR types (an empty
// array, not null, when no SAN extension is present in an imported cert).
func sanListValue(ctx context.Context, san []string) (types.List, diag.Diagnostics) {
	out := make([]string, len(san))
	for i, s := range san {
		out[i] = strings.TrimPrefix(s, "DNS:")
	}
	return types.ListValueFrom(ctx, types.StringType, out)
}

// responseToDataSourceModel maps a certificateAPI response onto a
// CertificateDataSourceModel.
func responseToDataSourceModel(ctx context.Context, api *certificateAPI, m *CertificateDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.AddToTrustedStore = types.BoolValue(api.AddToTrustedStore)
	m.RenewDays = types.Int64PointerValue(api.RenewDays)
	m.Certificate = types.StringPointerValue(api.Certificate)
	m.Privatekey = types.StringPointerValue(api.Privatekey)
	m.CSR = types.StringPointerValue(api.CSR)
	m.KeyType = types.StringPointerValue(api.KeyType)
	m.KeyLength = types.Int64PointerValue(api.KeyLength)
	m.City = types.StringPointerValue(api.City)
	m.Common = types.StringPointerValue(api.Common)
	m.Country = types.StringPointerValue(api.Country)
	m.Email = types.StringPointerValue(api.Email)
	m.Organization = types.StringPointerValue(api.Organization)
	m.OrganizationalUnit = types.StringPointerValue(api.OrganizationalUnit)
	m.State = types.StringPointerValue(api.State)
	m.CertificatePath = types.StringPointerValue(api.CertificatePath)
	m.PrivatekeyPath = types.StringPointerValue(api.PrivatekeyPath)
	m.CsrPath = types.StringPointerValue(api.CsrPath)
	m.RootPath = types.StringValue(api.RootPath)
	m.CertType = types.StringValue(api.CertType)
	m.Fingerprint = types.StringPointerValue(api.Fingerprint)
	m.Expired = types.BoolPointerValue(api.Expired)

	san, d := sanListValue(ctx, api.San)
	diags.Append(d...)
	m.San = san

	return diags
}

// Known create_type values (certificate.create's "create_type" enum,
// probed live and identical on 25.10/26.0).
const (
	CreateTypeImported    = "CERTIFICATE_CREATE_IMPORTED"
	CreateTypeCSR         = "CERTIFICATE_CREATE_CSR"
	CreateTypeImportedCSR = "CERTIFICATE_CREATE_IMPORTED_CSR"
	CreateTypeACME        = "CERTIFICATE_CREATE_ACME"
)

// preflight validates the create_type-specific required fields client-side,
// ahead of the certificate.create call, so misconfigurations surface as a
// clear Terraform-level error instead of an opaque job failure. Requirements
// below were confirmed by live probing against TrueNAS 25.10:
//
//   - IMPORTED requires "certificate" and "privatekey" (both empty by
//     default on a bare CERTIFICATE_CREATE_IMPORTED payload).
//   - IMPORTED_CSR requires "csr" and "privatekey" (per the API's own
//     create_type documentation, confirmed by a live import-CSR probe).
//   - CSR requires "san" to have at least one entry — confirmed live: a CSR
//     create with no "san" fails "[EINVAL] ...san: List should have at
//     least 1 item after validation, not 0", regardless of whether "common"
//     is also set. It additionally requires "key_length" when "key_type" is
//     RSA (the schema default) — confirmed live: "[EINVAL]
//     certificate_create.key_length: RSA-based keys require an entry in
//     this field." EC key_type does NOT require an explicit "ec_curve";
//     the schema default (SECP384R1) is accepted as-is (confirmed live).
//   - ACME requires "acme_directory_uri", "csr_id", "tos", and
//     "dns_mapping" per the task brief. "renew_days" has a usable
//     API-side default (10, confirmed live) so it is not preflight-required
//     here, unlike the other four ACME fields.
//
// "renew_days" gets its own validateRenewDays check (see below) rather than
// living in this function, because it must be checked against the raw
// config value, not the plan: on Update, an Optional+Computed field the
// caller never set in HCL still plans as a concrete known value (the prior
// state's, carried forward by UseStateForUnknown — e.g. the API's own
// default of 10), which is indistinguishable from an explicit override once
// resolved into the plan.
func (m *CertificateModel) preflight() diag.Diagnostics {
	var diags diag.Diagnostics

	// isSet reports whether a string attribute was given a concrete,
	// non-empty value in the plan (as opposed to null/unknown/empty-string).
	// Unknown covers the common "attribute left unset by the caller" case
	// for Optional+Computed attributes during Create, which plan as Unknown
	// rather than Null.
	isSet := func(v types.String) bool {
		return !v.IsNull() && !v.IsUnknown() && v.ValueString() != ""
	}

	switch m.CreateType.ValueString() {
	case CreateTypeImported:
		if !isSet(m.Certificate) {
			diags.AddError("Missing required field for create_type "+CreateTypeImported, "\"certificate\" is required.")
		}
		if !isSet(m.Privatekey) {
			diags.AddError("Missing required field for create_type "+CreateTypeImported, "\"privatekey\" is required.")
		}
	case CreateTypeImportedCSR:
		if !isSet(m.CSR) {
			diags.AddError("Missing required field for create_type "+CreateTypeImportedCSR, "\"csr\" is required.")
		}
		if !isSet(m.Privatekey) {
			diags.AddError("Missing required field for create_type "+CreateTypeImportedCSR, "\"privatekey\" is required.")
		}
	case CreateTypeCSR:
		if m.San.IsNull() || m.San.IsUnknown() || len(m.San.Elements()) == 0 {
			diags.AddError("Missing required field for create_type "+CreateTypeCSR,
				"\"san\" must have at least one entry.")
		}
		keyType := "RSA"
		if isSet(m.KeyType) {
			keyType = m.KeyType.ValueString()
		}
		if keyType == "RSA" && (m.KeyLength.IsNull() || m.KeyLength.IsUnknown()) {
			diags.AddError("Missing required field for create_type "+CreateTypeCSR,
				"\"key_length\" is required when \"key_type\" is RSA (the default).")
		}
	case CreateTypeACME:
		if !isSet(m.AcmeDirectoryURI) {
			diags.AddError("Missing required field for create_type "+CreateTypeACME, "\"acme_directory_uri\" is required.")
		}
		if m.CsrID.IsNull() || m.CsrID.IsUnknown() {
			diags.AddError("Missing required field for create_type "+CreateTypeACME, "\"csr_id\" is required.")
		}
		if m.Tos.IsNull() || m.Tos.IsUnknown() || !m.Tos.ValueBool() {
			diags.AddError("Missing required field for create_type "+CreateTypeACME, "\"tos\" must be set to true.")
		}
		if m.DNSMapping.IsNull() || m.DNSMapping.IsUnknown() || len(m.DNSMapping.Elements()) == 0 {
			diags.AddError("Missing required field for create_type "+CreateTypeACME, "\"dns_mapping\" must have at least one entry.")
		}
	}

	return diags
}

// validateRenewDays rejects "renew_days" for any create_type other than
// ACME, ahead of the certificate.create/update call. configRenewDays must
// come from the request's raw Config (never Plan): certificate.create
// accepts renew_days for any create_type but silently ignores it and
// forces the default (10) instead for non-ACME types (probed live: a CSR
// create with renew_days=5 succeeded but read back renew_days=10);
// certificate.update is stricter and rejects it outright for non-ACME
// types (probed live: "[EINVAL] certificate_update.renew_days: Certificate
// renewal days is only supported for ACME certificates"). Either way, a
// config that sets renew_days on a non-ACME certificate can never be
// honored, so this is rejected client-side up front with a clear message
// rather than surfacing later as a confusing "provider produced an
// inconsistent result after apply" plan-consistency error — which is
// exactly what would happen checking the Plan value instead of Config:
// an Optional+Computed field the caller never wrote in HCL still plans as
// a concrete known value once a prior apply has populated it (carried
// forward by UseStateForUnknown), indistinguishable at that point from an
// explicit override. Config, by contrast, is always exactly null when the
// caller left the field out of their HCL, on every plan/apply.
func validateRenewDays(createType string, configRenewDays types.Int64) diag.Diagnostics {
	var diags diag.Diagnostics
	if createType != CreateTypeACME && !configRenewDays.IsNull() {
		diags.AddError("Invalid field for create_type "+createType,
			"\"renew_days\" is only supported for create_type "+CreateTypeACME+".")
	}
	return diags
}

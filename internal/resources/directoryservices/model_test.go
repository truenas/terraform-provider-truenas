// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package directoryservices

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// baseModel returns a DirectoryServicesModel with enable=false and every
// other attribute null, suitable as a starting point for the "disabled"
// payload-shape tests. enable is Required in the schema (never actually
// null in a real plan/config), so false — not null — is the realistic
// baseline.
func baseModel() *DirectoryServicesModel {
	return &DirectoryServicesModel{
		ID:                           types.StringNull(),
		ServiceType:                  types.StringNull(),
		Enable:                       types.BoolValue(false),
		EnableAccountCache:           types.BoolNull(),
		EnableDNSUpdates:             types.BoolNull(),
		Timeout:                      types.Int64Null(),
		KerberosRealm:                types.StringNull(),
		Credential:                   types.ObjectNull(credentialAttrTypes),
		ConfigurationActiveDirectory: types.ObjectNull(adConfigAttrTypes),
		ConfigurationLDAP:            types.ObjectNull(ldapConfigAttrTypes),
		ConfigurationIPA:             types.ObjectNull(ipaConfigAttrTypes),
	}
}

// validCredential builds a minimal, valid KERBEROS_USER credential object,
// for tests exercising the enable=true payload shape (which requires one).
func validCredential(t *testing.T, ctx context.Context) types.Object {
	t.Helper()
	return credentialObject(t, ctx, CredentialModel{
		CredentialType: types.StringValue("KERBEROS_USER"),
		Username:       types.StringValue("Administrator"),
		Password:       types.StringValue("hunter2"),
	})
}

// credentialObject builds a "credential" types.Object from a CredentialModel.
// Every types.String field left at its Go zero value serializes identically
// to an explicit types.StringNull() (StringValue's zero value IS the null
// state — see basetypes.StringValue's doc comment), so callers only need to
// set the fields relevant to the credential_type under test.
func credentialObject(t *testing.T, ctx context.Context, cred CredentialModel) types.Object {
	t.Helper()
	obj, diags := types.ObjectValueFrom(ctx, credentialAttrTypes, cred)
	if diags.HasError() {
		t.Fatalf("unexpected error building credential object: %v", diags)
	}
	return obj
}

// validADConfig builds a minimal, valid configuration_activedirectory
// object (idmap left null), for tests exercising the enable=true payload
// shape (which requires one).
func validADConfig(t *testing.T, ctx context.Context) types.Object {
	t.Helper()
	return adConfigObject(t, ctx, ActiveDirectoryConfigModel{
		Hostname:             types.StringValue("tn2510"),
		Domain:               types.StringValue("TFTEST.LAN"),
		Site:                 types.StringNull(),
		ComputerAccountOU:    types.StringNull(),
		UseDefaultDomain:     types.BoolValue(false),
		EnableTrustedDomains: types.BoolValue(false),
		Idmap:                types.ObjectNull(idmapAttrTypes),
	})
}

// adConfigObject builds a "configuration_activedirectory" types.Object from
// an ActiveDirectoryConfigModel. Unlike credentialObject's plain String
// fields, "Idmap" is an Object-typed field: its Go zero value is NOT a valid
// null (types.Object's null representation must carry the correct
// attributeTypes map), so every caller must explicitly set Idmap to either
// types.ObjectNull(idmapAttrTypes) or a real idmap object built via
// idmapObject.
func adConfigObject(t *testing.T, ctx context.Context, ad ActiveDirectoryConfigModel) types.Object {
	t.Helper()
	obj, diags := types.ObjectValueFrom(ctx, adConfigAttrTypes, ad)
	if diags.HasError() {
		t.Fatalf("unexpected error building AD config object: %v", diags)
	}
	return obj
}

// idmapBuiltinObject builds an "idmap.builtin" types.Object.
func idmapBuiltinObject(t *testing.T, ctx context.Context, b IdmapBuiltinModel) types.Object {
	t.Helper()
	obj, diags := types.ObjectValueFrom(ctx, idmapBuiltinAttrTypes, b)
	if diags.HasError() {
		t.Fatalf("unexpected error building idmap.builtin object: %v", diags)
	}
	return obj
}

// idmapDomainObject builds an "idmap.idmap_domain" types.Object.
func idmapDomainObject(t *testing.T, ctx context.Context, d IdmapDomainModel) types.Object {
	t.Helper()
	obj, diags := types.ObjectValueFrom(ctx, idmapDomainAttrTypes, d)
	if diags.HasError() {
		t.Fatalf("unexpected error building idmap.idmap_domain object: %v", diags)
	}
	return obj
}

// idmapObject builds an "idmap" types.Object from builtin/idmap_domain
// sub-objects (either of which may be types.ObjectNull to leave that
// sub-block unset).
func idmapObject(t *testing.T, ctx context.Context, builtin, idmapDomain types.Object) types.Object {
	t.Helper()
	obj, diags := types.ObjectValueFrom(ctx, idmapAttrTypes, IdmapModel{
		Builtin:     builtin,
		IdmapDomain: idmapDomain,
	})
	if diags.HasError() {
		t.Fatalf("unexpected error building idmap object: %v", diags)
	}
	return obj
}

// validLDAPConfig builds a minimal, valid configuration_ldap object
// (search_bases/attribute_maps left null), for tests exercising the LDAP
// enable=true payload shape.
func validLDAPConfig(t *testing.T, ctx context.Context) types.Object {
	t.Helper()
	serverURLs, diags := types.ListValueFrom(ctx, types.StringType, []string{"ldaps://ldap.tftest.lan"})
	if diags.HasError() {
		t.Fatalf("unexpected error building server_urls list: %v", diags)
	}
	obj, diags := types.ObjectValueFrom(ctx, ldapConfigAttrTypes, LDAPConfigModel{
		ServerURLs:           serverURLs,
		BaseDN:               types.StringValue("dc=tftest,dc=lan"),
		StartTLS:             types.BoolValue(false),
		ValidateCertificates: types.BoolValue(true),
		Schema:               types.StringValue("RFC2307"),
		AuxiliaryParameters:  types.StringNull(),
		SearchBases:          types.ObjectNull(ldapSearchBasesAttrTypes),
		AttributeMaps:        types.ObjectNull(ldapAttributeMapsAttrTypes),
	})
	if diags.HasError() {
		t.Fatalf("unexpected error building LDAP config object: %v", diags)
	}
	return obj
}

// validIPAConfig builds a minimal, valid configuration_ipa object
// (smb_domain left null), for tests exercising the IPA enable=true payload
// shape.
func validIPAConfig(t *testing.T, ctx context.Context) types.Object {
	t.Helper()
	obj, diags := types.ObjectValueFrom(ctx, ipaConfigAttrTypes, IPAConfigModel{
		TargetServer:         types.StringValue("ipa.tfipa.lan"),
		Hostname:             types.StringValue("tn2510"),
		Domain:               types.StringValue("tfipa.lan"),
		BaseDN:               types.StringValue("dc=tfipa,dc=lan"),
		SMBDomain:            types.ObjectNull(ipaSMBDomainAttrTypes),
		ValidateCertificates: types.BoolValue(true),
	})
	if diags.HasError() {
		t.Fatalf("unexpected error building IPA config object: %v", diags)
	}
	return obj
}

// enabledModel returns a DirectoryServicesModel with enable=true and valid
// credential/configuration_activedirectory already set — the minimum
// required shape once "enable" is true (see updatePayload's doc comment).
func enabledModel(t *testing.T, ctx context.Context) *DirectoryServicesModel {
	m := baseModel()
	m.Enable = types.BoolValue(true)
	m.ServiceType = types.StringValue("ACTIVEDIRECTORY")
	m.Credential = validCredential(t, ctx)
	m.ConfigurationActiveDirectory = validADConfig(t, ctx)
	return m
}

// TestUpdatePayload_DisableShape_OnlyScalars verifies that whenever
// enable=false, the payload is exactly {enable, enable_account_cache,
// enable_dns_updates, timeout} (each still individually guarded by
// null/unknown) — confirmed live: TrueNAS accepts a bare {"enable": false}
// to disable an active join.
func TestUpdatePayload_DisableShape_OnlyScalars(t *testing.T) {
	m := baseModel()
	m.EnableAccountCache = types.BoolValue(true)
	m.EnableDNSUpdates = types.BoolValue(false)
	m.Timeout = types.Int64Value(15)

	p, diags := m.updatePayload(context.Background(), nil)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	want := map[string]any{
		"enable":               false,
		"enable_account_cache": true,
		"enable_dns_updates":   false,
		"timeout":              int64(15),
	}
	for k, v := range want {
		if p[k] != v {
			t.Errorf("payload[%q] = %v (%T), want %v (%T)", k, p[k], p[k], v, v)
		}
	}
	if len(p) != len(want) {
		t.Errorf("payload has %d keys (%v), want %d", len(p), p, len(want))
	}
}

// TestUpdatePayload_DisableShape_MinimalWhenAllScalarsUnset verifies the
// absolute minimum disable payload: just {"enable": false}.
func TestUpdatePayload_DisableShape_MinimalWhenAllScalarsUnset(t *testing.T) {
	m := baseModel()
	p, diags := m.updatePayload(context.Background(), nil)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	want := map[string]any{"enable": false}
	if len(p) != len(want) || p["enable"] != false {
		t.Errorf("payload = %v, want %v", p, want)
	}
}

// TestUpdatePayload_DisableIgnoresJoinFields is a regression test for a bug
// found live against a TrueNAS 25.10 box: TrueNAS rejects
// directoryservices.update with "[EINVAL]
// directoryservices.update.configuration: Permitted changes while
// directory services are enabled are limited to account caching, DNS
// updates, and timeouts" whenever "configuration" differs at all from what
// is currently persisted while directory services are (or would stay)
// enabled. Once enable=false, service_type/kerberos_realm/credential/
// configuration must be omitted entirely — even if a user's HCL still has
// them set (e.g. they flipped enable to false without deleting the
// credential/configuration_activedirectory blocks).
func TestUpdatePayload_DisableIgnoresJoinFields(t *testing.T) {
	ctx := context.Background()
	m := enabledModel(t, ctx)
	m.Enable = types.BoolValue(false) // disabling, but join fields still set
	m.KerberosRealm = types.StringValue("TFTEST.LAN")

	p, diags := m.updatePayload(ctx, nil)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	forbidden := []string{"service_type", "kerberos_realm", "credential", "configuration"}
	for _, k := range forbidden {
		if _, ok := p[k]; ok {
			t.Errorf("disable-shape payload must never include %q, got %v", k, p[k])
		}
	}
	if p["enable"] != false {
		t.Errorf(`payload["enable"] = %v, want false`, p["enable"])
	}
}

// TestUpdatePayload_EnableRequiresCredentialAndConfiguration verifies that
// enable=true without a valid credential and/or
// configuration_activedirectory fails fast with a diagnostic, rather than
// sending an incomplete payload TrueNAS would reject anyway with a much
// less clear EINVAL.
func TestUpdatePayload_EnableRequiresCredentialAndConfiguration(t *testing.T) {
	ctx := context.Background()

	t.Run("both missing", func(t *testing.T) {
		m := baseModel()
		m.Enable = types.BoolValue(true)
		_, diags := m.updatePayload(ctx, nil)
		if !diags.HasError() {
			t.Fatal("expected an error when enable=true with no credential/configuration")
		}
	})

	t.Run("credential missing", func(t *testing.T) {
		m := baseModel()
		m.Enable = types.BoolValue(true)
		m.ConfigurationActiveDirectory = validADConfig(t, ctx)
		_, diags := m.updatePayload(ctx, nil)
		if !diags.HasError() {
			t.Fatal("expected an error when enable=true with no credential")
		}
	})

	t.Run("configuration missing", func(t *testing.T) {
		m := baseModel()
		m.Enable = types.BoolValue(true)
		m.Credential = validCredential(t, ctx)
		_, diags := m.updatePayload(ctx, nil)
		if !diags.HasError() {
			t.Fatal("expected an error when enable=true with no configuration_activedirectory")
		}
	})

	t.Run("service_type missing", func(t *testing.T) {
		m := baseModel()
		m.Enable = types.BoolValue(true)
		m.Credential = validCredential(t, ctx)
		m.ConfigurationActiveDirectory = validADConfig(t, ctx)
		_, diags := m.updatePayload(ctx, nil)
		if !diags.HasError() {
			t.Fatal("expected an error when enable=true with no service_type")
		}
	})

	t.Run("credential missing but existing KERBEROS_PRINCIPAL available", func(t *testing.T) {
		// Regression coverage: once joined, m.Credential may legitimately
		// be omitted from HCL — this must NOT error, and must reuse the
		// existing principal (see the next test for the payload shape).
		// ServiceType is set here too: an already-joined resource always
		// has it populated from a prior apply (Optional+Computed).
		m := baseModel()
		m.Enable = types.BoolValue(true)
		m.ServiceType = types.StringValue("ACTIVEDIRECTORY")
		m.ConfigurationActiveDirectory = validADConfig(t, ctx)

		serviceType := "ACTIVEDIRECTORY"
		principal := "TN2510$@TFTEST.LAN"
		existing := &directoryServicesAPI{
			ServiceType: &serviceType,
			Credential:  &directoryServicesCredentialAPI{CredentialType: "KERBEROS_PRINCIPAL", Principal: &principal},
		}
		_, diags := m.updatePayload(ctx, existing)
		if diags.HasError() {
			t.Fatalf("unexpected error when a reusable KERBEROS_PRINCIPAL exists: %v", diags)
		}
	})
}

// TestUpdatePayload_ReusesExistingKerberosPrincipal is a regression test
// for a bug found live against a TrueNAS 25.10 box: TrueNAS swaps a raw
// KERBEROS_USER admin credential used for the initial join into a
// KERBEROS_PRINCIPAL backed by the new machine account's own keytab, and
// then REJECTS resending the raw KERBEROS_USER credential on a later call
// (the disabling call specifically failed with "[EINVAL]
// directoryservices.update.credential.credential_type: Kerberos user
// credentials may not be stored for disabled directory services...",
// because the immediately preceding update had re-stored a raw
// KERBEROS_USER). Once existing has a usable KERBEROS_PRINCIPAL, it must
// be resent verbatim instead of m.Credential — even when m.Credential is
// also set (e.g. the user hasn't removed it from HCL yet).
func TestUpdatePayload_ReusesExistingKerberosPrincipal(t *testing.T) {
	ctx := context.Background()
	m := enabledModel(t, ctx) // has a KERBEROS_USER m.Credential set

	serviceType := "ACTIVEDIRECTORY"
	principal := "TN2510$@TFTEST.LAN"
	existing := &directoryServicesAPI{
		ServiceType: &serviceType,
		Credential:  &directoryServicesCredentialAPI{CredentialType: "KERBEROS_PRINCIPAL", Principal: &principal},
	}

	p, diags := m.updatePayload(ctx, existing)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	got, ok := p["credential"].(map[string]any)
	if !ok {
		t.Fatalf("payload[\"credential\"] = %T, want map[string]any", p["credential"])
	}
	want := map[string]any{
		"credential_type": "KERBEROS_PRINCIPAL",
		"principal":       "TN2510$@TFTEST.LAN",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("credential[%q] = %v, want %v", k, got[k], v)
		}
	}
	if len(got) != len(want) {
		t.Errorf("credential has %d keys (%v), want %d — must not include username/password", len(got), got, len(want))
	}
}

// TestUpdatePayload_EnableShape_FullPayload verifies that enable=true with
// a valid credential and configuration produces the full join-shape
// payload confirmed live: service_type, credential, and configuration are
// all present alongside the scalar fields.
func TestUpdatePayload_EnableShape_FullPayload(t *testing.T) {
	ctx := context.Background()
	m := enabledModel(t, ctx)
	m.EnableAccountCache = types.BoolValue(false)
	m.EnableDNSUpdates = types.BoolValue(false)
	m.Timeout = types.Int64Value(30)

	p, diags := m.updatePayload(ctx, nil)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	wantScalars := map[string]any{
		"service_type":         "ACTIVEDIRECTORY",
		"enable":               true,
		"enable_account_cache": false,
		"enable_dns_updates":   false,
		"timeout":              int64(30),
	}
	for k, v := range wantScalars {
		if p[k] != v {
			t.Errorf("payload[%q] = %v (%T), want %v (%T)", k, p[k], p[k], v, v)
		}
	}

	gotCred, ok := p["credential"].(map[string]any)
	if !ok {
		t.Fatalf("payload[\"credential\"] = %T, want map[string]any", p["credential"])
	}
	wantCred := map[string]any{
		"credential_type": "KERBEROS_USER",
		"username":        "Administrator",
		"password":        "hunter2",
	}
	for k, v := range wantCred {
		if gotCred[k] != v {
			t.Errorf("credential[%q] = %v, want %v", k, gotCred[k], v)
		}
	}

	gotConfig, ok := p["configuration"].(map[string]any)
	if !ok {
		t.Fatalf("payload[\"configuration\"] = %T, want map[string]any", p["configuration"])
	}
	wantConfig := map[string]any{
		"service_type":           "ACTIVEDIRECTORY",
		"hostname":               "tn2510",
		"domain":                 "TFTEST.LAN",
		"site":                   nil,
		"computer_account_ou":    nil,
		"use_default_domain":     false,
		"enable_trusted_domains": false,
	}
	for k, v := range wantConfig {
		gv, present := gotConfig[k]
		if !present {
			t.Errorf("configuration missing key %q", k)
			continue
		}
		if gv != v {
			t.Errorf("configuration[%q] = %v, want %v", k, gv, v)
		}
	}
	if _, present := gotConfig["idmap"]; present {
		t.Error("configuration should not include idmap when existing is nil (fresh join)")
	}
	if _, present := gotConfig["trusted_domains"]; present {
		t.Error("configuration should not include trusted_domains when existing is nil (fresh join)")
	}
}

// TestUpdatePayload_ConfigurationSiteAndOUThreeWay verifies the
// configuration_activedirectory nested block's site/computer_account_ou
// follow the same three-way clear convention as kerberos_realm.
func TestUpdatePayload_ConfigurationSiteAndOUThreeWay(t *testing.T) {
	ctx := context.Background()
	ad := adConfigObject(t, ctx, ActiveDirectoryConfigModel{
		Hostname:             types.StringValue("tn2510"),
		Domain:               types.StringValue("TFTEST.LAN"),
		Site:                 types.StringValue(""), // explicit clear -> nil
		ComputerAccountOU:    types.StringValue("TRUENAS_SERVERS"),
		UseDefaultDomain:     types.BoolValue(true),
		EnableTrustedDomains: types.BoolValue(false),
		Idmap:                types.ObjectNull(idmapAttrTypes),
	})

	m := enabledModel(t, ctx)
	m.ConfigurationActiveDirectory = ad

	p, diags := m.updatePayload(ctx, nil)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	got, ok := p["configuration"].(map[string]any)
	if !ok {
		t.Fatalf("payload[\"configuration\"] = %T, want map[string]any", p["configuration"])
	}
	if v, present := got["site"]; !present || v != nil {
		t.Errorf(`configuration["site"] = %v (present=%v), want nil`, v, present)
	}
	if v, present := got["computer_account_ou"]; !present || v != "TRUENAS_SERVERS" {
		t.Errorf(`configuration["computer_account_ou"] = %v (present=%v), want "TRUENAS_SERVERS"`, v, present)
	}
}

// TestUpdatePayload_TrustedDomainsRoundTrip verifies that trusted_domains
// from a previously-fetched directoryServicesAPI is copied verbatim into
// the outgoing "configuration" map — required so an in-place update (e.g.
// bumping "timeout") on an already-joined resource doesn't look like a
// trusted_domains change to TrueNAS (out of scope for Terraform state; see
// the "existing" doc comment on updatePayload). idmap itself is NO LONGER
// round-tripped this way — see TestUpdatePayload_AD_IdmapFromModel — this
// mechanism now only applies to trusted_domains. Regression coverage for a
// live bug: the nested "configuration" object returned by the API does NOT
// echo back its own "service_type" key, so the round-trip guard must key
// off the TOP-LEVEL ServiceType (here, not the nested
// Configuration.ServiceType, which is always "" from a real response).
func TestUpdatePayload_TrustedDomainsRoundTrip(t *testing.T) {
	ctx := context.Background()
	m := enabledModel(t, ctx)

	serviceType := "ACTIVEDIRECTORY"
	existing := &directoryServicesAPI{
		ServiceType: &serviceType,
		Configuration: &directoryServicesConfigAPI{
			// ServiceType intentionally left "" (zero value): a real
			// directoryservices.config response never echoes it back on
			// the nested object, only on the top-level one above.
			Hostname:       "tn2510",
			Domain:         "TFTEST.LAN",
			TrustedDomains: json.RawMessage(`[]`),
		},
	}

	p, diags := m.updatePayload(ctx, existing)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	got, ok := p["configuration"].(map[string]any)
	if !ok {
		t.Fatalf("payload[\"configuration\"] = %T, want map[string]any", p["configuration"])
	}
	trustedDomains, ok := got["trusted_domains"].([]any)
	if !ok || len(trustedDomains) != 0 {
		t.Errorf("configuration[\"trusted_domains\"] = %v, want empty array", got["trusted_domains"])
	}
}

// TestUpdatePayload_TrustedDomainsIgnoredForOtherServiceType verifies that
// existing.Configuration.TrustedDomains is only round-tripped when the
// existing configuration's own top-level ServiceType is ACTIVEDIRECTORY
// (defensive: trusted_domains doesn't exist on the IPA/LDAP variants).
func TestUpdatePayload_TrustedDomainsIgnoredForOtherServiceType(t *testing.T) {
	ctx := context.Background()
	m := enabledModel(t, ctx)

	serviceType := "LDAP"
	existing := &directoryServicesAPI{
		ServiceType: &serviceType,
		Configuration: &directoryServicesConfigAPI{
			TrustedDomains: json.RawMessage(`["should not appear"]`),
		},
	}

	p, diags := m.updatePayload(ctx, existing)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	got, ok := p["configuration"].(map[string]any)
	if !ok {
		t.Fatalf("payload[\"configuration\"] = %T, want map[string]any", p["configuration"])
	}
	if _, present := got["trusted_domains"]; present {
		t.Error("trusted_domains should not be round-tripped when existing.ServiceType != ACTIVEDIRECTORY")
	}
}

// TestUpdatePayload_KerberosRealmThreeWay verifies the null/omitted,
// unknown/omitted, explicit-empty/nil, and value/value cases for
// kerberos_realm (only sent in the enable=true payload shape), mirroring
// network_config's nameserver1-3 three-way guard.
func TestUpdatePayload_KerberosRealmThreeWay(t *testing.T) {
	ctx := context.Background()

	// null -> omitted
	m := enabledModel(t, ctx)
	m.KerberosRealm = types.StringNull()
	p, diags := m.updatePayload(ctx, nil)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, ok := p["kerberos_realm"]; ok {
		t.Errorf("expected kerberos_realm to be omitted when null")
	}

	// unknown -> omitted
	m = enabledModel(t, ctx)
	m.KerberosRealm = types.StringUnknown()
	p, diags = m.updatePayload(ctx, nil)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, ok := p["kerberos_realm"]; ok {
		t.Errorf("expected kerberos_realm to be omitted when unknown")
	}

	// explicit "" -> nil (clears the realm on TrueNAS)
	m = enabledModel(t, ctx)
	m.KerberosRealm = types.StringValue("")
	p, diags = m.updatePayload(ctx, nil)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	v, ok := p["kerberos_realm"]
	if !ok {
		t.Fatalf("expected kerberos_realm to be present when explicitly \"\"")
	}
	if v != nil {
		t.Errorf("kerberos_realm = %v, want nil", v)
	}

	// value -> value
	m = enabledModel(t, ctx)
	m.KerberosRealm = types.StringValue("EXAMPLE.INTERNAL")
	p, diags = m.updatePayload(ctx, nil)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if v, ok := p["kerberos_realm"]; !ok || v != "EXAMPLE.INTERNAL" {
		t.Errorf("kerberos_realm = %v (present=%v), want EXAMPLE.INTERNAL", v, ok)
	}
}

// TestResponseToModel_MapsPlainFields verifies responseToModel against the
// shape probed from a live directoryservices.config call while joined to
// Active Directory.
func TestResponseToModel_MapsPlainFields(t *testing.T) {
	ctx := context.Background()
	serviceType := "ACTIVEDIRECTORY"
	realm := "TFTEST.LAN"
	site := "Default-First-Site-Name"
	api := &directoryServicesAPI{
		ID:                 1,
		ServiceType:        &serviceType,
		Enable:             true,
		EnableAccountCache: true,
		EnableDNSUpdates:   true,
		Timeout:            10,
		KerberosRealm:      &realm,
		Configuration: &directoryServicesConfigAPI{
			ServiceType:          "ACTIVEDIRECTORY",
			Hostname:             "tn2510",
			Domain:               "TFTEST.LAN",
			Site:                 &site,
			ComputerAccountOU:    nil,
			UseDefaultDomain:     false,
			EnableTrustedDomains: false,
		},
	}

	m := &DirectoryServicesModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueString() != directoryServicesResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), directoryServicesResourceID)
	}
	if m.ServiceType.ValueString() != "ACTIVEDIRECTORY" {
		t.Errorf("ServiceType = %q, want ACTIVEDIRECTORY", m.ServiceType.ValueString())
	}
	if !m.Enable.ValueBool() {
		t.Error("Enable = false, want true")
	}
	if m.KerberosRealm.ValueString() != "TFTEST.LAN" {
		t.Errorf("KerberosRealm = %q, want TFTEST.LAN", m.KerberosRealm.ValueString())
	}
	if m.ConfigurationActiveDirectory.IsNull() {
		t.Fatal("ConfigurationActiveDirectory should not be null")
	}
	var ad ActiveDirectoryConfigModel
	diags = m.ConfigurationActiveDirectory.As(ctx, &ad, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("unexpected error extracting AD config: %v", diags)
	}
	if ad.Hostname.ValueString() != "tn2510" {
		t.Errorf("Hostname = %q, want tn2510", ad.Hostname.ValueString())
	}
	if ad.Site.ValueString() != "Default-First-Site-Name" {
		t.Errorf("Site = %q, want Default-First-Site-Name", ad.Site.ValueString())
	}
	if ad.ComputerAccountOU.ValueString() != "" {
		t.Errorf("ComputerAccountOU = %q, want \"\" (nil on wire)", ad.ComputerAccountOU.ValueString())
	}
}

// TestResponseToModel_NeverFillsPassword is the "mapper never fills
// password" requirement from the task-8 brief: even starting from a model
// whose credential already carries a password, and mapping in an API
// response, responseToModel must leave Credential completely untouched
// (the API response is never decoded into anything password-shaped in the
// first place — see directoryServicesAPI's doc comment).
func TestResponseToModel_NeverFillsPassword(t *testing.T) {
	ctx := context.Background()
	cred, diags := types.ObjectValueFrom(ctx, credentialAttrTypes, CredentialModel{
		CredentialType: types.StringValue("KERBEROS_USER"),
		Username:       types.StringValue("Administrator"),
		Password:       types.StringValue("hunter2"),
	})
	if diags.HasError() {
		t.Fatalf("unexpected error building credential object: %v", diags)
	}

	m := &DirectoryServicesModel{Credential: cred}

	serviceType := "ACTIVEDIRECTORY"
	api := &directoryServicesAPI{
		ID:          1,
		ServiceType: &serviceType,
		Enable:      true,
		Timeout:     10,
		Configuration: &directoryServicesConfigAPI{
			ServiceType: "ACTIVEDIRECTORY",
			Hostname:    "tn2510",
			Domain:      "TFTEST.LAN",
		},
	}

	diags = responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	var got CredentialModel
	diags = m.Credential.As(ctx, &got, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("unexpected error extracting credential: %v", diags)
	}
	if got.Password.ValueString() != "hunter2" {
		t.Errorf("Credential.Password changed by responseToModel: got %q, want unchanged \"hunter2\"", got.Password.ValueString())
	}
}

// TestResponseToModel_NullServiceTypeAndConfiguration verifies the
// never-joined shape confirmed live on a disposable box:
// {"configuration":null,"credential":null,"enable":false,
// "enable_account_cache":true,"enable_dns_updates":true,"id":1,
// "kerberos_realm":null,"service_type":null,"timeout":10}.
func TestResponseToModel_NullServiceTypeAndConfiguration(t *testing.T) {
	ctx := context.Background()
	api := &directoryServicesAPI{
		ID:                 1,
		ServiceType:        nil,
		Enable:             false,
		EnableAccountCache: true,
		EnableDNSUpdates:   true,
		Timeout:            10,
		KerberosRealm:      nil,
		Configuration:      nil,
	}

	m := &DirectoryServicesModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if !m.ServiceType.IsNull() {
		t.Errorf("ServiceType = %v, want null", m.ServiceType)
	}
	if m.Enable.ValueBool() {
		t.Error("Enable = true, want false")
	}
	if m.KerberosRealm.ValueString() != "" {
		t.Errorf("KerberosRealm = %q, want \"\" (nil-on-wire mapped to empty)", m.KerberosRealm.ValueString())
	}
	if !m.ConfigurationActiveDirectory.IsNull() {
		t.Errorf("ConfigurationActiveDirectory = %v, want null", m.ConfigurationActiveDirectory)
	}
}

// TestResponseToDataSourceModel_StatusFields verifies that status/status_msg
// are mapped from the separate directoryservices.status response.
func TestResponseToDataSourceModel_StatusFields(t *testing.T) {
	ctx := context.Background()
	serviceType := "ACTIVEDIRECTORY"
	api := &directoryServicesAPI{
		ID:          1,
		ServiceType: &serviceType,
		Enable:      true,
		Timeout:     10,
	}
	status := "HEALTHY"
	statusAPI := &directoryServicesStatusAPI{
		Type:      &serviceType,
		Status:    &status,
		StatusMsg: nil,
	}

	m := &DirectoryServicesDataSourceModel{}
	diags := responseToDataSourceModel(ctx, api, statusAPI, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.Status.ValueString() != "HEALTHY" {
		t.Errorf("Status = %q, want HEALTHY", m.Status.ValueString())
	}
	if !m.StatusMsg.IsNull() {
		t.Errorf("StatusMsg = %v, want null", m.StatusMsg)
	}
}

// ---------------------------------------------------------------------
// LDAP / IPA / credential-variant / idmap / exactly-one-config coverage
// (Plan 14 Task 1)
// ---------------------------------------------------------------------

// ldapEnabledModel returns a DirectoryServicesModel with enable=true,
// service_type=LDAP, a valid LDAP_PLAIN credential, and a minimal valid
// configuration_ldap block.
func ldapEnabledModel(t *testing.T, ctx context.Context) *DirectoryServicesModel {
	m := baseModel()
	m.Enable = types.BoolValue(true)
	m.ServiceType = types.StringValue("LDAP")
	m.Credential = credentialObject(t, ctx, CredentialModel{
		CredentialType: types.StringValue("LDAP_PLAIN"),
		BindDN:         types.StringValue("cn=admin,dc=tftest,dc=lan"),
		BindPW:         types.StringValue("hunter2"),
	})
	m.ConfigurationLDAP = validLDAPConfig(t, ctx)
	return m
}

// ipaEnabledModel returns a DirectoryServicesModel with enable=true,
// service_type=IPA, a valid KERBEROS_USER credential, and a minimal valid
// configuration_ipa block.
func ipaEnabledModel(t *testing.T, ctx context.Context) *DirectoryServicesModel {
	m := baseModel()
	m.Enable = types.BoolValue(true)
	m.ServiceType = types.StringValue("IPA")
	m.Credential = validCredential(t, ctx)
	m.ConfigurationIPA = validIPAConfig(t, ctx)
	return m
}

// TestUpdatePayload_LDAP_MinimalPayload verifies the LDAP configuration
// payload shape when search_bases/attribute_maps are left unset: they must
// be omitted entirely (server defaults apply), and credential_type
// LDAP_ANONYMOUS produces a bare {"credential_type":"LDAP_ANONYMOUS"}.
func TestUpdatePayload_LDAP_MinimalPayload(t *testing.T) {
	ctx := context.Background()
	m := ldapEnabledModel(t, ctx)
	m.Credential = credentialObject(t, ctx, CredentialModel{
		CredentialType: types.StringValue("LDAP_ANONYMOUS"),
	})

	p, diags := m.updatePayload(ctx, nil)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if p["service_type"] != "LDAP" {
		t.Errorf(`payload["service_type"] = %v, want "LDAP"`, p["service_type"])
	}
	gotCred, ok := p["credential"].(map[string]any)
	if !ok || len(gotCred) != 1 || gotCred["credential_type"] != "LDAP_ANONYMOUS" {
		t.Errorf(`payload["credential"] = %v, want {"credential_type":"LDAP_ANONYMOUS"}`, p["credential"])
	}

	gotConfig, ok := p["configuration"].(map[string]any)
	if !ok {
		t.Fatalf("payload[\"configuration\"] = %T, want map[string]any", p["configuration"])
	}
	if gotConfig["service_type"] != "LDAP" {
		t.Errorf(`configuration["service_type"] = %v, want "LDAP"`, gotConfig["service_type"])
	}
	if gotConfig["basedn"] != "dc=tftest,dc=lan" {
		t.Errorf(`configuration["basedn"] = %v, want "dc=tftest,dc=lan"`, gotConfig["basedn"])
	}
	urls, ok := gotConfig["server_urls"].([]string)
	if !ok || len(urls) != 1 || urls[0] != "ldaps://ldap.tftest.lan" {
		t.Errorf(`configuration["server_urls"] = %v, want ["ldaps://ldap.tftest.lan"]`, gotConfig["server_urls"])
	}
	if _, present := gotConfig["search_bases"]; present {
		t.Error(`configuration["search_bases"] should be omitted when unset`)
	}
	if _, present := gotConfig["attribute_maps"]; present {
		t.Error(`configuration["attribute_maps"] should be omitted when unset`)
	}
	if _, present := gotConfig["auxiliary_parameters"]; present {
		t.Error(`configuration["auxiliary_parameters"] should be omitted when unset (null)`)
	}
}

// TestUpdatePayload_LDAP_FullPayload verifies the LDAP configuration
// payload shape when search_bases and attribute_maps ARE set: only the
// leaf fields the caller actually set are included, following the same
// three-way clearable-string convention as the rest of the package.
func TestUpdatePayload_LDAP_FullPayload(t *testing.T) {
	ctx := context.Background()
	m := ldapEnabledModel(t, ctx)

	searchBases, diags := types.ObjectValueFrom(ctx, ldapSearchBasesAttrTypes, LDAPSearchBasesModel{
		BaseUser:     types.StringValue("ou=people,dc=tftest,dc=lan"),
		BaseGroup:    types.StringNull(),
		BaseNetgroup: types.StringNull(),
	})
	if diags.HasError() {
		t.Fatalf("unexpected error building search_bases: %v", diags)
	}

	passwd, diags := types.ObjectValueFrom(ctx, ldapAttrMapPasswdAttrTypes, LDAPAttrMapPasswdModel{
		UserObjectClass:   types.StringValue("posixAccount"),
		UserName:          types.StringNull(),
		UserUID:           types.StringNull(),
		UserGID:           types.StringNull(),
		UserGecos:         types.StringNull(),
		UserHomeDirectory: types.StringNull(),
		UserShell:         types.StringNull(),
	})
	if diags.HasError() {
		t.Fatalf("unexpected error building attribute_maps.passwd: %v", diags)
	}
	attrMaps, diags := types.ObjectValueFrom(ctx, ldapAttributeMapsAttrTypes, LDAPAttributeMapsModel{
		Passwd:   passwd,
		Shadow:   types.ObjectNull(ldapAttrMapShadowAttrTypes),
		Group:    types.ObjectNull(ldapAttrMapGroupAttrTypes),
		Netgroup: types.ObjectNull(ldapAttrMapNetgroupAttrTypes),
	})
	if diags.HasError() {
		t.Fatalf("unexpected error building attribute_maps: %v", diags)
	}

	var l LDAPConfigModel
	diags = m.ConfigurationLDAP.As(ctx, &l, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("unexpected error extracting LDAP config: %v", diags)
	}
	l.SearchBases = searchBases
	l.AttributeMaps = attrMaps
	obj, diags := types.ObjectValueFrom(ctx, ldapConfigAttrTypes, l)
	if diags.HasError() {
		t.Fatalf("unexpected error rebuilding LDAP config: %v", diags)
	}
	m.ConfigurationLDAP = obj

	p, diags := m.updatePayload(ctx, nil)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	gotConfig := p["configuration"].(map[string]any)

	sb, ok := gotConfig["search_bases"].(map[string]any)
	if !ok {
		t.Fatalf(`configuration["search_bases"] = %T, want map[string]any`, gotConfig["search_bases"])
	}
	if sb["base_user"] != "ou=people,dc=tftest,dc=lan" {
		t.Errorf(`search_bases["base_user"] = %v, want "ou=people,dc=tftest,dc=lan"`, sb["base_user"])
	}
	if _, present := sb["base_group"]; present {
		t.Error(`search_bases["base_group"] should be omitted when unset`)
	}

	am, ok := gotConfig["attribute_maps"].(map[string]any)
	if !ok {
		t.Fatalf(`configuration["attribute_maps"] = %T, want map[string]any`, gotConfig["attribute_maps"])
	}
	passwdOut, ok := am["passwd"].(map[string]any)
	if !ok || passwdOut["user_object_class"] != "posixAccount" {
		t.Errorf(`attribute_maps["passwd"]["user_object_class"] = %v, want "posixAccount"`, am["passwd"])
	}
	if _, present := am["shadow"]; present {
		t.Error(`attribute_maps["shadow"] should be omitted when its whole block is unset`)
	}
}

// TestUpdatePayload_IPA_Payload verifies the IPA configuration payload
// shape: target_server/hostname/domain/basedn always present, smb_domain
// omitted when unset (the normal case — it's server-detected during join).
func TestUpdatePayload_IPA_Payload(t *testing.T) {
	ctx := context.Background()
	m := ipaEnabledModel(t, ctx)

	p, diags := m.updatePayload(ctx, nil)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if p["service_type"] != "IPA" {
		t.Errorf(`payload["service_type"] = %v, want "IPA"`, p["service_type"])
	}
	gotConfig, ok := p["configuration"].(map[string]any)
	if !ok {
		t.Fatalf("payload[\"configuration\"] = %T, want map[string]any", p["configuration"])
	}
	wantConfig := map[string]any{
		"service_type":  "IPA",
		"target_server": "ipa.tfipa.lan",
		"hostname":      "tn2510",
		"domain":        "tfipa.lan",
		"basedn":        "dc=tfipa,dc=lan",
	}
	for k, v := range wantConfig {
		if gotConfig[k] != v {
			t.Errorf("configuration[%q] = %v, want %v", k, gotConfig[k], v)
		}
	}
	if _, present := gotConfig["smb_domain"]; present {
		t.Error(`configuration["smb_domain"] should be omitted when unset`)
	}
}

// TestUpdatePayload_CredentialVariants table-tests credentialPayload for
// all five probed credential_type values.
func TestUpdatePayload_CredentialVariants(t *testing.T) {
	tests := []struct {
		name string
		cred CredentialModel
		want map[string]any
	}{
		{
			name: "KERBEROS_USER",
			cred: CredentialModel{
				CredentialType: types.StringValue("KERBEROS_USER"),
				Username:       types.StringValue("Administrator"),
				Password:       types.StringValue("hunter2"),
			},
			want: map[string]any{"credential_type": "KERBEROS_USER", "username": "Administrator", "password": "hunter2"},
		},
		{
			name: "KERBEROS_PRINCIPAL",
			cred: CredentialModel{
				CredentialType: types.StringValue("KERBEROS_PRINCIPAL"),
				Principal:      types.StringValue("TN2510$@TFTEST.LAN"),
			},
			want: map[string]any{"credential_type": "KERBEROS_PRINCIPAL", "principal": "TN2510$@TFTEST.LAN"},
		},
		{
			name: "LDAP_PLAIN",
			cred: CredentialModel{
				CredentialType: types.StringValue("LDAP_PLAIN"),
				BindDN:         types.StringValue("cn=admin,dc=tftest,dc=lan"),
				BindPW:         types.StringValue("hunter2"),
			},
			want: map[string]any{"credential_type": "LDAP_PLAIN", "binddn": "cn=admin,dc=tftest,dc=lan", "bindpw": "hunter2"},
		},
		{
			name: "LDAP_ANONYMOUS",
			cred: CredentialModel{
				CredentialType: types.StringValue("LDAP_ANONYMOUS"),
			},
			want: map[string]any{"credential_type": "LDAP_ANONYMOUS"},
		},
		{
			name: "LDAP_MTLS",
			cred: CredentialModel{
				CredentialType:    types.StringValue("LDAP_MTLS"),
				ClientCertificate: types.StringValue("my-client-cert"),
			},
			want: map[string]any{"credential_type": "LDAP_MTLS", "client_certificate": "my-client-cert"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diags := validateCredential(tt.cred); diags.HasError() {
				t.Fatalf("unexpected validation error: %v", diags)
			}
			got := credentialPayload(tt.cred)
			if len(got) != len(tt.want) {
				t.Fatalf("credentialPayload() = %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("credentialPayload()[%q] = %v, want %v", k, got[k], v)
				}
			}
		})
	}
}

// TestValidateCredential_MissingFields table-tests the per-credential-type
// required-field diagnostics.
func TestValidateCredential_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		cred CredentialModel
	}{
		{"KERBEROS_USER missing username", CredentialModel{CredentialType: types.StringValue("KERBEROS_USER"), Password: types.StringValue("x")}},
		{"KERBEROS_USER missing password", CredentialModel{CredentialType: types.StringValue("KERBEROS_USER"), Username: types.StringValue("x")}},
		{"KERBEROS_PRINCIPAL missing principal", CredentialModel{CredentialType: types.StringValue("KERBEROS_PRINCIPAL")}},
		{"LDAP_PLAIN missing binddn", CredentialModel{CredentialType: types.StringValue("LDAP_PLAIN"), BindPW: types.StringValue("x")}},
		{"LDAP_PLAIN missing bindpw", CredentialModel{CredentialType: types.StringValue("LDAP_PLAIN"), BindDN: types.StringValue("x")}},
		{"LDAP_MTLS missing client_certificate", CredentialModel{CredentialType: types.StringValue("LDAP_MTLS")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diags := validateCredential(tt.cred); !diags.HasError() {
				t.Fatal("expected a validation error, got none")
			}
		})
	}
}

// TestUpdatePayload_ExactlyOneConfigBlock_NoneSet verifies the preflight
// error when enable=true and service_type is set, but no configuration_*
// block is set at all.
func TestUpdatePayload_ExactlyOneConfigBlock_NoneSet(t *testing.T) {
	ctx := context.Background()
	m := baseModel()
	m.Enable = types.BoolValue(true)
	m.ServiceType = types.StringValue("LDAP")
	m.Credential = credentialObject(t, ctx, CredentialModel{CredentialType: types.StringValue("LDAP_ANONYMOUS")})

	_, diags := m.updatePayload(ctx, nil)
	if !diags.HasError() {
		t.Fatal("expected an error when no configuration_* block is set")
	}
}

// TestUpdatePayload_ExactlyOneConfigBlock_TwoSet verifies the preflight
// error when more than one configuration_* block is set simultaneously.
func TestUpdatePayload_ExactlyOneConfigBlock_TwoSet(t *testing.T) {
	ctx := context.Background()
	m := enabledModel(t, ctx) // has configuration_activedirectory set
	m.ConfigurationLDAP = validLDAPConfig(t, ctx)

	_, diags := m.updatePayload(ctx, nil)
	if !diags.HasError() {
		t.Fatal("expected an error when two configuration_* blocks are set")
	}
}

// TestUpdatePayload_ExactlyOneConfigBlock_ServiceTypeMismatch verifies the
// preflight error when exactly one configuration_* block is set, but it
// doesn't match service_type.
func TestUpdatePayload_ExactlyOneConfigBlock_ServiceTypeMismatch(t *testing.T) {
	ctx := context.Background()
	m := baseModel()
	m.Enable = types.BoolValue(true)
	m.ServiceType = types.StringValue("LDAP")
	m.Credential = credentialObject(t, ctx, CredentialModel{CredentialType: types.StringValue("LDAP_ANONYMOUS")})
	m.ConfigurationActiveDirectory = validADConfig(t, ctx) // wrong block for service_type=LDAP

	_, diags := m.updatePayload(ctx, nil)
	if !diags.HasError() {
		t.Fatal("expected an error when the set configuration_* block doesn't match service_type")
	}
}

// TestUpdatePayload_AD_IdmapFromModel verifies that an explicitly-set
// idmap block (builtin + idmap_domain RID) is reconstructed byte-for-byte
// into the outgoing "configuration.idmap" payload.
func TestUpdatePayload_AD_IdmapFromModel(t *testing.T) {
	ctx := context.Background()
	m := enabledModel(t, ctx)

	builtin := idmapBuiltinObject(t, ctx, IdmapBuiltinModel{
		Name:      types.StringNull(),
		RangeLow:  types.Int64Value(90000001),
		RangeHigh: types.Int64Value(100000000),
	})
	idmapDomain := idmapDomainObject(t, ctx, IdmapDomainModel{
		IdmapBackend: types.StringValue("RID"),
		Name:         types.StringValue("TFTEST"),
		RangeLow:     types.Int64Value(100000001),
		RangeHigh:    types.Int64Value(200000000),
		SSSDCompat:   types.BoolValue(false),
		SchemaMode:   types.StringNull(),
	})
	idmap := idmapObject(t, ctx, builtin, idmapDomain)

	var ad ActiveDirectoryConfigModel
	if diags := m.ConfigurationActiveDirectory.As(ctx, &ad, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("unexpected error extracting AD config: %v", diags)
	}
	ad.Idmap = idmap
	m.ConfigurationActiveDirectory = adConfigObject(t, ctx, ad)

	p, diags := m.updatePayload(ctx, nil)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	gotConfig := p["configuration"].(map[string]any)
	gotIdmap, ok := gotConfig["idmap"].(map[string]any)
	if !ok {
		t.Fatalf(`configuration["idmap"] = %T, want map[string]any`, gotConfig["idmap"])
	}
	gotBuiltin, ok := gotIdmap["builtin"].(map[string]any)
	if !ok || gotBuiltin["range_low"] != int64(90000001) || gotBuiltin["range_high"] != int64(100000000) {
		t.Errorf(`idmap["builtin"] = %v, want range_low=90000001 range_high=100000000`, gotIdmap["builtin"])
	}
	gotDomain, ok := gotIdmap["idmap_domain"].(map[string]any)
	if !ok || gotDomain["idmap_backend"] != "RID" || gotDomain["name"] != "TFTEST" {
		t.Errorf(`idmap["idmap_domain"] = %v, want idmap_backend=RID name=TFTEST`, gotIdmap["idmap_domain"])
	}
}

// TestUpdatePayload_AD_IdmapOmittedWhenUnset verifies that a null/unset
// idmap block is omitted entirely from the outgoing configuration payload
// (back-compat: the existing AD acceptance test config, which never sets
// idmap, must remain valid — see enabledModel/validADConfig, which leave
// Idmap null).
func TestUpdatePayload_AD_IdmapOmittedWhenUnset(t *testing.T) {
	ctx := context.Background()
	m := enabledModel(t, ctx)

	p, diags := m.updatePayload(ctx, nil)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	gotConfig := p["configuration"].(map[string]any)
	if _, present := gotConfig["idmap"]; present {
		t.Error(`configuration["idmap"] should be omitted when the idmap block is unset`)
	}
}

// TestUpdatePayload_AD_IdmapDomainRIDOmitsADOnlyFields verifies that a RID
// idmap_domain payload never includes unix_primary_group/unix_nss_info
// (AD-only fields), even when the model holds known (non-null) Bool values
// for them — the state their read-back path always populates (see
// adIdmapToModel: it decodes them unconditionally via types.BoolValue,
// never types.BoolNull, regardless of the actual backend). Confirmed live
// (TrueNAS 25.10, Task 4 of this plan): sending them alongside
// idmap_backend="RID" fails with "[EINVAL] directoryservices_update.
// configuration.ACTIVEDIRECTORY.idmap.idmap_domain.RID.unix_nss_info: Extra
// inputs are not permitted" (and the same for unix_primary_group) — this
// broke the AD test's own in-place timeout-bump step (Step 2), which
// resends the idmap block read back from Step 1's join.
func TestUpdatePayload_AD_IdmapDomainRIDOmitsADOnlyFields(t *testing.T) {
	ctx := context.Background()
	m := enabledModel(t, ctx)

	idmapDomain := idmapDomainObject(t, ctx, IdmapDomainModel{
		IdmapBackend:     types.StringValue("RID"),
		Name:             types.StringValue("TFTEST"),
		RangeLow:         types.Int64Value(201000001),
		RangeHigh:        types.Int64Value(202000000),
		SSSDCompat:       types.BoolValue(false),
		SchemaMode:       types.StringNull(),
		UnixPrimaryGroup: types.BoolValue(false), // known, non-null — as read back from a prior RID join
		UnixNSSInfo:      types.BoolValue(false),
	})
	idmap := idmapObject(t, ctx, types.ObjectNull(idmapBuiltinAttrTypes), idmapDomain)

	var ad ActiveDirectoryConfigModel
	if diags := m.ConfigurationActiveDirectory.As(ctx, &ad, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("unexpected error extracting AD config: %v", diags)
	}
	ad.Idmap = idmap
	m.ConfigurationActiveDirectory = adConfigObject(t, ctx, ad)

	p, diags := m.updatePayload(ctx, nil)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	gotConfig := p["configuration"].(map[string]any)
	gotIdmap := gotConfig["idmap"].(map[string]any)
	gotDomain := gotIdmap["idmap_domain"].(map[string]any)
	if _, present := gotDomain["unix_primary_group"]; present {
		t.Errorf(`idmap_domain (RID) payload = %v, want "unix_primary_group" omitted`, gotDomain)
	}
	if _, present := gotDomain["unix_nss_info"]; present {
		t.Errorf(`idmap_domain (RID) payload = %v, want "unix_nss_info" omitted`, gotDomain)
	}
	if _, present := gotDomain["schema_mode"]; present {
		t.Errorf(`idmap_domain (RID) payload = %v, want "schema_mode" omitted`, gotDomain)
	}
	if v, ok := gotDomain["sssd_compat"]; !ok || v != false {
		t.Errorf(`idmap_domain (RID) payload["sssd_compat"] = %v, want false present`, gotDomain["sssd_compat"])
	}
}

// TestUpdatePayload_AD_IdmapDomainADOmitsSSSDCompat verifies the converse of
// TestUpdatePayload_AD_IdmapDomainRIDOmitsADOnlyFields: an AD idmap_domain
// payload includes schema_mode/unix_primary_group/unix_nss_info but never
// sssd_compat (RID-only).
func TestUpdatePayload_AD_IdmapDomainADOmitsSSSDCompat(t *testing.T) {
	ctx := context.Background()
	m := enabledModel(t, ctx)

	idmapDomain := idmapDomainObject(t, ctx, IdmapDomainModel{
		IdmapBackend:     types.StringValue("AD"),
		SchemaMode:       types.StringValue("RFC2307"),
		UnixPrimaryGroup: types.BoolValue(true),
		UnixNSSInfo:      types.BoolValue(true),
		SSSDCompat:       types.BoolValue(false), // known, non-null — must still be omitted for AD
	})
	idmap := idmapObject(t, ctx, types.ObjectNull(idmapBuiltinAttrTypes), idmapDomain)

	var ad ActiveDirectoryConfigModel
	if diags := m.ConfigurationActiveDirectory.As(ctx, &ad, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("unexpected error extracting AD config: %v", diags)
	}
	ad.Idmap = idmap
	m.ConfigurationActiveDirectory = adConfigObject(t, ctx, ad)

	p, diags := m.updatePayload(ctx, nil)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	gotConfig := p["configuration"].(map[string]any)
	gotIdmap := gotConfig["idmap"].(map[string]any)
	gotDomain := gotIdmap["idmap_domain"].(map[string]any)
	if _, present := gotDomain["sssd_compat"]; present {
		t.Errorf(`idmap_domain (AD) payload = %v, want "sssd_compat" omitted`, gotDomain)
	}
	if v, ok := gotDomain["schema_mode"]; !ok || v != "RFC2307" {
		t.Errorf(`idmap_domain (AD) payload["schema_mode"] = %v, want "RFC2307" present`, gotDomain["schema_mode"])
	}
	if v, ok := gotDomain["unix_primary_group"]; !ok || v != true {
		t.Errorf(`idmap_domain (AD) payload["unix_primary_group"] = %v, want true present`, gotDomain["unix_primary_group"])
	}
	if v, ok := gotDomain["unix_nss_info"]; !ok || v != true {
		t.Errorf(`idmap_domain (AD) payload["unix_nss_info"] = %v, want true present`, gotDomain["unix_nss_info"])
	}
}

// TestUpdatePayload_AD_IdmapDomainADRequiresSchemaMode verifies the
// idmap_domain preflight: idmap_backend "AD" requires schema_mode.
func TestUpdatePayload_AD_IdmapDomainADRequiresSchemaMode(t *testing.T) {
	ctx := context.Background()
	m := enabledModel(t, ctx)

	idmapDomain := idmapDomainObject(t, ctx, IdmapDomainModel{
		IdmapBackend: types.StringValue("AD"),
		SchemaMode:   types.StringNull(), // missing -> should error
	})
	idmap := idmapObject(t, ctx, types.ObjectNull(idmapBuiltinAttrTypes), idmapDomain)

	var ad ActiveDirectoryConfigModel
	if diags := m.ConfigurationActiveDirectory.As(ctx, &ad, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("unexpected error extracting AD config: %v", diags)
	}
	ad.Idmap = idmap
	m.ConfigurationActiveDirectory = adConfigObject(t, ctx, ad)

	_, diags := m.updatePayload(ctx, nil)
	if !diags.HasError() {
		t.Fatal("expected an error when idmap_backend is AD but schema_mode is unset")
	}
}

// TestUpdatePayload_AD_IdmapDomainRIDRejectsADOnlyFields verifies the
// preflight added for finding #1: a user-set AD-only idmap_domain field
// (schema_mode/unix_primary_group/unix_nss_info) under idmap_backend="RID"
// is rejected with a diagnostic before any API call, rather than being
// silently dropped by buildADConfigPayload's backend gating (which would
// otherwise let the join succeed and only surface as a
// "Provider produced inconsistent result after apply" error once read-back
// decodes the dropped field back to false).
func TestUpdatePayload_AD_IdmapDomainRIDRejectsADOnlyFields(t *testing.T) {
	tests := []struct {
		name   string
		domain IdmapDomainModel
	}{
		{
			name: "schema_mode",
			domain: IdmapDomainModel{
				IdmapBackend: types.StringValue("RID"),
				SchemaMode:   types.StringValue("RFC2307"),
			},
		},
		{
			name: "unix_primary_group",
			domain: IdmapDomainModel{
				IdmapBackend:     types.StringValue("RID"),
				SchemaMode:       types.StringNull(),
				UnixPrimaryGroup: types.BoolValue(true),
			},
		},
		{
			name: "unix_nss_info",
			domain: IdmapDomainModel{
				IdmapBackend: types.StringValue("RID"),
				SchemaMode:   types.StringNull(),
				UnixNSSInfo:  types.BoolValue(true),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			m := enabledModel(t, ctx)

			idmapDomain := idmapDomainObject(t, ctx, tt.domain)
			idmap := idmapObject(t, ctx, types.ObjectNull(idmapBuiltinAttrTypes), idmapDomain)

			var ad ActiveDirectoryConfigModel
			if diags := m.ConfigurationActiveDirectory.As(ctx, &ad, basetypes.ObjectAsOptions{}); diags.HasError() {
				t.Fatalf("unexpected error extracting AD config: %v", diags)
			}
			ad.Idmap = idmap
			m.ConfigurationActiveDirectory = adConfigObject(t, ctx, ad)

			_, diags := m.updatePayload(ctx, nil)
			if !diags.HasError() {
				t.Fatalf("expected an error when %q is set under idmap_backend=RID", tt.name)
			}
		})
	}
}

// TestUpdatePayload_AD_IdmapDomainADRejectsSSSDCompat verifies the converse
// preflight added for finding #1: a user-set sssd_compat under
// idmap_backend="AD" is rejected before any API call (sssd_compat is
// RID-only).
func TestUpdatePayload_AD_IdmapDomainADRejectsSSSDCompat(t *testing.T) {
	ctx := context.Background()
	m := enabledModel(t, ctx)

	idmapDomain := idmapDomainObject(t, ctx, IdmapDomainModel{
		IdmapBackend: types.StringValue("AD"),
		SchemaMode:   types.StringValue("RFC2307"),
		SSSDCompat:   types.BoolValue(true),
	})
	idmap := idmapObject(t, ctx, types.ObjectNull(idmapBuiltinAttrTypes), idmapDomain)

	var ad ActiveDirectoryConfigModel
	if diags := m.ConfigurationActiveDirectory.As(ctx, &ad, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("unexpected error extracting AD config: %v", diags)
	}
	ad.Idmap = idmap
	m.ConfigurationActiveDirectory = adConfigObject(t, ctx, ad)

	_, diags := m.updatePayload(ctx, nil)
	if !diags.HasError() {
		t.Fatal("expected an error when sssd_compat is set under idmap_backend=AD")
	}
}

// TestUpdatePayload_AD_IdmapDomainUnknownBackendOmitsBackendSpecificFields
// covers finding #2: an idmap_domain read back from an out-of-band
// LDAP/RFC2307 config (idmap_backend values this provider doesn't model as
// a full variant — see idmapDomainAttrTypes's doc comment) carries forward
// into the plan via UseStateForUnknown. buildADConfigPayload must not gain
// sssd_compat (previously sent unconditionally for "any non-AD backend",
// which live LDAP/RFC2307 boxes reject with "Extra inputs are not
// permitted") nor the AD-only fields for such a backend; only the fields
// common to every idmap_domain variant (idmap_backend/name/range_low/
// range_high) may be sent.
func TestUpdatePayload_AD_IdmapDomainUnknownBackendOmitsBackendSpecificFields(t *testing.T) {
	ctx := context.Background()
	m := enabledModel(t, ctx)

	idmapDomain := idmapDomainObject(t, ctx, IdmapDomainModel{
		IdmapBackend:     types.StringValue("LDAP"),
		Name:             types.StringValue("TFTEST"),
		RangeLow:         types.Int64Value(100000001),
		RangeHigh:        types.Int64Value(200000000),
		SchemaMode:       types.StringNull(),
		UnixPrimaryGroup: types.BoolValue(false), // known, non-null — carried forward from state
		UnixNSSInfo:      types.BoolValue(false),
		SSSDCompat:       types.BoolValue(false),
	})
	idmap := idmapObject(t, ctx, types.ObjectNull(idmapBuiltinAttrTypes), idmapDomain)

	var ad ActiveDirectoryConfigModel
	if diags := m.ConfigurationActiveDirectory.As(ctx, &ad, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("unexpected error extracting AD config: %v", diags)
	}
	ad.Idmap = idmap
	m.ConfigurationActiveDirectory = adConfigObject(t, ctx, ad)

	p, diags := m.updatePayload(ctx, nil)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	gotConfig := p["configuration"].(map[string]any)
	gotIdmap := gotConfig["idmap"].(map[string]any)
	gotDomain := gotIdmap["idmap_domain"].(map[string]any)

	for _, key := range []string{"sssd_compat", "schema_mode", "unix_primary_group", "unix_nss_info"} {
		if _, present := gotDomain[key]; present {
			t.Errorf(`idmap_domain (LDAP) payload = %v, want %q omitted`, gotDomain, key)
		}
	}
	if gotDomain["idmap_backend"] != "LDAP" || gotDomain["name"] != "TFTEST" {
		t.Errorf(`idmap_domain (LDAP) payload = %v, want idmap_backend=LDAP name=TFTEST`, gotDomain)
	}
}

// TestResponseToModel_LDAPConfig verifies the mapper fills
// ConfigurationLDAP (and leaves AD/IPA null) when service_type is LDAP,
// including search_bases/attribute_maps sub-objects.
func TestResponseToModel_LDAPConfig(t *testing.T) {
	ctx := context.Background()
	serviceType := "LDAP"
	api := &directoryServicesAPI{
		ID:          1,
		ServiceType: &serviceType,
		Enable:      true,
		Timeout:     10,
		Configuration: &directoryServicesConfigAPI{
			BaseDN:               "dc=tftest,dc=lan",
			ValidateCertificates: true,
			ServerURLs:           []string{"ldaps://ldap.tftest.lan"},
			StartTLS:             false,
			Schema:               "RFC2307",
			SearchBases: &directoryServicesLDAPSearchBasesAPI{
				BaseUser: strPtr("ou=people,dc=tftest,dc=lan"),
			},
			AttributeMaps: &directoryServicesLDAPAttributeMapsAPI{
				Passwd: &directoryServicesLDAPAttrMapPasswdAPI{UserObjectClass: strPtr("posixAccount")},
			},
		},
	}

	m := &DirectoryServicesModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !m.ConfigurationActiveDirectory.IsNull() {
		t.Error("ConfigurationActiveDirectory should be null when service_type is LDAP")
	}
	if !m.ConfigurationIPA.IsNull() {
		t.Error("ConfigurationIPA should be null when service_type is LDAP")
	}
	if m.ConfigurationLDAP.IsNull() {
		t.Fatal("ConfigurationLDAP should not be null when service_type is LDAP")
	}

	var l LDAPConfigModel
	diags = m.ConfigurationLDAP.As(ctx, &l, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("unexpected error extracting LDAP config: %v", diags)
	}
	if l.BaseDN.ValueString() != "dc=tftest,dc=lan" {
		t.Errorf("BaseDN = %q, want dc=tftest,dc=lan", l.BaseDN.ValueString())
	}
	var urls []string
	diags = l.ServerURLs.ElementsAs(ctx, &urls, false)
	if diags.HasError() || len(urls) != 1 || urls[0] != "ldaps://ldap.tftest.lan" {
		t.Errorf("ServerURLs = %v, want [ldaps://ldap.tftest.lan]", urls)
	}

	var sb LDAPSearchBasesModel
	diags = l.SearchBases.As(ctx, &sb, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("unexpected error extracting search_bases: %v", diags)
	}
	if sb.BaseUser.ValueString() != "ou=people,dc=tftest,dc=lan" {
		t.Errorf("SearchBases.BaseUser = %q, want ou=people,dc=tftest,dc=lan", sb.BaseUser.ValueString())
	}

	var am LDAPAttributeMapsModel
	diags = l.AttributeMaps.As(ctx, &am, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("unexpected error extracting attribute_maps: %v", diags)
	}
	var passwd LDAPAttrMapPasswdModel
	diags = am.Passwd.As(ctx, &passwd, basetypes.ObjectAsOptions{})
	if diags.HasError() || passwd.UserObjectClass.ValueString() != "posixAccount" {
		t.Errorf("AttributeMaps.Passwd.UserObjectClass = %q, want posixAccount", passwd.UserObjectClass.ValueString())
	}
}

// TestResponseToModel_IPAConfig verifies the mapper fills ConfigurationIPA
// (and leaves AD/LDAP null) when service_type is IPA.
func TestResponseToModel_IPAConfig(t *testing.T) {
	ctx := context.Background()
	serviceType := "IPA"
	api := &directoryServicesAPI{
		ID:          1,
		ServiceType: &serviceType,
		Enable:      true,
		Timeout:     10,
		Configuration: &directoryServicesConfigAPI{
			Hostname:             "tn2510",
			Domain:               "tfipa.lan",
			BaseDN:               "dc=tfipa,dc=lan",
			TargetServer:         "ipa.tfipa.lan",
			ValidateCertificates: true,
		},
	}

	m := &DirectoryServicesModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !m.ConfigurationActiveDirectory.IsNull() || !m.ConfigurationLDAP.IsNull() {
		t.Error("ConfigurationActiveDirectory/ConfigurationLDAP should be null when service_type is IPA")
	}
	if m.ConfigurationIPA.IsNull() {
		t.Fatal("ConfigurationIPA should not be null when service_type is IPA")
	}

	var ipa IPAConfigModel
	diags = m.ConfigurationIPA.As(ctx, &ipa, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("unexpected error extracting IPA config: %v", diags)
	}
	if ipa.TargetServer.ValueString() != "ipa.tfipa.lan" {
		t.Errorf("TargetServer = %q, want ipa.tfipa.lan", ipa.TargetServer.ValueString())
	}
	if !ipa.SMBDomain.IsNull() {
		t.Error("SMBDomain should be null when the API response omits it")
	}
}

// TestResponseToModel_ADIdmapReadBack verifies the mapper decodes
// configuration.idmap into the new explicit ActiveDirectoryConfigModel.Idmap
// object, replacing the old verbatim-JSON round-trip.
func TestResponseToModel_ADIdmapReadBack(t *testing.T) {
	ctx := context.Background()
	serviceType := "ACTIVEDIRECTORY"
	api := &directoryServicesAPI{
		ID:          1,
		ServiceType: &serviceType,
		Enable:      true,
		Timeout:     10,
		Configuration: &directoryServicesConfigAPI{
			Hostname: "tn2510",
			Domain:   "TFTEST.LAN",
			Idmap: &directoryServicesADIdmapAPI{
				Builtin: &directoryServicesIdmapRangeAPI{RangeLow: 90000001, RangeHigh: 100000000},
				IdmapDomain: &directoryServicesIdmapDomainAPI{
					IdmapBackend: "RID",
					Name:         strPtr("TFTEST"),
					RangeLow:     100000001,
					RangeHigh:    200000000,
					SSSDCompat:   false,
				},
			},
		},
	}

	m := &DirectoryServicesModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	var ad ActiveDirectoryConfigModel
	diags = m.ConfigurationActiveDirectory.As(ctx, &ad, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("unexpected error extracting AD config: %v", diags)
	}
	if ad.Idmap.IsNull() {
		t.Fatal("Idmap should not be null when the API response includes it")
	}

	var idm IdmapModel
	diags = ad.Idmap.As(ctx, &idm, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("unexpected error extracting idmap: %v", diags)
	}
	var builtin IdmapBuiltinModel
	diags = idm.Builtin.As(ctx, &builtin, basetypes.ObjectAsOptions{})
	if diags.HasError() || builtin.RangeLow.ValueInt64() != 90000001 || builtin.RangeHigh.ValueInt64() != 100000000 {
		t.Errorf("idmap.builtin = %+v, want range_low=90000001 range_high=100000000", builtin)
	}
	var domain IdmapDomainModel
	diags = idm.IdmapDomain.As(ctx, &domain, basetypes.ObjectAsOptions{})
	if diags.HasError() || domain.IdmapBackend.ValueString() != "RID" || domain.Name.ValueString() != "TFTEST" {
		t.Errorf("idmap.idmap_domain = %+v, want idmap_backend=RID name=TFTEST", domain)
	}
}

// strPtr returns a pointer to s, for building API structs with nullable
// string fields inline in test literals.
func strPtr(s string) *string { return &s }

// TestNeedsServiceTypeReset covers the predicate that gates
// DirectoryServicesResource.resetStaleServiceType (resource.go) — see that
// method's doc comment for the confirmed-live middleware defect this
// exists to work around (switching service_type away from a previous
// ACTIVEDIRECTORY/IPA join leaves a stale kerberos_realm/credential in
// place unless this singleton is fully cleared first).
func TestNeedsServiceTypeReset(t *testing.T) {
	planLDAP := &DirectoryServicesModel{ServiceType: types.StringValue("LDAP")}

	t.Run("nil existing", func(t *testing.T) {
		if needsServiceTypeReset(planLDAP, nil) {
			t.Error("want false when existing is nil (fresh box, never configured)")
		}
	})

	t.Run("existing never configured (nil service_type)", func(t *testing.T) {
		existing := &directoryServicesAPI{ServiceType: nil}
		if needsServiceTypeReset(planLDAP, existing) {
			t.Error("want false when existing.ServiceType is nil")
		}
	})

	t.Run("existing empty service_type", func(t *testing.T) {
		existing := &directoryServicesAPI{ServiceType: strPtr("")}
		if needsServiceTypeReset(planLDAP, existing) {
			t.Error("want false when existing.ServiceType is an empty string")
		}
	})

	t.Run("plan service_type unknown", func(t *testing.T) {
		plan := &DirectoryServicesModel{ServiceType: types.StringUnknown()}
		existing := &directoryServicesAPI{ServiceType: strPtr("ACTIVEDIRECTORY")}
		if needsServiceTypeReset(plan, existing) {
			t.Error("want false when plan.ServiceType is unknown")
		}
	})

	t.Run("plan service_type null", func(t *testing.T) {
		plan := &DirectoryServicesModel{ServiceType: types.StringNull()}
		existing := &directoryServicesAPI{ServiceType: strPtr("ACTIVEDIRECTORY")}
		if needsServiceTypeReset(plan, existing) {
			t.Error("want false when plan.ServiceType is null")
		}
	})

	t.Run("switching AD to LDAP needs reset", func(t *testing.T) {
		existing := &directoryServicesAPI{ServiceType: strPtr("ACTIVEDIRECTORY")}
		if !needsServiceTypeReset(planLDAP, existing) {
			t.Error("want true when existing.ServiceType (ACTIVEDIRECTORY) differs from plan.ServiceType (LDAP)")
		}
	})

	t.Run("switching IPA to LDAP needs reset", func(t *testing.T) {
		existing := &directoryServicesAPI{ServiceType: strPtr("IPA")}
		if !needsServiceTypeReset(planLDAP, existing) {
			t.Error("want true when existing.ServiceType (IPA) differs from plan.ServiceType (LDAP)")
		}
	})

	t.Run("same service_type does not need reset", func(t *testing.T) {
		existing := &directoryServicesAPI{ServiceType: strPtr("LDAP")}
		if needsServiceTypeReset(planLDAP, existing) {
			t.Error("want false when existing.ServiceType already matches plan.ServiceType")
		}
	})
}

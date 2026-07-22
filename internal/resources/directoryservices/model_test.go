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
	}
}

// validCredential builds a minimal, valid KERBEROS_USER credential object,
// for tests exercising the enable=true payload shape (which requires one).
func validCredential(t *testing.T, ctx context.Context) types.Object {
	t.Helper()
	cred, diags := types.ObjectValueFrom(ctx, credentialAttrTypes, CredentialModel{
		CredentialType: types.StringValue("KERBEROS_USER"),
		Username:       types.StringValue("Administrator"),
		Password:       types.StringValue("hunter2"),
	})
	if diags.HasError() {
		t.Fatalf("unexpected error building credential object: %v", diags)
	}
	return cred
}

// validADConfig builds a minimal, valid configuration_activedirectory
// object, for tests exercising the enable=true payload shape (which
// requires one).
func validADConfig(t *testing.T, ctx context.Context) types.Object {
	t.Helper()
	ad, diags := types.ObjectValueFrom(ctx, adConfigAttrTypes, ActiveDirectoryConfigModel{
		Hostname:             types.StringValue("tn2510"),
		Domain:               types.StringValue("TFTEST.LAN"),
		Site:                 types.StringNull(),
		ComputerAccountOU:    types.StringNull(),
		UseDefaultDomain:     types.BoolValue(false),
		EnableTrustedDomains: types.BoolValue(false),
	})
	if diags.HasError() {
		t.Fatalf("unexpected error building AD config object: %v", diags)
	}
	return ad
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
// found live against a SCALE 25.10 box: TrueNAS rejects
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
// for a bug found live against a SCALE 25.10 box: TrueNAS swaps a raw
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
	ad, diags := types.ObjectValueFrom(ctx, adConfigAttrTypes, ActiveDirectoryConfigModel{
		Hostname:             types.StringValue("tn2510"),
		Domain:               types.StringValue("TFTEST.LAN"),
		Site:                 types.StringValue(""), // explicit clear -> nil
		ComputerAccountOU:    types.StringValue("TRUENAS_SERVERS"),
		UseDefaultDomain:     types.BoolValue(true),
		EnableTrustedDomains: types.BoolValue(false),
	})
	if diags.HasError() {
		t.Fatalf("unexpected error building AD config object: %v", diags)
	}

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

// TestUpdatePayload_ExistingIdmapRoundTrip verifies that idmap/
// trusted_domains from a previously-fetched directoryServicesAPI are
// copied verbatim into the outgoing "configuration" map — required so an
// in-place update (e.g. bumping "timeout") on an already-joined resource
// doesn't look like an idmap/trusted_domains change to TrueNAS (neither is
// modeled in Terraform state; see the "existing" doc comment on
// updatePayload). Regression coverage for a live bug: the nested
// "configuration" object returned by the API does NOT echo back its own
// "service_type" key, so the round-trip guard must key off the TOP-LEVEL
// ServiceType (here, not the nested Configuration.ServiceType, which is
// always "" from a real response).
func TestUpdatePayload_ExistingIdmapRoundTrip(t *testing.T) {
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
			Idmap:          json.RawMessage(`{"idmap_domain":{"idmap_backend":"RID","name":"TFTEST"}}`),
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
	idmap, ok := got["idmap"].(map[string]any)
	if !ok {
		t.Fatalf("configuration[\"idmap\"] = %T, want map[string]any", got["idmap"])
	}
	idmapDomain, ok := idmap["idmap_domain"].(map[string]any)
	if !ok || idmapDomain["name"] != "TFTEST" {
		t.Errorf("configuration[\"idmap\"][\"idmap_domain\"] = %v, want name=TFTEST", idmap["idmap_domain"])
	}
	trustedDomains, ok := got["trusted_domains"].([]any)
	if !ok || len(trustedDomains) != 0 {
		t.Errorf("configuration[\"trusted_domains\"] = %v, want empty array", got["trusted_domains"])
	}
}

// TestUpdatePayload_ExistingIdmapIgnoredForOtherServiceType verifies that
// existing.Idmap/TrustedDomains are only round-tripped when the existing
// configuration's own service_type is ACTIVEDIRECTORY (defensive: the
// discriminated union has different shapes for IPA/LDAP, which this
// provider does not support in v1).
func TestUpdatePayload_ExistingIdmapIgnoredForOtherServiceType(t *testing.T) {
	ctx := context.Background()
	m := enabledModel(t, ctx)

	serviceType := "LDAP"
	existing := &directoryServicesAPI{
		ServiceType: &serviceType,
		Configuration: &directoryServicesConfigAPI{
			Idmap: json.RawMessage(`{"should":"not appear"}`),
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
	if _, present := got["idmap"]; present {
		t.Error("idmap should not be round-tripped when existing.ServiceType != ACTIVEDIRECTORY")
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

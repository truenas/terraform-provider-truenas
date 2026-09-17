// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acme_dns_authenticator

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestAttributesMap_RequiresAuthenticatorKey verifies attributesMap rejects
// JSON missing the "authenticator" discriminator key.
func TestAttributesMap_RequiresAuthenticatorKey(t *testing.T) {
	m := &AcmeDnsAuthenticatorModel{
		Attributes: types.StringValue(`{"api_token":"x"}`),
	}
	_, diags := m.attributesMap()
	if !diags.HasError() {
		t.Fatal("expected an error for attributes JSON with no \"authenticator\" key")
	}
}

// TestAttributesMap_InvalidJSON verifies attributesMap rejects malformed JSON.
func TestAttributesMap_InvalidJSON(t *testing.T) {
	m := &AcmeDnsAuthenticatorModel{
		Attributes: types.StringValue(`not json`),
	}
	_, diags := m.attributesMap()
	if !diags.HasError() {
		t.Fatal("expected an error for malformed attributes JSON")
	}
}

// TestCreatePayload_Cloudflare verifies createPayload against the shape
// probed live from a real acme.dns.authenticator.create call.
func TestCreatePayload_Cloudflare(t *testing.T) {
	m := &AcmeDnsAuthenticatorModel{
		Name:       types.StringValue("tf-acc-cf"),
		Attributes: types.StringValue(`{"authenticator":"cloudflare","api_token":"definitely-not-a-real-token"}`),
	}

	p, diags := m.createPayload()
	if diags.HasError() {
		t.Fatalf("createPayload returned diagnostics errors: %v", diags)
	}
	if p["name"] != "tf-acc-cf" {
		t.Errorf("payload[name] = %v, want tf-acc-cf", p["name"])
	}
	attrs, ok := p["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("payload[attributes] is %T, want map[string]any", p["attributes"])
	}
	if attrs["authenticator"] != "cloudflare" {
		t.Errorf("attrs[authenticator] = %v, want cloudflare", attrs["authenticator"])
	}
	if attrs["api_token"] != "definitely-not-a-real-token" {
		t.Errorf("attrs[api_token] = %v, want definitely-not-a-real-token", attrs["api_token"])
	}
}

// TestUpdatePayload_SameShapeAsCreate verifies updatePayload builds the
// same {name, attributes} shape as createPayload — probed live:
// acme.dns.authenticator.update accepts a full replace, not a partial
// patch.
func TestUpdatePayload_SameShapeAsCreate(t *testing.T) {
	m := &AcmeDnsAuthenticatorModel{
		Name:       types.StringValue("tf-acc-cf-renamed"),
		Attributes: types.StringValue(`{"authenticator":"route53","access_key_id":"AKIA","secret_access_key":"shh"}`),
	}

	p, diags := m.updatePayload()
	if diags.HasError() {
		t.Fatalf("updatePayload returned diagnostics errors: %v", diags)
	}
	if p["name"] != "tf-acc-cf-renamed" {
		t.Errorf("payload[name] = %v, want tf-acc-cf-renamed", p["name"])
	}
	attrs, ok := p["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("payload[attributes] is %T, want map[string]any", p["attributes"])
	}
	if attrs["authenticator"] != "route53" {
		t.Errorf("attrs[authenticator] = %v, want route53", attrs["authenticator"])
	}
}

// TestResponseToModel_ProbedShape verifies responseToModel against the
// exact shape observed from a live acme.dns.authenticator.create call
// (cloudflare variant, dummy api_token — credentials returned unmasked).
func TestResponseToModel_ProbedShape(t *testing.T) {
	api := &acmeDnsAuthenticatorAPI{
		ID:   1,
		Name: "tf-probe-acme-dns-auth-cf",
		Attributes: map[string]any{
			"authenticator":    "cloudflare",
			"cloudflare_email": nil,
			"api_key":          nil,
			"api_token":        "definitely-not-a-real-token-0000000000000",
		},
	}

	m := &AcmeDnsAuthenticatorModel{}
	responseToModel(api, m)

	if m.ID.ValueInt64() != 1 {
		t.Errorf("ID = %v, want 1", m.ID)
	}
	if m.Name.ValueString() != "tf-probe-acme-dns-auth-cf" {
		t.Errorf("Name = %q, want tf-probe-acme-dns-auth-cf", m.Name.ValueString())
	}
}

// TestAttributesDrifted_UserKeyChanged verifies attributesDrifted detects a
// changed user-set key.
func TestAttributesDrifted_UserKeyChanged(t *testing.T) {
	state := map[string]any{"authenticator": "cloudflare", "api_token": "old"}
	apiResp := map[string]any{"authenticator": "cloudflare", "api_token": "new", "api_key": nil}
	if !attributesDrifted(state, apiResp) {
		t.Error("expected drift when a user-set key's value changed")
	}
}

// TestAttributesDrifted_APIAddedKeyIgnored verifies attributesDrifted does
// not flag keys the API adds that weren't in the user's original state.
func TestAttributesDrifted_APIAddedKeyIgnored(t *testing.T) {
	state := map[string]any{"authenticator": "cloudflare", "api_token": "same"}
	apiResp := map[string]any{"authenticator": "cloudflare", "api_token": "same", "cloudflare_email": nil, "api_key": nil}
	if attributesDrifted(state, apiResp) {
		t.Error("did not expect drift when only API-added keys differ")
	}
}

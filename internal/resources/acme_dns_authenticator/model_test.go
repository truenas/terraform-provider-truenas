// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acme_dns_authenticator

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

// TestAttributesMap_RequiresAuthenticatorKey verifies attributesMap rejects
// JSON missing the "authenticator" discriminator key.
func TestAttributesMap_RequiresAuthenticatorKey(t *testing.T) {
	m := &AcmeDnsAuthenticatorModel{
		Attributes: normalize.NewJSONSubsetValue(`{"api_token":"x"}`),
	}
	_, diags := m.attributesMap()
	if !diags.HasError() {
		t.Fatal("expected an error for attributes JSON with no \"authenticator\" key")
	}
}

// TestAttributesMap_InvalidJSON verifies attributesMap rejects malformed JSON.
func TestAttributesMap_InvalidJSON(t *testing.T) {
	m := &AcmeDnsAuthenticatorModel{
		Attributes: normalize.NewJSONSubsetValue(`not json`),
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
		Attributes: normalize.NewJSONSubsetValue(`{"authenticator":"cloudflare","api_token":"definitely-not-a-real-token"}`),
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
		Attributes: normalize.NewJSONSubsetValue(`{"authenticator":"route53","access_key_id":"AKIA","secret_access_key":"shh"}`),
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
		Attributes: apiAttributes{M: map[string]any{
			"authenticator":    "cloudflare",
			"cloudflare_email": nil,
			"api_key":          nil,
			"api_token":        "definitely-not-a-real-token-0000000000000",
		}},
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

func readEqualsPrior(t *testing.T, prior string, api apiAttributes, wo bool) bool {
	t.Helper()
	v, d := readAttributes(api, prior, wo)
	if d.HasError() {
		t.Fatal(d)
	}
	eq, d := v.StringSemanticEquals(context.Background(), normalize.NewJSONSubsetValue(prior))
	if d.HasError() {
		t.Fatal(d)
	}
	return eq
}

// Server-added null keys and a heredoc spelling are not drift; a changed
// configured value is (B-061, B-120).
func TestReadAttributes_Subset(t *testing.T) {
	heredoc := "{\n  \"authenticator\": \"cloudflare\",\n  \"api_token\": \"same\"\n}\n"
	same := apiAttributes{M: map[string]any{"authenticator": "cloudflare", "api_token": "same", "cloudflare_email": nil, "api_key": nil}}
	changed := apiAttributes{M: map[string]any{"authenticator": "cloudflare", "api_token": "new"}}
	if !readEqualsPrior(t, heredoc, same, false) {
		t.Error("server-added keys should not be drift")
	}
	if !readEqualsPrior(t, heredoc, same, true) {
		t.Error("write-only projection of the same values should not be drift")
	}
	if readEqualsPrior(t, heredoc, changed, false) {
		t.Error("a changed configured value should be drift")
	}
}

// A read-only session gets the whole attributes object as "********" (B-031):
// the decode must not fail, Read keeps the prior value, the data source
// reports null.
func TestRedactedAttributes(t *testing.T) {
	var api acmeDnsAuthenticatorAPI
	if err := json.Unmarshal([]byte(`{"id": 3, "name": "x", "attributes": "********"}`), &api); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !api.Attributes.Redacted {
		t.Fatal("Redacted not set")
	}
	prior := `{"authenticator":"cloudflare","api_token":"t"}`
	v, d := readAttributes(api.Attributes, prior, false)
	if d.HasError() || v.ValueString() != prior {
		t.Errorf("readAttributes = %q (%v), want the prior value", v.ValueString(), d)
	}
	var ds AcmeDnsAuthenticatorDataSourceModel
	if d := responseToDataSourceModel(&api, &ds); d.HasError() || !ds.Attributes.IsNull() {
		t.Errorf("data source attributes = %v (%v), want null", ds.Attributes, d)
	}
	// A single masked key keeps its prior value.
	got := unmaskRedacted(map[string]any{"api_token": "********", "authenticator": "cloudflare"}, prior)
	if got["api_token"] != "t" {
		t.Errorf("api_token = %v, want the prior value", got["api_token"])
	}
}

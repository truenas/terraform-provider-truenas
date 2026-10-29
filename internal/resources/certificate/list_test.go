// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package certificate

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestListMetadata(t *testing.T) {
	l := &CertificateListResource{}
	req := resource.MetadataRequest{ProviderTypeName: "truenas"}
	resp := &resource.MetadataResponse{}
	l.Metadata(context.Background(), req, resp)
	if resp.TypeName != "truenas_certificate" {
		t.Errorf("TypeName = %q, want %q", resp.TypeName, "truenas_certificate")
	}
}

func TestCertificateIdentitySchema(t *testing.T) {
	r := &CertificateResource{}
	resp := &resource.IdentitySchemaResponse{}
	r.IdentitySchema(context.Background(), resource.IdentitySchemaRequest{}, resp)
	if _, ok := resp.IdentitySchema.Attributes["id"]; !ok {
		t.Error("identity schema missing 'id' attribute")
	}
}

// TestListCreateType: generated configuration must never carry a null
// create_type (Required); a CSR lists as CERTIFICATE_CREATE_CSR (B-138).
func TestListCreateType(t *testing.T) {
	uri := "https://acme.example/dir"
	cases := []struct {
		api  certificateAPI
		want string
	}{
		{certificateAPI{CertTypeCSR: true}, CreateTypeCSR},
		{certificateAPI{AcmeURI: &uri}, CreateTypeACME},
		{certificateAPI{}, CreateTypeImported},
	}
	for _, c := range cases {
		if got := listCreateType(&c.api); got.ValueString() != c.want {
			t.Errorf("%+v: got %v, want %s", c.api, got, c.want)
		}
	}
}

// TestSanPayload: bare IPs are sent "IP:"-prefixed (25.10 mangles a bare IPv6).
func TestSanPayload(t *testing.T) {
	got := sanPayload([]string{"a.example.com", "10.0.0.1", "2001:db8::7", "IP:10.0.0.2", "DNS:b.example.com"})
	want := []string{"a.example.com", "IP:10.0.0.1", "IP:2001:db8::7", "IP:10.0.0.2", "DNS:b.example.com"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %q, want %q", i, got[i], want[i])
		}
	}
}

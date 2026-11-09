// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_power

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestMetadataAndSchema(t *testing.T) {
	r := &VMPowerResource{}
	var md resource.MetadataResponse
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "truenas"}, &md)
	if md.TypeName != "truenas_vm_power" {
		t.Errorf("TypeName = %q", md.TypeName)
	}
	var sr resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &sr)
	if d := sr.Schema.ValidateImplementation(context.Background()); d.HasError() {
		t.Fatal(d)
	}
	for _, req := range []string{"vm_id", "running"} {
		if !sr.Schema.Attributes[req].IsRequired() {
			t.Errorf("%s must be required", req)
		}
	}
	for _, opt := range []string{"overcommit", "restart_triggers"} {
		if !sr.Schema.Attributes[opt].IsOptional() || sr.Schema.Attributes[opt].IsComputed() {
			t.Errorf("%s must be optional and not computed (unset = default, no state-only drift)", opt)
		}
	}
	var ir resource.IdentitySchemaResponse
	r.IdentitySchema(context.Background(), resource.IdentitySchemaRequest{}, &ir)
	if _, ok := ir.IdentitySchema.Attributes["id"]; !ok {
		t.Error("identity schema missing id")
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container_device

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
)

func TestSchema_IDComputedUseStateForUnknown(t *testing.T) {
	s := resourceSchema()

	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	idInt64, ok := idAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.Int64Attribute", idAttr)
	}
	if !idInt64.IsComputed() {
		t.Error("'id' should be Computed")
	}
	if idInt64.IsRequired() || idInt64.IsOptional() {
		t.Error("'id' should be Computed-only (not Required/Optional)")
	}

	foundUseState := false
	for _, pm := range idInt64.PlanModifiers {
		if pm.Description(context.Background()) == int64planmodifier.UseStateForUnknown().Description(context.Background()) {
			foundUseState = true
		}
	}
	if !foundUseState {
		t.Error("'id' should have a UseStateForUnknown plan modifier")
	}
}

func TestSchema_ContainerRequiresReplace(t *testing.T) {
	s := resourceSchema()

	containerAttr, ok := s.Attributes["container"]
	if !ok {
		t.Fatal("schema missing 'container' attribute")
	}
	containerInt64, ok := containerAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'container' attribute is %T, want schema.Int64Attribute", containerAttr)
	}
	if !containerInt64.IsRequired() {
		t.Error("'container' should be Required")
	}

	foundReplace := false
	for _, pm := range containerInt64.PlanModifiers {
		if pm.Description(context.Background()) == int64planmodifier.RequiresReplace().Description(context.Background()) {
			foundReplace = true
		}
	}
	if !foundReplace {
		t.Error("'container' should have a RequiresReplace plan modifier")
	}
}

func TestSchema_AttributesRequired(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["attributes"]
	if !ok {
		t.Fatal("schema missing 'attributes' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'attributes' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsRequired() {
		t.Error("'attributes' should be Required")
	}
}

func TestSchema_NoUnexpectedAttributes(t *testing.T) {
	s := resourceSchema()
	want := map[string]bool{"id": true, "container": true, "attributes": true}
	if len(s.Attributes) != len(want) {
		t.Errorf("schema has %d attributes, want %d: %v", len(s.Attributes), len(want), attrNames(s))
	}
	for name := range s.Attributes {
		if !want[name] {
			t.Errorf("unexpected schema attribute %q", name)
		}
	}
}

func attrNames(s schema.Schema) []string {
	names := make([]string, 0, len(s.Attributes))
	for name := range s.Attributes {
		names = append(names, name)
	}
	return names
}

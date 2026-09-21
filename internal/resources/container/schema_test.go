// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func TestContainerSchema_IDIsComputed(t *testing.T) {
	s := resourceSchema()

	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	idInt, ok := idAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.Int64Attribute", idAttr)
	}
	if !idInt.IsComputed() {
		t.Error("'id' should be Computed")
	}
	if idInt.IsRequired() || idInt.IsOptional() {
		t.Error("'id' should be Computed-only (not Required/Optional)")
	}
}

func TestContainerSchema_NameIsRequiredAndMutable(t *testing.T) {
	// "name" must be Required but NOT carry a RequiresReplace plan
	// modifier: container.update accepts and applies a rename, confirmed
	// live (see model.go's updatePayload doc comment).
	s := resourceSchema()
	nameAttr, ok := s.Attributes["name"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("'name' attribute is %T, want schema.StringAttribute", s.Attributes["name"])
	}
	if !nameAttr.IsRequired() {
		t.Error("'name' should be Required")
	}
	if len(nameAttr.PlanModifiers) != 0 {
		t.Errorf("'name' should have no plan modifiers (mutable via container.update), got %d", len(nameAttr.PlanModifiers))
	}
}

func TestContainerSchema_PoolIsRequiredAndForceNew(t *testing.T) {
	s := resourceSchema()
	poolAttr, ok := s.Attributes["pool"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("'pool' attribute is %T, want schema.StringAttribute", s.Attributes["pool"])
	}
	if !poolAttr.IsRequired() {
		t.Error("'pool' should be Required")
	}
	if len(poolAttr.PlanModifiers) == 0 {
		t.Error("'pool' should carry a RequiresReplace plan modifier (container.update rejects it, probed live)")
	}
}

func TestContainerSchema_ImageIsRequiredAndForceNew(t *testing.T) {
	s := resourceSchema()
	imgAttr, ok := s.Attributes["image"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("'image' attribute is %T, want schema.SingleNestedAttribute", s.Attributes["image"])
	}
	if !imgAttr.IsRequired() {
		t.Error("'image' should be Required")
	}
	if len(imgAttr.PlanModifiers) == 0 {
		t.Error("'image' should carry a RequiresReplace plan modifier (container.update omits it, probed live)")
	}
	for _, sub := range []string{"name", "version"} {
		attr, ok := imgAttr.Attributes[sub]
		if !ok {
			t.Fatalf("'image' missing nested %q attribute", sub)
		}
		strAttr, ok := attr.(schema.StringAttribute)
		if !ok || !strAttr.IsRequired() {
			t.Errorf("'image.%s' should be a Required StringAttribute", sub)
		}
	}
}

func TestContainerSchema_IdmapIsForceNew(t *testing.T) {
	// idmap is container.update-immutable (probed: absent from its
	// accepted field list) so it must carry RequiresReplace even though
	// it's Optional+Computed (has a server default), unlike pool/image
	// which are Required.
	s := resourceSchema()
	idmapAttr, ok := s.Attributes["idmap"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("'idmap' attribute is %T, want schema.StringAttribute", s.Attributes["idmap"])
	}
	if !idmapAttr.IsOptional() || !idmapAttr.IsComputed() {
		t.Error("'idmap' should be Optional+Computed")
	}
	if len(idmapAttr.PlanModifiers) < 2 {
		t.Errorf("'idmap' should carry both UseStateForUnknown and RequiresReplace, got %d modifiers", len(idmapAttr.PlanModifiers))
	}
}

func TestContainerSchema_MutableFieldsAreOptionalComputedWithoutForceNew(t *testing.T) {
	// Every field container.update DOES accept (probed live) must be
	// Optional+Computed with exactly one plan modifier (UseStateForUnknown
	// only — no RequiresReplace).
	s := resourceSchema()
	for _, name := range []string{"description", "autostart", "cpuset", "time", "shutdown_timeout",
		"init", "initdir", "inituser", "initgroup", "capabilities_policy", "running"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		strAttr, ok := attr.(schema.StringAttribute)
		if ok {
			if !strAttr.IsOptional() || !strAttr.IsComputed() {
				t.Errorf("%q should be Optional+Computed", name)
			}
			if len(strAttr.PlanModifiers) != 1 {
				t.Errorf("%q should have exactly 1 plan modifier (UseStateForUnknown only), got %d", name, len(strAttr.PlanModifiers))
			}
			continue
		}
		// autostart/running are BoolAttribute, shutdown_timeout is Int64Attribute.
		switch a := attr.(type) {
		case schema.BoolAttribute:
			if !a.IsOptional() || !a.IsComputed() {
				t.Errorf("%q should be Optional+Computed", name)
			}
		case schema.Int64Attribute:
			if !a.IsOptional() || !a.IsComputed() {
				t.Errorf("%q should be Optional+Computed", name)
			}
		default:
			t.Errorf("%q has unexpected type %T", name, attr)
		}
	}
}

func TestContainerSchema_ComputedOnlyFields(t *testing.T) {
	s := resourceSchema()
	for _, name := range []string{"uuid", "dataset", "default_network", "status"} {
		attr, ok := s.Attributes[name].(schema.StringAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.StringAttribute", name, s.Attributes[name])
		}
		if !attr.IsComputed() {
			t.Errorf("%q should be Computed", name)
		}
		if attr.IsRequired() || attr.IsOptional() {
			t.Errorf("%q should be Computed-only", name)
		}
	}
}

func TestContainerSchema_NoUnexpectedAttributes(t *testing.T) {
	s := resourceSchema()
	want := map[string]bool{
		"id": true, "uuid": true, "name": true, "pool": true, "image": true,
		"description": true, "autostart": true, "cpuset": true, "time": true,
		"shutdown_timeout": true, "init": true, "initdir": true, "initenv": true,
		"inituser": true, "initgroup": true, "idmap": true, "capabilities_policy": true,
		"capabilities_state": true, "running": true, "dataset": true,
		"default_network": true, "status": true,
	}
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

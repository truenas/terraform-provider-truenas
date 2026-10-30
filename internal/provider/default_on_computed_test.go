// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package provider_test

// Guards the import-rewrite class (hunt findings F3/F4/F9, 2026-10-30): a
// schema Default on an Optional+Computed attribute makes the plan write the
// default whenever the configuration is silent, so an object imported with a
// minimal configuration — or state from an older provider — is rewritten on
// the first apply. Use planmod.ResetWhenUnset* (+ planmod.RecordManaged in
// Create) instead. Any attribute listed here must say why a plain Default is
// right for it (the value is never server-owned, e.g. a pure provider flag).

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	fwschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/provider"
)

// defaultOnComputedJustified: "type.attr[.nested]" -> reason.
var defaultOnComputedJustified = map[string]string{
	// syslogservers elements are authored wholesale in config; transport only
	// applies to elements the user writes, never to an imported value.
	"truenas_system_advanced.syslogservers.transport": "default inside a user-authored list element",
	// provider-only flags: the server has no such value, so nothing can be
	// rewritten; the only effect is a one-shot no-op diff after import (F6).
	"truenas_dataset.force_destroy": "provider-only flag, never read from the server",
	"truenas_zvol.force_destroy":    "provider-only flag, never read from the server",
	// create-time directives the API does not return; Update never sends them.
	"truenas_certificate.add_to_trusted_store": "create-time directive, not read back, never sent on update",
	"truenas_certificate.store_privatekey":     "create-time directive, not read back, never sent on update",
}

func TestNoDefaultOnOptionalComputed(t *testing.T) {
	ctx := context.Background()
	p := provider.New("test")()
	pp := p.(interface {
		Resources(context.Context) []func() resource.Resource
	})
	var offenders []string
	seen := map[string]bool{}
	for _, ctor := range pp.Resources(ctx) {
		res := ctor()
		var md resource.MetadataResponse
		res.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "truenas"}, &md)
		var sr resource.SchemaResponse
		res.Schema(ctx, resource.SchemaRequest{}, &sr)
		walkAttrs(md.TypeName, sr.Schema.Attributes, func(path string, a fwschema.Attribute) {
			if !a.IsOptional() || !a.IsComputed() || !hasDefault(a) {
				return
			}
			seen[path] = true
			if reason, ok := defaultOnComputedJustified[path]; !ok || strings.TrimSpace(reason) == "" {
				offenders = append(offenders, path)
			}
		})
	}
	sort.Strings(offenders)
	for _, o := range offenders {
		t.Errorf("%s: Default on an Optional+Computed attribute rewrites imported/upgraded objects (F4 class); "+
			"use planmod.ResetWhenUnset* + planmod.RecordManaged, or justify it in defaultOnComputedJustified", o)
	}
	for path := range defaultOnComputedJustified {
		if !seen[path] {
			t.Errorf("%s is justified but no longer has a Default on an Optional+Computed attribute; drop the entry", path)
		}
	}
}

type withDefault interface{ GetDefault() any }

func hasDefault(a fwschema.Attribute) bool {
	// every concrete schema attribute type exposes its Default via a
	// typed getter; reflect through the small set of interfaces.
	switch v := a.(type) {
	case fwschema.StringAttribute:
		return v.Default != nil
	case fwschema.BoolAttribute:
		return v.Default != nil
	case fwschema.Int64Attribute:
		return v.Default != nil
	case fwschema.Float64Attribute:
		return v.Default != nil
	case fwschema.NumberAttribute:
		return v.Default != nil
	case fwschema.ListAttribute:
		return v.Default != nil
	case fwschema.SetAttribute:
		return v.Default != nil
	case fwschema.MapAttribute:
		return v.Default != nil
	case fwschema.ObjectAttribute:
		return v.Default != nil
	case fwschema.SingleNestedAttribute:
		return v.Default != nil
	case fwschema.ListNestedAttribute:
		return v.Default != nil
	case fwschema.SetNestedAttribute:
		return v.Default != nil
	case fwschema.MapNestedAttribute:
		return v.Default != nil
	}
	return false
}

func walkAttrs(prefix string, attrs map[string]fwschema.Attribute, fn func(path string, a fwschema.Attribute)) {
	for name, a := range attrs {
		path := prefix + "." + name
		fn(path, a)
		switch v := a.(type) {
		case fwschema.SingleNestedAttribute:
			walkAttrs(path, v.Attributes, fn)
		case fwschema.ListNestedAttribute:
			walkAttrs(path, v.NestedObject.Attributes, fn)
		case fwschema.SetNestedAttribute:
			walkAttrs(path, v.NestedObject.Attributes, fn)
		case fwschema.MapNestedAttribute:
			walkAttrs(path, v.NestedObject.Attributes, fn)
		}
	}
}

var _ withDefault

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

import (
	"context"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/types"
	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/truenas/terraform-provider-truenas/internal/acctest"
	"github.com/truenas/terraform-provider-truenas/internal/planmod"
	"github.com/truenas/terraform-provider-truenas/internal/provider"
)

// schemaDefaults returns attr -> concrete default value for every top-level
// Bool/Int64/String attribute of typ whose Default is set. A Default that
// evaluates to null is omitted (there is nothing to compare). Int64 defaults
// come back as int64, bools as bool, strings as string.
func schemaDefaults(t *testing.T, typ string) map[string]any {
	t.Helper()
	ctx := context.Background()
	p := provider.New("test")()
	for _, ctor := range providerResources(ctx, p) {
		r := ctor()
		var md resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "truenas"}, &md)
		if md.TypeName != typ {
			continue
		}
		var sr resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &sr)
		out := map[string]any{}
		for name, attr := range sr.Schema.Attributes {
			switch a := attr.(type) {
			case schema.BoolAttribute:
				if a.Default == nil {
					for _, m := range a.PlanModifiers {
						if v, ok := planmod.ResetWhenUnsetDefault(m); ok {
							out[name] = v.(types.Bool).ValueBool()
						}
					}
					continue
				}
				var resp defaults.BoolResponse
				a.Default.DefaultBool(ctx, defaults.BoolRequest{}, &resp)
				if !resp.PlanValue.IsNull() && !resp.PlanValue.IsUnknown() {
					out[name] = resp.PlanValue.ValueBool()
				}
			case schema.Int64Attribute:
				if a.Default == nil {
					for _, m := range a.PlanModifiers {
						if v, ok := planmod.ResetWhenUnsetDefault(m); ok {
							out[name] = v.(types.Int64).ValueInt64()
						}
					}
					continue
				}
				var resp defaults.Int64Response
				a.Default.DefaultInt64(ctx, defaults.Int64Request{}, &resp)
				if !resp.PlanValue.IsNull() && !resp.PlanValue.IsUnknown() {
					out[name] = resp.PlanValue.ValueInt64()
				}
			case schema.StringAttribute:
				if a.Default == nil {
					for _, m := range a.PlanModifiers {
						if v, ok := planmod.ResetWhenUnsetDefault(m); ok {
							out[name] = v.(types.String).ValueString()
						}
					}
					continue
				}
				var resp defaults.StringResponse
				a.Default.DefaultString(ctx, defaults.StringRequest{}, &resp)
				if !resp.PlanValue.IsNull() && !resp.PlanValue.IsUnknown() {
					out[name] = resp.PlanValue.ValueString()
				}
			}
		}
		return out
	}
	t.Fatalf("resource type %s not registered", typ)
	return nil
}

// serverPathOverride maps "<type>.<attr>" to a dotted path in the server
// instance when the field is not top-level. keychaincredential stores its
// connection parameters under a nested "attributes" object, which the provider
// flattens into top-level Terraform attributes.
var serverPathOverride = map[string]string{
	"truenas_keychain_ssh_connection.port":            "attributes.port",
	"truenas_keychain_ssh_connection.username":        "attributes.username",
	"truenas_keychain_ssh_connection.connect_timeout": "attributes.connect_timeout",
}

// apiPath maps a schema attribute to its path in the server instance.
func apiPath(typ, attr string) string {
	if p, ok := serverPathOverride[typ+"."+attr]; ok {
		return p
	}
	if a, ok := apiFieldAlias[typ+"."+attr]; ok {
		return a
	}
	return attr
}

// serverComparable normalizes a schema default for comparison with decoded
// JSON: GetInstance yields numbers as float64, so int64 defaults are converted
// here (the audit, not AssertServerValue, owns this).
func serverComparable(v any) any {
	if n, ok := v.(int64); ok {
		return float64(n)
	}
	return v
}

// serverUnverifiableDefaults lists "<type>.<attr>" defaults that have no
// persisted server property, so the audit must not look for them on the
// server. Only keys listed here are skipped; a missing server field for any
// other default is still a failure.
var serverUnverifiableDefaults = map[string]string{
	"truenas_certificate.store_privatekey": "create-only behavior flag; no corresponding server property",
}

func TestSchemaDefaults_Values(t *testing.T) {
	ssh := schemaDefaults(t, "truenas_keychain_ssh_connection")
	want := map[string]any{"port": int64(22), "connect_timeout": int64(10), "username": "root"}
	for k, v := range want {
		if !reflect.DeepEqual(ssh[k], v) {
			t.Errorf("ssh_connection %s = %#v, want %#v", k, ssh[k], v)
		}
	}
	if got := schemaDefaults(t, "truenas_certificate")["store_privatekey"]; got != true {
		t.Errorf("certificate store_privatekey = %#v, want true", got)
	}
	if got := schemaDefaults(t, "truenas_replication_task")["transport"]; got != "LOCAL" {
		t.Errorf("replication_task transport = %#v, want LOCAL", got)
	}
}

// TestUnsetDefaults_Audit applies each wired resource with required attributes
// only and asserts (a) the create succeeds (#53) and (b) every schema Default
// is materialized on the server (#56). Unwired types must be in
// unsetDefaultsRatchet.
func TestUnsetDefaults_Audit(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Set TF_ACC=1 (and TRUENAS_* env) to run the live unset-defaults audit")
	}
	wired := map[string]serverTruthResource{}
	for _, r := range serverTruthResources() {
		wired[r.typ] = r
	}
	singletons := map[string]bool{}
	for _, r := range singletonResources() {
		singletons[r.typ] = true
	}
	types := allProviderResourceTypes(t)
	sort.Strings(types)
	for _, typ := range types {
		typ := typ
		if singletons[typ] {
			t.Run(typ, func(t *testing.T) {
				t.Skip("config singleton: not created, so no unset-defaults check applies (see TestSingletonUnrelatedUpdate_Audit)")
			})
			continue
		}
		entry, ok := wired[typ]
		if !ok {
			reason, ratcheted := unsetDefaultsRatchet[typ]
			if !ratcheted {
				t.Errorf("%s is neither wired in serverTruthResources() nor in unsetDefaultsRatchet", typ)
				continue
			}
			t.Run(typ, func(t *testing.T) { t.Skip(reason) })
			continue
		}
		t.Run(typ, func(t *testing.T) {
			if entry.minMajor > 0 && !acctest.ServerVersionAtLeast(t, entry.minMajor, entry.minMinor) {
				t.Skipf("%s requires TrueNAS %d.%d+", typ, entry.minMajor, entry.minMinor)
			}
			defs := schemaDefaults(t, typ)
			name := acctest.RandName("tfud")
			tfresource.Test(t, tfresource.TestCase{
				PreCheck:                 func() { acctest.PreCheck(t) },
				ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
				Steps: []tfresource.TestStep{{
					Config: acctest.ProviderConfig() + entry.minimalConfig(name),
					Check: func(s *terraform.State) error {
						inst := entry.fetch(t, s)
						for attr, want := range defs {
							if _, skip := serverUnverifiableDefaults[typ+"."+attr]; skip {
								continue
							}
							acctest.AssertServerValue(t, inst, apiPath(typ, attr), serverComparable(want))
						}
						return nil
					},
				}},
			})
		})
	}
}

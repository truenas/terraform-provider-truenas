// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/provider"
)

// TestEnumConformance_Audit reports, for every resource, where our schema's
// allowed string values diverge from the value domain the TrueNAS API actually
// advertises (core.get_methods `accepts` enum). It is the systematic answer to
// "we aren't testing all the API's values": for each enum value the API accepts
// on a field, it runs our schema's own validators for the matching attribute and
// flags the value if we reject it — so missing enum members (e.g. INHERIT on
// sync/checksum/acltype, the ZSTD ladder on compression) surface before a user
// hits them. It also flags attributes the API constrains to an enum where our
// schema has no validator at all (unconstrained — invalid values reach the API).
//
// Enforcing: it fails if any divergence remains. TF_ACC + TRUENAS_*
// required (it reads the live method schemas). Run:
//
//	TF_ACC=1 TRUENAS_ENDPOINT=... TRUENAS_API_KEY=... \
//	  go test ./internal/acctest -run TestEnumConformance_Audit -v
func TestEnumConformance_Audit(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Set TF_ACC=1 (and TRUENAS_* env) to run the live enum-conformance audit")
	}
	root := repoRoot(t)
	methods := getMethods(t)

	ctx := context.Background()
	p := provider.New("test")()

	var rejects, unconstrained []string

	for _, ctor := range providerResources(ctx, p) {
		r := ctor()
		var md fwresource.MetadataResponse
		r.Metadata(ctx, fwresource.MetadataRequest{ProviderTypeName: "truenas"}, &md)
		var sc fwresource.SchemaResponse
		r.Schema(ctx, fwresource.SchemaRequest{}, &sc)

		ns := resourceMethodNamespace(t, root, md.TypeName)
		if ns == "" {
			continue
		}
		// Prefer the create method's accepts; fall back to update (config singletons).
		accepts, ok := methods[ns+".create"]
		if !ok {
			accepts, ok = methods[ns+".update"]
		}
		if !ok {
			continue
		}
		apiEnums := acceptsEnums(accepts) // field -> sorted enum values

		for name, attr := range sc.Schema.Attributes {
			if name == "id" || !isWritable(attr) {
				continue
			}
			apiField := name
			if a, ok := apiFieldAlias[md.TypeName+"."+name]; ok {
				apiField = a
			}
			enum, has := apiEnums[apiField]
			if !has || len(enum) == 0 {
				continue
			}
			sa, isStr := attr.(rschema.StringAttribute)
			if !isStr {
				continue // only string enums handled in this pass
			}
			if len(sa.Validators) == 0 {
				unconstrained = append(unconstrained,
					fmt.Sprintf("%s.%s: API constrains to {%s} but the schema has no validator (invalid values reach the API)",
						md.TypeName, name, strings.Join(enum, ", ")))
				continue
			}
			var rejected []string
			for _, v := range enum {
				if !schemaAccepts(ctx, sa.Validators, v) {
					rejected = append(rejected, v)
				}
			}
			if len(rejected) > 0 {
				rejects = append(rejects,
					fmt.Sprintf("%s.%s: schema rejects API-valid value(s): %s", md.TypeName, name, strings.Join(rejected, ", ")))
			}
		}
	}

	sort.Strings(rejects)
	sort.Strings(unconstrained)
	var b strings.Builder
	fmt.Fprintf(&b, "\nEnum conformance vs TrueNAS API:\n")
	fmt.Fprintf(&b, "\n[%d] schema REJECTS values the API accepts:\n", len(rejects))
	for _, l := range rejects {
		fmt.Fprintf(&b, "  - %s\n", l)
	}
	fmt.Fprintf(&b, "\n[%d] API has an enum but schema is UNCONSTRAINED:\n", len(unconstrained))
	for _, l := range unconstrained {
		fmt.Fprintf(&b, "  - %s\n", l)
	}
	// Enforcing: any divergence from the API's value domain fails. A new
	// resource/enum that legitimately diverges (a value we deliberately do not
	// expose) must be recorded in an allowlist here with a reason, not left to
	// surface as a user-reported failure after release.
	if len(rejects)+len(unconstrained) > 0 {
		t.Errorf("%s\nReconcile the schema with the API (add the missing enum members / a OneOf validator), "+
			"or record a justified exception.", b.String())
	} else {
		t.Log(b.String())
	}
}

// schemaAccepts runs the attribute's string validators against a value and
// reports whether they accept it (no error diagnostics).
func schemaAccepts(ctx context.Context, vs []validator.String, value string) bool {
	req := validator.StringRequest{ConfigValue: types.StringValue(value)}
	for _, v := range vs {
		resp := &validator.StringResponse{}
		v.ValidateString(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			return false
		}
	}
	return true
}

// acceptsEnums returns, per top-level accepts field, the enum values the API
// advertises for it (recursing through oneOf/anyOf/allOf and picking up an enum
// declared directly on the field or on one of its union branches).
func acceptsEnums(accepts json.RawMessage) map[string][]string {
	out := map[string][]string{}
	if len(accepts) == 0 {
		return out
	}
	var params []map[string]any
	if json.Unmarshal(accepts, &params) != nil {
		return out
	}
	var walk func(s map[string]any)
	walk = func(s map[string]any) {
		if props, ok := s["properties"].(map[string]any); ok {
			for k, pv := range props {
				if pm, ok := pv.(map[string]any); ok {
					if e := extractEnum(pm); len(e) > 0 {
						out[k] = e
					}
				}
			}
		}
		for _, key := range []string{"oneOf", "anyOf", "allOf"} {
			if subs, ok := s[key].([]any); ok {
				for _, sub := range subs {
					if sm, ok := sub.(map[string]any); ok {
						walk(sm)
					}
				}
			}
		}
	}
	for _, pr := range params {
		walk(pr)
	}
	return out
}

// extractEnum pulls a string enum from a field schema: either `enum` directly,
// or the union of enums across its oneOf/anyOf branches.
func extractEnum(s map[string]any) []string {
	seen := map[string]bool{}
	var collect func(m map[string]any)
	collect = func(m map[string]any) {
		if e, ok := m["enum"].([]any); ok {
			for _, v := range e {
				if sv, ok := v.(string); ok {
					seen[sv] = true
				}
			}
		}
		for _, key := range []string{"oneOf", "anyOf", "allOf"} {
			if subs, ok := m[key].([]any); ok {
				for _, sub := range subs {
					if sm, ok := sub.(map[string]any); ok {
						collect(sm)
					}
				}
			}
		}
	}
	collect(s)
	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

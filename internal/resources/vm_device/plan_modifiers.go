// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_device

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// importedKey is the private-state key ImportState sets: attributes in state
// is then the full server document, not a configured value.
const importedKey = "imported"

// privateSetter is the SetKey method of a response's private state.
type privateSetter interface {
	SetKey(ctx context.Context, key string, value []byte) diag.Diagnostics
}

// setImported sets (ImportState) or clears (Create, Update) the imported flag.
func setImported(ctx context.Context, p privateSetter, imported bool) diag.Diagnostics {
	if imported {
		return p.SetKey(ctx, importedKey, []byte("true"))
	}
	return p.SetKey(ctx, importedKey, nil)
}

// keepAttributesIfConfigMatchesState keeps the prior state value for the
// attributes JSON when every key the configuration sets already has its
// configured value in state. After import the state holds every key the API
// reports (defaults included) while a configuration normally sets only a
// subset, so the first plan would otherwise propose rewriting attributes to
// that subset even though nothing on the device changes — mirroring the Read
// path, which already ignores keys the configuration omits (attributesDrifted). (#30)
//
// It applies only while the imported flag is set: once Terraform has written
// the value, state holds the configured document, and keeping it would hide
// the removal of a key from the configuration.
type keepAttributesIfConfigMatchesState struct{}

func (m keepAttributesIfConfigMatchesState) Description(_ context.Context) string {
	return "keeps the imported attributes when the configuration is a subset that already matches"
}

func (m keepAttributesIfConfigMatchesState) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m keepAttributesIfConfigMatchesState) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.StateValue.IsNull() || req.StateValue.IsUnknown() {
		return
	}
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if req.ConfigValue.Equal(req.StateValue) || req.Private == nil {
		return
	}
	flag, diags := req.Private.GetKey(ctx, importedKey)
	resp.Diagnostics.Append(diags...)
	if len(flag) == 0 {
		return
	}
	var stateAttrs, configAttrs map[string]any
	if json.Unmarshal([]byte(req.StateValue.ValueString()), &stateAttrs) != nil {
		return
	}
	if json.Unmarshal([]byte(req.ConfigValue.ValueString()), &configAttrs) != nil {
		return
	}
	// attributesDrifted(configAttrs, stateAttrs) is true when a configured key
	// is missing from, or differs in, state. When it is false every configured
	// key already matches, so keep the (fuller) state value.
	if !attributesDrifted(configAttrs, stateAttrs) {
		resp.PlanValue = req.StateValue
	}
}

// validateNoSecretAttributes rejects an attributes document that contains a
// secret key (see secretAttributeKeys): attributes is shown in plans and kept
// in state, so the secret must go in attributes_secrets_wo. (B-159)
type validateNoSecretAttributes struct{}

func (v validateNoSecretAttributes) Description(_ context.Context) string {
	return "attributes must not contain a secret key such as password"
}

func (v validateNoSecretAttributes) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v validateNoSecretAttributes) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	var attrs map[string]any
	if json.Unmarshal([]byte(req.ConfigValue.ValueString()), &attrs) != nil {
		return // attributesMap reports invalid JSON at apply time
	}
	for _, k := range secretAttributeKeys {
		if _, ok := attrs[k]; ok {
			resp.Diagnostics.AddAttributeError(req.Path, "Secret in attributes",
				fmt.Sprintf("attributes must not contain %q: attributes is shown in plans and stored in state. "+
					"Move it to the write-only attributes_secrets_wo, e.g. attributes_secrets_wo = jsonencode({ %s = ... }), "+
					"and set attributes_secrets_wo_version (bump it whenever the secret changes).", k, k))
		}
	}
}

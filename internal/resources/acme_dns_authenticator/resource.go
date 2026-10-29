// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acme_dns_authenticator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
	"github.com/truenas/terraform-provider-truenas/internal/writeonly"
)

// injectAttributesSecrets merges the write-only attributes_secrets_wo overlay
// (read from config) over the payload's attributes map; never stored in state.
func injectAttributesSecrets(payload map[string]any, cfg *AcmeDnsAuthenticatorModel, diags *diag.Diagnostics) {
	if cfg.AttributesSecretsWO.IsNull() || cfg.AttributesSecretsWO.IsUnknown() || cfg.AttributesSecretsWO.ValueString() == "" {
		return
	}
	base, _ := payload["attributes"].(map[string]any)
	merged, err := writeonly.MergeOverlay(base, cfg.AttributesSecretsWO.ValueString())
	if err != nil {
		diags.AddError("Invalid attributes_secrets_wo", "attributes_secrets_wo is not a valid JSON object")
		return
	}
	payload["attributes"] = merged
}

var _ resource.Resource = &AcmeDnsAuthenticatorResource{}
var _ resource.ResourceWithImportState = &AcmeDnsAuthenticatorResource{}
var _ resource.ResourceWithIdentity = &AcmeDnsAuthenticatorResource{}

// AcmeDnsAuthenticatorResource implements the truenas_acme_dns_authenticator resource.
type AcmeDnsAuthenticatorResource struct{ client *client.Client }

// NewResource returns a new instance of AcmeDnsAuthenticatorResource.
func NewResource() resource.Resource { return &AcmeDnsAuthenticatorResource{} }

func (r *AcmeDnsAuthenticatorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_acme_dns_authenticator"
}

func (r *AcmeDnsAuthenticatorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *AcmeDnsAuthenticatorResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.IntIDIdentitySchema()
}

func (r *AcmeDnsAuthenticatorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

// acme.dns.authenticator.create/update/delete/get_instance/query are all
// job:false (probed live via core.get_methods, confirmed identical on
// TrueNAS 25.10/26.0) — plain synchronous calls, no CallJob needed.

func (r *AcmeDnsAuthenticatorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AcmeDnsAuthenticatorModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.createPayload()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var cfg AcmeDnsAuthenticatorModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	injectAttributesSecrets(payload, &cfg, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "acme.dns.authenticator.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create ACME DNS authenticator failed", err.Error())
		return
	}

	var apiResp acmeDnsAuthenticatorAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	// Keep the plan's attributes JSON string in state (write-what-you-said):
	// the API may echo back a normalized/expanded attributes document.
	responseToModel(&apiResp, &plan)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(setImported(ctx, resp.Private, false)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AcmeDnsAuthenticatorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AcmeDnsAuthenticatorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "acme.dns.authenticator.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read ACME DNS authenticator failed", err.Error())
		return
	}

	var apiResp acmeDnsAuthenticatorAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	responseToModel(&apiResp, &state)

	attrs, diags := readAttributes(apiResp.Attributes, state.Attributes.ValueString(), state.usesWriteOnlyAttributes())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Attributes = attrs

	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *AcmeDnsAuthenticatorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AcmeDnsAuthenticatorModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state AcmeDnsAuthenticatorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	payload, diags := plan.updatePayload()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var cfg AcmeDnsAuthenticatorModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if plan.AttributesSecretsWOVersion.Equal(state.AttributesSecretsWOVersion) &&
		!cfg.AttributesSecretsWO.IsNull() && !cfg.AttributesSecretsWO.IsUnknown() && cfg.AttributesSecretsWO.ValueString() != "" {
		// Version unchanged: keep the stored secrets (see storedSecretsOverlay).
		rawStored, err := r.client.CallRead(ctx, "acme.dns.authenticator.get_instance", plan.ID.ValueInt64())
		if err != nil {
			resp.Diagnostics.AddError("Read stored secrets before update failed", err.Error())
			return
		}
		var stored acmeDnsAuthenticatorAPI
		if err := json.Unmarshal(rawStored, &stored); err != nil {
			resp.Diagnostics.AddError("Parse get_instance response", err.Error())
			return
		}
		cfg.AttributesSecretsWO = types.StringValue(storedSecretsOverlay(cfg.AttributesSecretsWO.ValueString(), stored.Attributes.M))
	}
	injectAttributesSecrets(payload, &cfg, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "acme.dns.authenticator.update", plan.ID.ValueInt64(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update ACME DNS authenticator failed", err.Error())
		return
	}

	var apiResp acmeDnsAuthenticatorAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	// Keep the plan's attributes JSON string in state (write-what-you-said).
	responseToModel(&apiResp, &plan)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(setImported(ctx, resp.Private, false)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AcmeDnsAuthenticatorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AcmeDnsAuthenticatorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "acme.dns.authenticator.delete", state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete ACME DNS authenticator failed", err.Error())
	}
}

func (r *AcmeDnsAuthenticatorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, idDiags := listing.ImportInt64(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "acme.dns.authenticator.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import ACME DNS authenticator failed", err.Error())
		return
	}

	var apiResp acmeDnsAuthenticatorAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	var state AcmeDnsAuthenticatorModel
	responseToModel(&apiResp, &state)
	if apiResp.Attributes.Redacted {
		resp.Diagnostics.AddError("ACME DNS authenticator attributes are redacted",
			"TrueNAS returned \"attributes\" as \"********\": the session lacks the write role needed to "+
				"read them, so the authenticator cannot be imported. Import with a full-admin API key.")
		return
	}
	attrs, diags := readAttributes(apiResp.Attributes, "", false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Attributes = attrs

	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, id)...)
	resp.Diagnostics.Append(setImported(ctx, resp.Private, true)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

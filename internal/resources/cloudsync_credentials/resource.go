// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync_credentials

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

// injectProviderSecrets merges the write-only provider_secrets_wo overlay (read
// from config) over the payload's provider object; never stored in state.
func injectProviderSecrets(payload map[string]any, cfg *CredentialsModel, diags *diag.Diagnostics) {
	if cfg.ProviderSecretsWO.IsNull() || cfg.ProviderSecretsWO.IsUnknown() || cfg.ProviderSecretsWO.ValueString() == "" {
		return
	}
	base, _ := payload["provider"].(map[string]any)
	merged, err := writeonly.MergeOverlay(base, cfg.ProviderSecretsWO.ValueString())
	if err != nil {
		diags.AddError("Invalid provider_secrets_wo", "provider_secrets_wo is not a valid JSON object")
		return
	}
	payload["provider"] = merged
}

var _ resource.Resource = &CredentialsResource{}
var _ resource.ResourceWithImportState = &CredentialsResource{}
var _ resource.ResourceWithIdentity = &CredentialsResource{}

// CredentialsResource implements the truenas_cloudsync_credentials resource.
type CredentialsResource struct{ client *client.Client }

// NewResource returns a new CredentialsResource.
func NewResource() resource.Resource { return &CredentialsResource{} }

func (r *CredentialsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloudsync_credentials"
}

func (r *CredentialsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *CredentialsResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.IntIDIdentitySchema()
}

func (r *CredentialsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data",
			fmt.Sprintf("expected *client.Client, got %T", req.ProviderData),
		)
		return
	}
	r.client = c
}

func (r *CredentialsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CredentialsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.createPayload()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var cfg CredentialsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	injectProviderSecrets(payload, &cfg, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "cloudsync.credentials.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create cloudsync credentials failed", err.Error())
		return
	}

	// Parse only the ID from the create response, then read back via
	// get_instance to pick up server-assigned defaults.
	var created credentialsAPI
	if err := json.Unmarshal(raw, &created); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	raw, err = r.client.CallRead(ctx, "cloudsync.credentials.get_instance", created.ID)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	var apiResp credentialsAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse get_instance response", err.Error())
		return
	}

	// Keep the plan's provider_config JSON string in state
	// (write-what-you-said): the API may echo back a normalized/expanded
	// provider document.
	responseToModel(&apiResp, &plan)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(setImported(ctx, resp.Private, false)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CredentialsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state CredentialsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "cloudsync.credentials.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read cloudsync credentials failed", err.Error())
		return
	}

	var apiResp credentialsAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	responseToModel(&apiResp, &state)

	providerJSON, diags := readProvider(combinedProviderMap(&apiResp), state.Provider.ValueString(), state.usesWriteOnlyProvider())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Provider = types.StringValue(providerJSON)

	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *CredentialsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan CredentialsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Carry the existing state ID into the plan.
	var state CredentialsModel
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
	var cfg CredentialsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if plan.ProviderSecretsWOVersion.Equal(state.ProviderSecretsWOVersion) &&
		!cfg.ProviderSecretsWO.IsNull() && !cfg.ProviderSecretsWO.IsUnknown() && cfg.ProviderSecretsWO.ValueString() != "" {
		// Version unchanged: keep the stored secrets (see storedSecretsOverlay).
		rawStored, err := r.client.CallRead(ctx, "cloudsync.credentials.get_instance", plan.ID.ValueInt64())
		if err != nil {
			resp.Diagnostics.AddError("Read stored secrets before update failed", err.Error())
			return
		}
		var stored credentialsAPI
		if err := json.Unmarshal(rawStored, &stored); err != nil {
			resp.Diagnostics.AddError("Parse get_instance response", err.Error())
			return
		}
		cfg.ProviderSecretsWO = types.StringValue(storedSecretsOverlay(cfg.ProviderSecretsWO.ValueString(), stored.Provider))
	}
	injectProviderSecrets(payload, &cfg, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "cloudsync.credentials.update", plan.ID.ValueInt64(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update cloudsync credentials failed", err.Error())
		return
	}

	raw, err := r.client.CallRead(ctx, "cloudsync.credentials.get_instance", plan.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	var apiResp credentialsAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse get_instance response", err.Error())
		return
	}

	// Keep the plan's provider_config JSON string in state
	// (write-what-you-said).
	responseToModel(&apiResp, &plan)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(setImported(ctx, resp.Private, false)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CredentialsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state CredentialsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "cloudsync.credentials.delete", state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete cloudsync credentials failed", err.Error())
	}
}

func (r *CredentialsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, idDiags := listing.ImportInt64(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "cloudsync.credentials.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import cloudsync credentials failed", err.Error())
		return
	}

	var apiResp credentialsAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	var state CredentialsModel
	responseToModel(&apiResp, &state)
	providerJSON, diags := apiProviderJSON(&apiResp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Provider = types.StringValue(providerJSON)

	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, id)...)
	resp.Diagnostics.Append(setImported(ctx, resp.Private, true)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

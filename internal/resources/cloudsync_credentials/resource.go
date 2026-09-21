// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync_credentials

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &CredentialsResource{}
var _ resource.ResourceWithImportState = &CredentialsResource{}

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

	// Drift-aware provider_config handling: only overwrite the state string
	// when a user-set key's value differs in the API response. API-added
	// keys are not considered drift. If the state string is empty/null (as
	// on import), populate it directly from the API JSON.
	if state.Provider.IsNull() || state.Provider.ValueString() == "" {
		providerJSON, diags := apiProviderJSON(&apiResp)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.Provider = types.StringValue(providerJSON)
	} else {
		var stateProvider map[string]any
		if err := json.Unmarshal([]byte(state.Provider.ValueString()), &stateProvider); err != nil {
			resp.Diagnostics.AddError("Parse state provider_config JSON", err.Error())
			return
		}
		if providerDrifted(stateProvider, combinedProviderMap(&apiResp)) {
			providerJSON, diags := apiProviderJSON(&apiResp)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			state.Provider = types.StringValue(providerJSON)
		}
	}

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
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Import ID must be an integer", req.ID)
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

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

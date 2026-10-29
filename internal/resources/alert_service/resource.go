// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package alert_service

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

var _ resource.Resource = &AlertServiceResource{}
var _ resource.ResourceWithImportState = &AlertServiceResource{}
var _ resource.ResourceWithIdentity = &AlertServiceResource{}

// AlertServiceResource implements the truenas_alert_service resource.
type AlertServiceResource struct{ client *client.Client }

// NewResource returns a new AlertServiceResource.
func NewResource() resource.Resource { return &AlertServiceResource{} }

func (r *AlertServiceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alert_service"
}

func (r *AlertServiceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *AlertServiceResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.IntIDIdentitySchema()
}

func (r *AlertServiceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// injectAttributesSecrets merges the write-only attributes_secrets_wo overlay
// (read from config) over the payload's attributes map; never stored in state.
func injectAttributesSecrets(payload map[string]any, cfg *AlertServiceModel, diags *diag.Diagnostics) {
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

func (r *AlertServiceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AlertServiceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.createPayload()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var cfg AlertServiceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	injectAttributesSecrets(payload, &cfg, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "alertservice.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create alert service failed", err.Error())
		return
	}

	// Parse only the ID from the create response, then read back via
	// get_instance to pick up server-assigned defaults.
	var created alertServiceAPI
	if err := json.Unmarshal(raw, &created); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	raw, err = r.client.CallRead(ctx, "alertservice.get_instance", created.ID)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	var apiResp alertServiceAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse get_instance response", err.Error())
		return
	}

	// Keep the plan's attributes JSON string in state (write-what-you-said):
	// the API may echo back a normalized/expanded attributes document.
	responseToModel(&apiResp, &plan)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(setImported(ctx, resp.Private, false)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AlertServiceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AlertServiceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "alertservice.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read alert service failed", err.Error())
		return
	}

	var apiResp alertServiceAPI
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

func (r *AlertServiceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AlertServiceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Carry the existing state ID into the plan.
	var state AlertServiceModel
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
	var cfg AlertServiceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if plan.AttributesSecretsWOVersion.Equal(state.AttributesSecretsWOVersion) &&
		!cfg.AttributesSecretsWO.IsNull() && !cfg.AttributesSecretsWO.IsUnknown() && cfg.AttributesSecretsWO.ValueString() != "" {
		// Version unchanged: keep the stored secrets (see storedSecretsOverlay).
		rawStored, err := r.client.CallRead(ctx, "alertservice.get_instance", plan.ID.ValueInt64())
		if err != nil {
			resp.Diagnostics.AddError("Read stored secrets before update failed", err.Error())
			return
		}
		var stored alertServiceAPI
		if err := json.Unmarshal(rawStored, &stored); err != nil {
			resp.Diagnostics.AddError("Parse get_instance response", err.Error())
			return
		}
		cfg.AttributesSecretsWO = types.StringValue(storedSecretsOverlay(cfg.AttributesSecretsWO.ValueString(), stored.Attributes))
	}
	injectAttributesSecrets(payload, &cfg, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "alertservice.update", plan.ID.ValueInt64(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update alert service failed", err.Error())
		return
	}

	raw, err := r.client.CallRead(ctx, "alertservice.get_instance", plan.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	var apiResp alertServiceAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse get_instance response", err.Error())
		return
	}

	// Keep the plan's attributes JSON string in state (write-what-you-said).
	responseToModel(&apiResp, &plan)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(setImported(ctx, resp.Private, false)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AlertServiceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AlertServiceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "alertservice.delete", state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete alert service failed", err.Error())
	}
}

func (r *AlertServiceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, idDiags := listing.ImportInt64(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "alertservice.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import alert service failed", err.Error())
		return
	}

	var apiResp alertServiceAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	var state AlertServiceModel
	responseToModel(&apiResp, &state)
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

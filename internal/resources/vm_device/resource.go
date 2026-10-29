// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_device

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
// (read from config) over the payload's attributes map. The overlay is never
// stored in state. A parse failure is reported without echoing the value.
func injectAttributesSecrets(payload map[string]any, cfg *VMDeviceModel, diags *diag.Diagnostics) {
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

var _ resource.Resource = &VMDeviceResource{}
var _ resource.ResourceWithImportState = &VMDeviceResource{}
var _ resource.ResourceWithIdentity = &VMDeviceResource{}

// VMDeviceResource implements the truenas_vm_device resource.
type VMDeviceResource struct{ client *client.Client }

// NewResource returns a new VMDeviceResource.
func NewResource() resource.Resource { return &VMDeviceResource{} }

func (r *VMDeviceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm_device"
}

func (r *VMDeviceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *VMDeviceResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.IntIDIdentitySchema()
}

func (r *VMDeviceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *VMDeviceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan VMDeviceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.createPayload()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var cfg VMDeviceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	injectAttributesSecrets(payload, &cfg, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "vm.device.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create VM device failed", err.Error())
		return
	}

	// Parse only the ID from the create response, then read back via
	// get_instance to pick up server-assigned defaults (e.g. order).
	var created deviceAPI
	if err := json.Unmarshal(raw, &created); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	raw, err = r.client.CallRead(ctx, "vm.device.get_instance", created.ID)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	var apiResp deviceAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse get_instance response", err.Error())
		return
	}

	// Keep the plan's Attributes JSON string in state (write-what-you-said):
	// the API may normalize/add default keys.
	responseToModel(&apiResp, &plan)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(setImported(ctx, resp.Private, false)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *VMDeviceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state VMDeviceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "vm.device.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read VM device failed", err.Error())
		return
	}

	var apiResp deviceAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	responseToModel(&apiResp, &state)

	var stateAttrs map[string]any
	if err := json.Unmarshal([]byte(state.Attributes.ValueString()), &stateAttrs); err != nil {
		resp.Diagnostics.AddError("Parse state attributes JSON", err.Error())
		return
	}
	// Secrets never go into state (B-159). This also drops a password that an
	// older provider version stored in attributes.
	dropSecretAttributes(apiResp.Attributes)
	scrubRedacted(apiResp.Attributes, stateAttrs)

	if state.usesWriteOnlyAttributes() {
		// Write-only secrets overlay in use: reconcile attributes by projecting
		// the live device onto only the keys already in state, so the secret
		// keys (present only in the write-only overlay) are never read back.
		state.Attributes = types.StringValue(writeonly.ProjectOntoKeys(state.Attributes.ValueString(), apiResp.Attributes))
	} else {
		// Drift-aware attributes handling: only overwrite state Attributes when
		// a user-set key's value differs in the API. API-added default keys are
		// not considered drift, and are not absorbed into state on drift either:
		// state takes the server values of the keys it already holds, so the
		// plan shows only the keys that really changed. (BUGS B-083)
		if attributesDrifted(stateAttrs, apiResp.Attributes) {
			state.Attributes = types.StringValue(writeonly.ProjectOntoKeys(state.Attributes.ValueString(), apiResp.Attributes))
		}
	}

	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *VMDeviceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan VMDeviceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Carry the existing state ID into the plan.
	var state VMDeviceModel
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
	var cfg VMDeviceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// The write-only overlay is sent only when attributes_secrets_wo_version
	// changed; vm.device.update merges the attributes it gets into the stored
	// ones, so the stored password is kept when it is absent. (B-141)
	if !plan.AttributesSecretsWOVersion.Equal(state.AttributesSecretsWOVersion) {
		injectAttributesSecrets(payload, &cfg, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	_, err := r.client.Call(ctx, "vm.device.update", plan.ID.ValueInt64(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update VM device failed", err.Error())
		return
	}

	raw, err := r.client.CallRead(ctx, "vm.device.get_instance", plan.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	var apiResp deviceAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse get_instance response", err.Error())
		return
	}

	// Keep the plan's Attributes JSON string in state (write-what-you-said).
	responseToModel(&apiResp, &plan)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(setImported(ctx, resp.Private, false)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *VMDeviceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state VMDeviceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "vm.device.delete", state.ID.ValueInt64(), map[string]any{
		"force":    false,
		"raw_file": false,
		"zvol":     false,
	})
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete VM device failed", err.Error())
	}
}

func (r *VMDeviceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, idDiags := listing.ImportInt64(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "vm.device.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import VM device failed", err.Error())
		return
	}

	var apiResp deviceAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	var state VMDeviceModel
	responseToModel(&apiResp, &state)
	scrubRedacted(apiResp.Attributes, nil)
	attrJSON, diags := apiAttributesJSON(&apiResp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Attributes = types.StringValue(attrJSON)

	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, id)...)
	resp.Diagnostics.Append(setImported(ctx, resp.Private, true)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package keychain_ssh_connection

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &KeychainSSHConnectionResource{}
var _ resource.ResourceWithImportState = &KeychainSSHConnectionResource{}

// KeychainSSHConnectionResource implements the
// truenas_keychain_ssh_connection resource.
type KeychainSSHConnectionResource struct{ client *client.Client }

// NewResource returns a new instance of KeychainSSHConnectionResource.
func NewResource() resource.Resource { return &KeychainSSHConnectionResource{} }

func (r *KeychainSSHConnectionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_keychain_ssh_connection"
}

func (r *KeychainSSHConnectionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *KeychainSSHConnectionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// keychaincredential.create/update/delete/get_instance/query are all
// non-job (sync) methods (probed via core.get_methods: every one reports
// "job": false).

func (r *KeychainSSHConnectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan KeychainSSHConnectionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "keychaincredential.create", plan.createPayload())
	if err != nil {
		resp.Diagnostics.AddError("Create keychain SSH connection failed", err.Error())
		return
	}

	var apiResp keychainSSHConnectionAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(&apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *KeychainSSHConnectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state KeychainSSHConnectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "keychaincredential.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read keychain SSH connection failed", err.Error())
		return
	}

	var apiResp keychainSSHConnectionAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}
	if apiResp.Type != keychainCredentialType {
		resp.Diagnostics.AddError(
			"Unexpected keychain credential type",
			fmt.Sprintf("id %d is a %q keychain credential, not %q — it was not created by (and cannot be "+
				"managed as) truenas_keychain_ssh_connection", state.ID.ValueInt64(), apiResp.Type, keychainCredentialType),
		)
		return
	}

	resp.Diagnostics.Append(responseToModel(&apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *KeychainSSHConnectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan KeychainSSHConnectionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state KeychainSSHConnectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "keychaincredential.update", state.ID.ValueInt64(), plan.updatePayload())
	if err != nil {
		resp.Diagnostics.AddError("Update keychain SSH connection failed", err.Error())
		return
	}

	var apiResp keychainSSHConnectionAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(&apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *KeychainSSHConnectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state KeychainSSHConnectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "keychaincredential.delete", state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete keychain SSH connection failed", err.Error())
	}
}

func (r *KeychainSSHConnectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Import ID must be an integer", req.ID)
		return
	}

	raw, err := r.client.CallRead(ctx, "keychaincredential.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import keychain SSH connection failed", err.Error())
		return
	}

	var apiResp keychainSSHConnectionAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}
	if apiResp.Type != keychainCredentialType {
		resp.Diagnostics.AddError(
			"Unexpected keychain credential type",
			fmt.Sprintf("id %d is a %q keychain credential, not %q — it cannot be imported as "+
				"truenas_keychain_ssh_connection", id, apiResp.Type, keychainCredentialType),
		)
		return
	}

	var state KeychainSSHConnectionModel
	resp.Diagnostics.Append(responseToModel(&apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

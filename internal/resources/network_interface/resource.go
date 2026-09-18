// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package network_interface

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &NetworkInterfaceResource{}
var _ resource.ResourceWithImportState = &NetworkInterfaceResource{}

// stagingMu serializes the stage→commit→checkin lifecycle. TrueNAS stages
// interface changes globally, so concurrent applies of multiple
// truenas_network_interface resources would otherwise commit or roll back
// each other's pending changes.
var stagingMu sync.Mutex

// NetworkInterfaceResource implements the truenas_network_interface resource.
//
// TrueNAS stages interface changes; they do not take effect until committed.
// Every Create/Update/Delete therefore ends with commitAndCheckin: commit
// arms an auto-rollback timer, and checkin (reached over the same
// connection) proves connectivity survived and cancels the timer.
type NetworkInterfaceResource struct{ client *client.Client }

// NewResource returns a new NetworkInterfaceResource.
func NewResource() resource.Resource { return &NetworkInterfaceResource{} }

func (r *NetworkInterfaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network_interface"
}

func (r *NetworkInterfaceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *NetworkInterfaceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// commitAndCheckin applies staged interface changes and confirms them.
// rollback=true arms auto-revert: if the apply cuts connectivity, TrueNAS
// restores the old config after checkin_timeout seconds. Reaching checkin
// over the same WebSocket proves connectivity survived.
func (r *NetworkInterfaceResource) commitAndCheckin(ctx context.Context) error {
	if _, err := r.client.Call(ctx, "interface.commit", map[string]any{
		"checkin_timeout": 60,
		"rollback":        true,
	}); err != nil {
		// discard staged changes so the failed plan doesn't poison later applies
		if _, rbErr := r.client.Call(ctx, "interface.rollback"); rbErr != nil {
			return fmt.Errorf("commit failed: %w; rollback of staged changes also failed: %v", err, rbErr)
		}
		return err
	}
	if _, err := r.client.Call(ctx, "interface.checkin"); err != nil {
		return fmt.Errorf("commit applied but checkin failed (config will auto-rollback in 60s): %w", err)
	}
	return nil
}

// getInstance reads back an interface by name via interface.get_instance.
func (r *NetworkInterfaceResource) getInstance(ctx context.Context, name string) (*interfaceAPI, error) {
	raw, err := r.client.CallRead(ctx, "interface.get_instance", name)
	if err != nil {
		return nil, err
	}
	var api interfaceAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

func (r *NetworkInterfaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan NetworkInterfaceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := validateCreateType(plan.Type.ValueString()); err != nil {
		resp.Diagnostics.AddError("Cannot create interface", err.Error())
		return
	}

	payload, diags := plan.createPayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	stagingMu.Lock()
	defer stagingMu.Unlock()

	if _, err := r.client.Call(ctx, "interface.create", payload); err != nil {
		resp.Diagnostics.AddError("Create interface failed", err.Error())
		if _, rbErr := r.client.Call(ctx, "interface.rollback"); rbErr != nil {
			resp.Diagnostics.AddError("Rollback of staged changes also failed",
				"Staged interface changes may remain pending on TrueNAS: "+rbErr.Error())
		}
		return
	}

	if err := r.commitAndCheckin(ctx); err != nil {
		resp.Diagnostics.AddError("Commit interface changes failed", err.Error())
		return
	}

	api, err := r.getInstance(ctx, plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, api, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *NetworkInterfaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state NetworkInterfaceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	api, err := r.getInstance(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read interface failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *NetworkInterfaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan NetworkInterfaceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state NetworkInterfaceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	payload, diags := plan.updatePayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	stagingMu.Lock()
	defer stagingMu.Unlock()

	if _, err := r.client.Call(ctx, "interface.update", plan.ID.ValueString(), payload); err != nil {
		resp.Diagnostics.AddError("Update interface failed", err.Error())
		if _, rbErr := r.client.Call(ctx, "interface.rollback"); rbErr != nil {
			resp.Diagnostics.AddError("Rollback of staged changes also failed",
				"Staged interface changes may remain pending on TrueNAS: "+rbErr.Error())
		}
		return
	}

	if err := r.commitAndCheckin(ctx); err != nil {
		resp.Diagnostics.AddError("Commit interface changes failed", err.Error())
		return
	}

	// If this read-back fails, the update was already committed successfully;
	// we just surface the error and leave prior state in place. Terraform
	// keeps the last-known-good state and the next refresh/plan will read
	// the real (already-applied) config and self-heal, so no data is lost.
	api, err := r.getInstance(ctx, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, api, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *NetworkInterfaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state NetworkInterfaceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.Type.ValueString() == "PHYSICAL" {
		resp.Diagnostics.AddWarning(
			"Physical interface not deleted",
			"Physical interfaces cannot be deleted via the API; the resource is being removed "+
				"from Terraform state only, the interface itself is left untouched on TrueNAS.",
		)
		return
	}

	stagingMu.Lock()
	defer stagingMu.Unlock()

	_, err := r.client.Call(ctx, "interface.delete", state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			// Nothing was staged; no need to commit.
			return
		}
		resp.Diagnostics.AddError("Delete interface failed", err.Error())
		if _, rbErr := r.client.Call(ctx, "interface.rollback"); rbErr != nil {
			resp.Diagnostics.AddError("Rollback of staged changes also failed",
				"Staged interface changes may remain pending on TrueNAS: "+rbErr.Error())
		}
		return
	}

	if err := r.commitAndCheckin(ctx); err != nil {
		resp.Diagnostics.AddError("Commit interface changes failed", err.Error())
		return
	}
}

func (r *NetworkInterfaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// id and name are both the interface name string; set both so the
	// framework's subsequent Read (which looks up by state.ID) succeeds.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
)

var _ resource.Resource = &VMResource{}
var _ resource.ResourceWithImportState = &VMResource{}
var _ resource.ResourceWithIdentity = &VMResource{}

// VMResource implements the truenas_vm resource.
type VMResource struct{ client *client.Client }

// NewResource returns a new VMResource.
func NewResource() resource.Resource { return &VMResource{} }

func (r *VMResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm"
}

func (r *VMResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *VMResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.IntIDIdentitySchema()
}

func (r *VMResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// getInstance reads back a VM by ID via vm.get_instance.
func (r *VMResource) getInstance(ctx context.Context, id int64) (*vmAPI, error) {
	raw, err := r.client.CallRead(ctx, "vm.get_instance", id)
	if err != nil {
		return nil, err
	}
	var api vmAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

func (r *VMResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan VMModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := plan.apiPayload()

	raw, err := r.client.Call(ctx, "vm.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create VM failed", err.Error())
		return
	}

	// Parse the create response only to get the ID; read back the full object.
	var created vmAPI
	if err := json.Unmarshal(raw, &created); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	// Read before the read-back below overwrites plan.Running with the
	// current (stopped) state.
	wantRunning := !plan.Running.IsNull() && !plan.Running.IsUnknown() && plan.Running.ValueBool()

	// The VM exists from here on: record it in state before anything else can
	// fail, so a failed start or read-back leaves a tainted resource that the
	// next apply replaces, not an orphan whose name blocks the next create.
	// (BUGS B-045)
	apiResp, err := r.getInstance(ctx, created.ID)
	if err != nil {
		resp.Diagnostics.Append(r.saveCreated(ctx, resp, plan, created.ID)...)
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}
	responseToModel(apiResp, &plan)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !wantRunning {
		return
	}
	if _, err := r.client.Call(ctx, "vm.start", created.ID); err != nil {
		resp.Diagnostics.AddError("Start VM failed", err.Error())
		return
	}
	apiResp, err = r.getInstance(ctx, created.ID)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after start failed", err.Error())
		return
	}
	responseToModel(apiResp, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// saveCreated records a just-created VM by id alone when its read-back
// failed, so it is tracked (tainted) rather than orphaned. Unknown computed
// values become null; the next refresh fills them in.
func (r *VMResource) saveCreated(ctx context.Context, resp *resource.CreateResponse, plan VMModel, id int64) diag.Diagnostics {
	var diags diag.Diagnostics
	plan.ID = types.Int64Value(id)
	nullUnknowns(&plan)
	diags.Append(listing.SetIdentity(ctx, resp.Identity, id)...)
	diags.Append(resp.State.Set(ctx, &plan)...)
	return diags
}

func (r *VMResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state VMModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.getInstance(ctx, state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read VM failed", err.Error())
		return
	}

	responseToModel(apiResp, &state)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *VMResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan VMModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Carry the existing state ID into the plan (plan.ID is Unknown until apply).
	var state VMModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	// vm.update rejects create-only fields ("Extra inputs are not permitted").
	// They are marked RequiresReplace in the schema, so a change to them recreates
	// the VM rather than reaching this in-place update; here they are always
	// unchanged, so dropping them loses nothing.
	payload := plan.apiUpdatePayload()

	if _, err := r.client.Call(ctx, "vm.update", plan.ID.ValueInt64(), payload); err != nil {
		resp.Diagnostics.AddError("Update VM failed", err.Error())
		return
	}

	wantRunning := !plan.Running.IsNull() && !plan.Running.IsUnknown() && plan.Running.ValueBool()
	wasRunning := state.Running.ValueBool()
	if wantRunning != wasRunning {
		if wantRunning {
			if _, err := r.client.Call(ctx, "vm.start", plan.ID.ValueInt64()); err != nil {
				resp.Diagnostics.AddError("Start VM failed", err.Error())
				return
			}
		} else {
			if _, err := r.client.CallJob(ctx, "vm.stop", plan.ID.ValueInt64(), map[string]any{"force": false, "force_after_timeout": true}); err != nil {
				resp.Diagnostics.AddError("Stop VM failed", err.Error())
				return
			}
		}
	}

	apiResp, err := r.getInstance(ctx, plan.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	responseToModel(apiResp, &plan)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *VMResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state VMModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read current state to determine whether the VM needs to be stopped first.
	apiResp, err := r.getInstance(ctx, state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Read VM before delete failed", err.Error())
		return
	}
	if apiResp != nil && apiResp.Status.State == "RUNNING" {
		// Best-effort stop; ignore errors since the VM is being deleted regardless.
		r.client.CallJob(ctx, "vm.stop", state.ID.ValueInt64(), map[string]any{"force": true, "force_after_timeout": true}) //nolint:errcheck
	}

	_, err = r.client.Call(ctx, "vm.delete", state.ID.ValueInt64(), map[string]any{"force": false, "zvols": false})
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete VM failed", err.Error())
	}
}

func (r *VMResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, idDiags := listing.ImportInt64(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.getInstance(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Import VM failed", err.Error())
		return
	}

	var state VMModel
	responseToModel(apiResp, &state)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, id)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
)

var _ resource.Resource = &SnapshotResource{}
var _ resource.ResourceWithImportState = &SnapshotResource{}
var _ resource.ResourceWithIdentity = &SnapshotResource{}

// SnapshotResource implements the truenas_snapshot resource.
// Snapshots are immutable — there is no Update method.
type SnapshotResource struct {
	client *client.Client
}

func NewResource() resource.Resource { return &SnapshotResource{} }

func (r *SnapshotResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_snapshot"
}

func (r *SnapshotResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *SnapshotResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.StringIDIdentitySchema()
}

func (r *SnapshotResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type",
			fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *SnapshotResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SnapshotModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := map[string]any{
		"dataset":   plan.Dataset.ValueString(),
		"name":      plan.Name.ValueString(),
		"recursive": plan.Recursive.ValueBool(),
	}

	_, err := r.client.Call(ctx, "pool.snapshot.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create snapshot failed", err.Error())
		return
	}

	// pool.snapshot.create returns the snapshot name string, not a full object.
	// Read back via get_instance for canonical state.
	snapID := plan.Dataset.ValueString() + "@" + plan.Name.ValueString()
	raw, err := r.client.CallRead(ctx, "pool.snapshot.get_instance", snapID)
	if err != nil {
		resp.Diagnostics.AddError("Read after create failed", err.Error())
		return
	}

	var api snapshotAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse read response", err.Error())
		return
	}

	responseToModel(&api, &plan)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SnapshotResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SnapshotModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	raw, err := r.client.CallRead(ctx, "pool.snapshot.get_instance", id)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read snapshot failed", err.Error())
		return
	}

	var api snapshotAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse read response", err.Error())
		return
	}

	// preserve write-only recursive from state
	responseToModel(&api, &state)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update only records "recursive" after import: it is never returned, so it
// reads back null, and planmod.ReplaceFromKnownBool lets a null prior value be
// set in place. Every other change replaces the snapshot, so there is nothing
// to send.
func (r *SnapshotResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state SnapshotModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Recursive = plan.Recursive
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SnapshotResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SnapshotModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete the way it was created: a recursive snapshot also took
	// child@snap on every child dataset, and leaving those behind blocks the
	// (non-recursive) delete of the child datasets themselves.
	_, err := r.client.Call(ctx, "pool.snapshot.delete", state.ID.ValueString(),
		map[string]any{"recursive": state.Recursive.ValueBool()})
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete snapshot failed", err.Error())
	}
}

func (r *SnapshotResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, idDiags := listing.ImportString(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Import ID must be "dataset@snapname".
	parts := strings.SplitN(id, "@", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID",
			fmt.Sprintf("expected \"dataset@snapname\", got %q", id))
		return
	}

	raw, err := r.client.CallRead(ctx, "pool.snapshot.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import snapshot failed", err.Error())
		return
	}

	var api snapshotAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	var state SnapshotModel
	// recursive is unknown after import; leave it null
	responseToModel(&api, &state)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

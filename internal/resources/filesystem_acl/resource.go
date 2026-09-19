// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_acl

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &FilesystemAclResource{}
var _ resource.ResourceWithImportState = &FilesystemAclResource{}

// FilesystemAclResource implements the truenas_filesystem_acl resource: a
// declarative wrapper around the imperative filesystem.setacl action,
// keyed by path rather than a TrueNAS-assigned id (same pattern as
// truenas_filesystem_permissions).
type FilesystemAclResource struct{ client *client.Client }

// NewResource returns a new FilesystemAclResource.
func NewResource() resource.Resource { return &FilesystemAclResource{} }

func (r *FilesystemAclResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_filesystem_acl"
}

func (r *FilesystemAclResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *FilesystemAclResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// filesystem.setacl is job:true (probed live via core.get_methods and a
// real CallJob round trip); its job result carries the exact same shape as
// filesystem.getacl's response (both probed live), so applyACL parses the
// CallJob result directly instead of a follow-up filesystem.getacl call.
// filesystem.getacl itself is job:false (plain synchronous Call).

// applyACL calls filesystem.setacl with the plan's entries/uid/gid/options
// and parses its job result (same shape as filesystem.getacl's response).
func (r *FilesystemAclResource) applyACL(ctx context.Context, m *FilesystemAclModel) (*fsGetAclAPI, error) {
	raw, err := r.client.CallJob(ctx, "filesystem.setacl", m.setaclPayload())
	if err != nil {
		return nil, fmt.Errorf("filesystem.setacl: %w", err)
	}
	var api fsGetAclAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, fmt.Errorf("parse filesystem.setacl job result: %w", err)
	}
	return &api, nil
}

func (r *FilesystemAclResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan FilesystemAclModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	api, err := r.applyACL(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Create filesystem ACL failed", err.Error())
		return
	}

	// Keep the plan's exact "entries" JSON text in state (write-what-you-
	// said): the server may normalize cosmetic details (e.g. an omitted
	// "id" becomes an explicit -1). See entriesDrifted / responseToModel.
	resp.Diagnostics.Append(responseToModel(api, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *FilesystemAclResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state FilesystemAclModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "filesystem.getacl", state.Path.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read filesystem ACL failed", err.Error())
		return
	}

	var api fsGetAclAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse filesystem.getacl response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(&api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	drifted, diags := entriesDrifted(state.Entries.ValueString(), api.ACL)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if drifted {
		entriesJSON, d := canonicalEntriesJSON(api.ACL)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.Entries = types.StringValue(entriesJSON)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *FilesystemAclResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan FilesystemAclModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	api, err := r.applyACL(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Update filesystem ACL failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(api, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete calls filesystem.setacl with options.stripacl=true: probed live
// (see model.go's deleteACLPayload / deleteWarningDiagnostics and the task
// report) to cleanly convert the path's ACL to a trivial, mode-derived one
// on both NFS4 and POSIX1E - the "strip if clean" branch of this task's
// documented Delete-semantics decision rule. If the path is already gone
// (client.IsNotFound), there is nothing to strip and Delete succeeds
// silently, matching Read/ImportState's same-path not-found handling.
func (r *FilesystemAclResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state FilesystemAclModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := state.Path.ValueString()
	_, err := r.client.CallJob(ctx, "filesystem.setacl", deleteACLPayload(path))
	if err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Delete filesystem ACL failed", err.Error())
		return
	}

	resp.Diagnostics.Append(deleteWarningDiagnostics(path)...)
}

func (r *FilesystemAclResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by path (e.g. "terraform import truenas_filesystem_acl.example /mnt/tank/mydata").
	// recursive/traverse are apply-time-only options with nothing on the
	// wire to recover them from; they're left null and must be ignored via
	// ImportStateVerifyIgnore.
	raw, err := r.client.CallRead(ctx, "filesystem.getacl", req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Import filesystem ACL failed", err.Error())
		return
	}

	var api fsGetAclAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse filesystem.getacl response", err.Error())
		return
	}

	var state FilesystemAclModel
	resp.Diagnostics.Append(responseToModel(&api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	entriesJSON, diags := canonicalEntriesJSON(api.ACL)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Entries = types.StringValue(entriesJSON)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

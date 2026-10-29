// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_acl

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
)

var _ resource.Resource = &FilesystemAclResource{}
var _ resource.ResourceWithImportState = &FilesystemAclResource{}
var _ resource.ResourceWithIdentity = &FilesystemAclResource{}
var _ resource.ResourceWithModifyPlan = &FilesystemAclResource{}

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

// IdentitySchema is provided for identity-based import even though this
// resource is NOT listable: filesystem.getacl/setacl operate on an
// arbitrary, caller-supplied filesystem path rather than a TrueNAS-enumerated
// object, and there is no "list every ACL-managed path" query method to back
// a list resource with (see the task report for the full reasoning).
func (r *FilesystemAclResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.StringIDIdentitySchema()
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

// applyACL calls filesystem.setacl with the plan's entries/uid/gid and the
// configuration's write-only recursive/traverse options, and parses its job
// result (same shape as filesystem.getacl's response).
func (r *FilesystemAclResource) applyACL(ctx context.Context, m *FilesystemAclModel, cfg tfsdk.Config) (*fsGetAclAPI, error) {
	withOpts := *m
	cfg.GetAttribute(ctx, path.Root("recursive"), &withOpts.Recursive)
	cfg.GetAttribute(ctx, path.Root("traverse"), &withOpts.Traverse)
	raw, err := r.client.CallFilesystemMutation(ctx, "filesystem.setacl", withOpts.setaclPayload())
	if err != nil {
		return nil, fmt.Errorf("filesystem.setacl: %w", err)
	}
	var api fsGetAclAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, fmt.Errorf("parse filesystem.setacl job result: %w", err)
	}
	return &api, nil
}

// ModifyPlan keeps the entries text in state when the configured entries
// mean the same ACL (EntriesDiffer): after an import (state holds the
// server's canonical JSON) or a spelling-only edit (a partial bag, who
// instead of id, key order) the plan is empty instead of an update. Terraform
// accepts a planned value that differs from the configuration when it equals
// the prior state. (B-092, B-093)
func (r *FilesystemAclResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if r.client == nil {
		return
	}
	resp.Diagnostics.Append(KeepEquivalentEntries(ctx, r.client, req, resp, path.Root("entries"))...)
}

// KeepEquivalentEntries plans the state value of the ACL attribute at p when
// the configured value is the same ACL (EntriesDiffer). Shared with
// truenas_acl_template.
func KeepEquivalentEntries(ctx context.Context, c IDLookup, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse, p path.Path) diag.Diagnostics {
	var diags diag.Diagnostics
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return diags
	}
	var cfg, prior types.String
	diags.Append(req.Config.GetAttribute(ctx, p, &cfg)...)
	diags.Append(req.State.GetAttribute(ctx, p, &prior)...)
	if diags.HasError() || cfg.IsNull() || cfg.IsUnknown() || prior.IsNull() || prior.IsUnknown() ||
		cfg.ValueString() == prior.ValueString() {
		return diags
	}
	differ, err := EntriesDiffer(ctx, c, cfg.ValueString(), json.RawMessage(prior.ValueString()))
	if err != nil {
		// Leave the plan as configured; apply reports a real problem.
		return diags
	}
	if !differ {
		diags.Append(resp.Plan.SetAttribute(ctx, p, prior)...)
	}
	return diags
}

// getACL reads a path's ACL with filesystem.getacl.
func (r *FilesystemAclResource) getACL(ctx context.Context, p string) (*fsGetAclAPI, error) {
	raw, err := r.client.CallRead(ctx, "filesystem.getacl", p)
	if err != nil {
		return nil, fmt.Errorf("filesystem.getacl: %w", err)
	}
	var api fsGetAclAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, fmt.Errorf("parse filesystem.getacl response: %w", err)
	}
	return &api, nil
}

func (r *FilesystemAclResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan FilesystemAclModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	api, err := r.applyACL(ctx, &plan, req.Config)
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
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueString())...)
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

	drifted, diags := entriesDrifted(ctx, r.client, state.Entries.ValueString(), api.ACL)
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

	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *FilesystemAclResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state FilesystemAclModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// An update whose entries mean the same ACL as state (another spelling:
	// an import's canonical text, a partial bag, who instead of id) and whose
	// owner is unchanged only records the new text: re-running setacl would
	// rewrite the ACL, recursively if recursive is set, for nothing.
	differ, err := EntriesDiffer(ctx, r.client, plan.Entries.ValueString(), json.RawMessage(state.Entries.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Compare ACL entries failed", err.Error())
		return
	}
	var api *fsGetAclAPI
	if differ || !plan.UID.Equal(state.UID) || !plan.GID.Equal(state.GID) {
		api, err = r.applyACL(ctx, &plan, req.Config)
	} else {
		api, err = r.getACL(ctx, plan.Path.ValueString())
	}
	if err != nil {
		resp.Diagnostics.AddError("Update filesystem ACL failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(api, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueString())...)
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

	p := state.Path.ValueString()
	_, err := r.client.CallFilesystemMutation(ctx, "filesystem.setacl", deleteACLPayload(p))
	if err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Delete filesystem ACL failed", err.Error())
		return
	}

	resp.Diagnostics.Append(deleteWarningDiagnostics(p)...)
}

func (r *FilesystemAclResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by path (e.g. "terraform import truenas_filesystem_acl.example /mnt/tank/mydata").
	// recursive/traverse are write-only apply-time options: never in state,
	// so an import cannot differ from a configuration that sets them (B-115).
	id, idDiags := listing.ImportString(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "filesystem.getacl", id)
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

	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

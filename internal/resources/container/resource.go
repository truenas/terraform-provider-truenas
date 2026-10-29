// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container

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

var _ resource.Resource = &ContainerResource{}
var _ resource.ResourceWithImportState = &ContainerResource{}
var _ resource.ResourceWithIdentity = &ContainerResource{}

// ContainerResource implements the truenas_container resource.
type ContainerResource struct{ client *client.Client }

// NewResource returns a new ContainerResource.
func NewResource() resource.Resource { return &ContainerResource{} }

func (r *ContainerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_container"
}

func (r *ContainerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *ContainerResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.IntIDIdentitySchema()
}

func (r *ContainerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// checkVersion probes the target server's release and returns a clean error
// diagnostic if it is below the TrueNAS 26.0 floor the container namespace
// requires, before any container.* call is made. Every resource entry
// point (Create/Read/Update) and the datasource's Read call this first.
func (r *ContainerResource) checkVersion(ctx context.Context) diag.Diagnostics {
	var diags diag.Diagnostics
	version, err := r.client.ServerVersion(ctx)
	if err != nil {
		diags.AddError("Version probe failed", err.Error())
		return diags
	}
	diags.Append(versionGateDiagnostics(version)...)
	return diags
}

// getInstance reads back a container by ID via container.get_instance
// (job:false, probed live).
func (r *ContainerResource) getInstance(ctx context.Context, id int64) (*containerAPI, error) {
	raw, err := r.client.CallRead(ctx, "container.get_instance", id)
	if err != nil {
		return nil, err
	}
	var api containerAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

// startContainer starts a container. container.start is job:false (probed
// live), so a plain synchronous Call is sufficient.
func (r *ContainerResource) startContainer(ctx context.Context, id int64) error {
	_, err := r.client.Call(ctx, "container.start", id)
	return err
}

// stopContainer stops a container via container.stop, which IS a job
// (probed live, unlike start/delete). Stopping an already-stopped
// container fails the job — with "Domain '<name>' does not exist" if it
// was never started, or "Domain '<name>' is not active" if it was started
// at least once but is already stopped (both probed live) —
// isContainerAlreadyStopped identifies both cases so callers can treat
// them as already-stopped rather than a genuine error.
func (r *ContainerResource) stopContainer(ctx context.Context, id int64, force bool) error {
	_, err := r.client.CallJob(ctx, "container.stop", id, map[string]any{
		"force":               force,
		"force_after_timeout": true,
	})
	if err != nil && isContainerAlreadyStopped(err) {
		return nil
	}
	return err
}

func (r *ContainerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.Append(r.checkVersion(ctx)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan ContainerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.createPayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// container.create is job:true (probed live).
	raw, err := r.client.CallJob(ctx, "container.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create container failed", err.Error())
		return
	}
	var created containerAPI
	if err := json.Unmarshal(raw, &created); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	// Read before the read-back below overwrites plan.Running with the
	// current (stopped) state.
	wantRunning := !plan.Running.IsNull() && !plan.Running.IsUnknown() && plan.Running.ValueBool()

	// The container exists from here on: record it in state before anything
	// else can fail, so a failed start or read-back leaves a tainted resource
	// that the next apply replaces, not an orphan whose name blocks the next
	// create. (BUGS B-045)
	apiResp, err := r.getInstance(ctx, created.ID)
	if err != nil {
		resp.Diagnostics.Append(saveCreated(ctx, resp, plan, created.ID)...)
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	// Idmap must be resolved BEFORE responseToModel overwrites other fields,
	// since it needs the planned value (see idmapResponseValue's doc
	// comment for why this can't just be re-derived from the API response
	// unconditionally). Image is never touched by responseToModel at all —
	// plan.Image (already in `plan`) is carried through unchanged.
	plan.Idmap = idmapResponseValue(plan.Idmap, apiResp.Idmap)

	resp.Diagnostics.Append(responseToModel(ctx, apiResp, &plan)...)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !wantRunning {
		return
	}
	if err := r.startContainer(ctx, created.ID); err != nil {
		resp.Diagnostics.AddError("Start container failed", err.Error())
		return
	}
	apiResp, err = r.getInstance(ctx, created.ID)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after start failed", err.Error())
		return
	}
	resp.Diagnostics.Append(responseToModel(ctx, apiResp, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// saveCreated records a just-created container by id alone when its
// read-back failed, so it is tracked (tainted) rather than orphaned. Unknown
// computed values become null; the next refresh fills them in.
func saveCreated(ctx context.Context, resp *resource.CreateResponse, plan ContainerModel, id int64) diag.Diagnostics {
	var diags diag.Diagnostics
	plan.ID = types.Int64Value(id)
	nullUnknowns(&plan)
	diags.Append(listing.SetIdentity(ctx, resp.Identity, id)...)
	diags.Append(resp.State.Set(ctx, &plan)...)
	return diags
}

func (r *ContainerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	resp.Diagnostics.Append(r.checkVersion(ctx)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state ContainerModel
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
		resp.Diagnostics.AddError("Read container failed", err.Error())
		return
	}

	// Image and Idmap are deliberately left untouched: both are immutable
	// after creation, and the API never echoes back "image" at all (see
	// ContainerModel's doc comment) while re-deriving "idmap" from the API
	// response here would risk a formatting mismatch against the value
	// already recorded in state (see idmapResponseValue's doc comment).
	resp.Diagnostics.Append(responseToModel(ctx, apiResp, &state)...)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ContainerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.Append(r.checkVersion(ctx)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan ContainerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Carry the existing state ID into the plan (plan.ID is Unknown until apply).
	var state ContainerModel
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

	// container.update is job:false (probed live).
	if _, err := r.client.Call(ctx, "container.update", plan.ID.ValueInt64(), payload); err != nil {
		resp.Diagnostics.AddError("Update container failed", err.Error())
		return
	}

	wantRunning := !plan.Running.IsNull() && !plan.Running.IsUnknown() && plan.Running.ValueBool()
	wasRunning := state.Running.ValueBool()
	if wantRunning != wasRunning {
		if wantRunning {
			if err := r.startContainer(ctx, plan.ID.ValueInt64()); err != nil {
				resp.Diagnostics.AddError("Start container failed", err.Error())
				return
			}
		} else {
			if err := r.stopContainer(ctx, plan.ID.ValueInt64(), false); err != nil {
				resp.Diagnostics.AddError("Stop container failed", err.Error())
				return
			}
		}
	}

	apiResp, err := r.getInstance(ctx, plan.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, apiResp, &plan)...)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete stops the container (force, tolerating the never-started "Domain
// does not exist" case — see stopContainer) and then calls container.delete
// by id. container.delete is version-dependent: job:false on 26.0 but job:true
// on 28.0 (probed live). CallJob handles both — it waits for the delete job on
// 28.0 (so a dependent CheckDestroy / re-apply sees the container actually gone)
// and falls back cleanly when the method returns synchronously on 26.0. Using a
// plain Call here returned before the 28.0 delete job finished, leaving the
// container present. It takes no options beyond the id.
func (r *ContainerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(r.checkVersion(ctx)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state ContainerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueInt64()
	if err := r.stopContainer(ctx, id, true); err != nil {
		resp.Diagnostics.AddError("Stop container before delete failed", err.Error())
		return
	}

	if _, err := r.client.CallJob(ctx, "container.delete", id); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete container failed", err.Error())
	}
}

// ImportState imports a container by its numeric ID. "image" cannot be
// recovered from the API (see ContainerModel's doc comment) and is left
// null; the acceptance test's ImportStateVerify must ignore it.
// "idmap" IS recovered from the API response here, since import has no
// prior planned value to stay consistent with.
func (r *ContainerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(r.checkVersion(ctx)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, idDiags := listing.ImportInt64(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.getInstance(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Import container failed", err.Error())
		return
	}

	state := ContainerModel{Image: types.ObjectNull(imageAttrTypes)}
	state.Idmap = idmapFromAPI(apiResp.Idmap)
	resp.Diagnostics.Append(responseToModel(ctx, apiResp, &state)...)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, id)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

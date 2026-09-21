// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package pool

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &PoolResource{}
var _ resource.ResourceWithImportState = &PoolResource{}
var _ resource.ResourceWithModifyPlan = &PoolResource{}

// PoolResource manages a ZFS pool via the TrueNAS WebSocket API.
type PoolResource struct {
	client *client.Client
}

// NewResource returns a new PoolResource.
func NewResource() resource.Resource { return &PoolResource{} }

func (r *PoolResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pool"
}

func (r *PoolResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *PoolResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data",
			fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

// diskResolver builds a disk-name resolver from disk.query. A failure is
// non-fatal: newDiskResolver returns a usable empty resolver whose lookups
// fall back to the raw name, so a transient disk.query hiccup never blocks an
// otherwise-valid pool operation (the worst case is state keeps the volatile
// sdX name, i.e. the pre-#9 behavior) — hence the error is intentionally
// dropped here rather than surfaced as a diagnostic.
func (r *PoolResource) diskResolver(ctx context.Context) *diskResolver {
	res, _ := newDiskResolver(ctx, r.client)
	return res
}

// ModifyPlan reconciles the planned topology against state so that a disk
// named in a different form than state — but pointing at the same physical
// disk — does not read as a change and force a pool replacement (issue #9),
// and so a "DISK"/"STRIPE" type spelling difference likewise collapses. It
// runs after the topology attribute's RequiresReplace modifier but before
// Terraform core makes the final replacement determination against the
// returned plan, so a suppressed form-only difference does not trigger
// replacement while a real disk swap or topology change still does.
func (r *PoolResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() { // destroy
		return
	}
	if r.client == nil { // not configured (e.g. some plan-only paths)
		return
	}

	var plan PoolModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Nothing to reconcile on Create: there is no prior state, and Terraform
	// forbids a plan modifier from setting a Required attribute (topology
	// disks/type) to any value other than the config value when there is no
	// prior state to normalize against. Create stores the planned (config-form)
	// topology verbatim so applied == planned; the first Read canonicalizes it
	// to serials, and subsequent plans reconcile against that state here.
	if req.State.Raw.IsNull() {
		return
	}
	var state PoolModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	newTopo, d := reconcilePlanTopology(ctx, plan.Topology, &state.Topology, r.diskResolver(ctx))
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Topology = newTopo
	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
}

func (r *PoolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan PoolModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res := r.diskResolver(ctx)
	payload, diags := plan.apiPayload(ctx, res)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallJob(ctx, "pool.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create pool failed", err.Error())
		return
	}

	var apiResp poolAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	// autotrim is not accepted by pool.create (it is a pool.update field), so
	// apply it in a follow-up update when the user set it.
	if !plan.AutoTrim.IsNull() && !plan.AutoTrim.IsUnknown() {
		if _, err := r.client.CallJob(ctx, "pool.update", apiResp.ID,
			map[string]any{"autotrim": autotrimStr(plan.AutoTrim.ValueBool())}); err != nil {
			resp.Diagnostics.AddError("Set autotrim after pool create failed", err.Error())
			return
		}
		// Re-read so state reflects the applied autotrim.
		raw, err = r.client.CallRead(ctx, "pool.get_instance", apiResp.ID)
		if err != nil {
			resp.Diagnostics.AddError("Read-back after pool create failed", err.Error())
			return
		}
		if err := json.Unmarshal(raw, &apiResp); err != nil {
			resp.Diagnostics.AddError("Parse get_instance response", err.Error())
			return
		}
	}

	// Preserve the planned (config-form) topology: Terraform requires the
	// applied state of the Required topology to equal what was planned, and the
	// plan carries the disk names/type spellings exactly as the user wrote them
	// (responseToModel would instead substitute the canonical serials/types the
	// next Read will store). The first Read after create canonicalizes state.
	plannedTopo := plan.Topology
	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &plan, res)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Topology = applyPlannedTopology(plannedTopo, plan.Topology)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PoolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state PoolModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "pool.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read pool failed", err.Error())
		return
	}

	var apiResp poolAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &state, r.diskResolver(ctx))...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *PoolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan PoolModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Only autotrim is updatable; topology and name are ForceNew, so this is the
	// only field that changes. Apply it and persist the plan as-is rather than
	// re-reading: the computed pool attributes carry UseStateForUnknown, so the
	// plan already holds their (known) prior values, and overwriting them with a
	// fresh read here would make the applied state differ from the plan for the
	// live counters (allocated/free) — "Provider produced inconsistent result
	// after apply". They are refreshed on the next Read.
	if _, err := r.client.CallJob(ctx, "pool.update", plan.ID.ValueInt64(),
		map[string]any{"autotrim": autotrimStr(plan.AutoTrim.ValueBool())}); err != nil {
		resp.Diagnostics.AddError("Update pool failed", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PoolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state PoolModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// TrueNAS has no pool.delete; a pool is destroyed via pool.export with
	// destroy=true. cascade=true removes attachments (shares, etc.) that would
	// otherwise block the destroy — appropriate here since the pool is being
	// permanently torn down.
	_, err := r.client.CallJob(ctx, "pool.export", state.ID.ValueInt64(),
		map[string]any{"cascade": true, "destroy": true})
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete pool failed", err.Error())
	}
}

func (r *PoolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by pool id (integer) or name — the id form is what
	// ImportStateVerify uses (the resource id is the numeric pool id), while a
	// name is friendlier for a manual `terraform import`.
	var filter [][]any
	if id, err := strconv.ParseInt(req.ID, 10, 64); err == nil {
		filter = [][]any{{"id", "=", id}}
	} else {
		filter = [][]any{{"name", "=", req.ID}}
	}
	raw, err := r.client.CallRead(ctx, "pool.query", filter)
	if err != nil {
		resp.Diagnostics.AddError("Import pool failed", err.Error())
		return
	}

	var pools []poolAPI
	if err := json.Unmarshal(raw, &pools); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	if len(pools) == 0 {
		resp.Diagnostics.AddError("Pool not found",
			fmt.Sprintf("no pool with id or name %q found on TrueNAS", req.ID))
		return
	}

	var state PoolModel
	resp.Diagnostics.Append(responseToModel(ctx, &pools[0], &state, r.diskResolver(ctx))...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package pool

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
	"github.com/truenas/terraform-provider-truenas/internal/resources/dataset"
)

var _ resource.Resource = &PoolResource{}
var _ resource.ResourceWithImportState = &PoolResource{}
var _ resource.ResourceWithModifyPlan = &PoolResource{}
var _ resource.ResourceWithIdentity = &PoolResource{}

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

func (r *PoolResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.IntIDIdentitySchema()
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

// ModifyPlan does two things for a ZFS pool, whose data must never be put at
// risk by a plan:
//
//  1. It reconciles the planned topology against state so a disk named in a
//     different form than state — but pointing at the same physical disk, or a
//     member that is merely degraded/removed — does not read as a change
//     (issue #9), and so a "DISK"/"STRIPE" spelling difference collapses.
//
//  2. After reconciling, if the pool's topology or name has REALLY changed, it
//     refuses the apply with an actionable error rather than letting it become
//     a destroy/recreate. name and topology are not marked RequiresReplace
//     precisely so this runs instead: replacing a pool destroys all its data,
//     which is essentially never a valid automatic outcome (cf. AWS RDS/S3
//     deletion_protection/force_destroy, and Terraform's own prevent_destroy).
//     A genuine topology change is done in TrueNAS/zpool and reconciled with
//     `terraform apply -refresh-only`; a deliberate teardown is `terraform
//     destroy`.
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
	// topology verbatim so applied == planned; the first Read canonicalizes it,
	// and subsequent plans reconcile against that state here.
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

	// Refuse a real topology or name change rather than destroy the pool.
	if !plan.Name.Equal(state.Name) {
		resp.Diagnostics.AddError(
			"Pool cannot be renamed in place",
			fmt.Sprintf("This resource's pool is named %q but the configuration now asks for %q. "+
				"Renaming a ZFS pool cannot be done without destroying and recreating it, which would "+
				"erase all of its data, so this provider will not do it automatically. Rename the pool "+
				"outside Terraform if you truly intend to, or revert the name in configuration. To "+
				"deliberately destroy this pool, use `terraform destroy`.",
				state.Name.ValueString(), plan.Name.ValueString()),
		)
	}
	if topologyChanged(newTopo, state.Topology) {
		resp.Diagnostics.AddError(
			"Pool topology cannot be changed in place",
			"The configured pool topology differs from the pool's actual topology in a way that is not "+
				"a harmless disk-name difference (a real change of vdevs or member disks). Applying it "+
				"would require destroying and recreating the pool, erasing all of its data, so this "+
				"provider refuses rather than plan that.\n\n"+
				"If a disk failed or a spare stepped in, no configuration change is needed — the pool "+
				"reconciles automatically. To actually change the topology (grow the pool, replace a "+
				"disk, add/remove a cache/log/spare), do it in the TrueNAS UI or with `zpool`, then run "+
				"`terraform apply -refresh-only` to reconcile Terraform state. To deliberately destroy "+
				"and rebuild the pool, use `terraform destroy` (or `terraform apply -replace`).",
		)
	}

	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
}

// topologyChanged reports whether a reconciled plan topology differs from state
// in a way that represents a real change (different vdevs or member disks),
// as opposed to a harmless disk-name-form difference that reconcilePlanTopology
// has already collapsed to the state value.
func topologyChanged(planned TopologyModel, state TopologyModel) bool {
	return !planned.Data.Equal(state.Data) ||
		!planned.Log.Equal(state.Log) ||
		!planned.Cache.Equal(state.Cache) ||
		!planned.Spare.Equal(state.Spare)
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
	r.applyRootProps(ctx, plan.Name.ValueString(), &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// applyRootProps reads the pool's root dataset and sets the model's
// deduplication/checksum (which pool.query returns as null). A failure is
// non-fatal: the pool itself is fine, so it is surfaced as a warning.
func (r *PoolResource) applyRootProps(ctx context.Context, poolName string, m *PoolModel, diags *diag.Diagnostics) {
	raw, err := r.client.CallRead(ctx, "pool.dataset.get_instance", poolName)
	if err != nil {
		diags.AddWarning("Could not read pool root-dataset properties",
			fmt.Sprintf("deduplication/checksum for pool %q could not be read: %v", poolName, err))
		return
	}
	var rp rootDatasetProps
	if err := json.Unmarshal(raw, &rp); err != nil {
		diags.AddWarning("Could not parse pool root-dataset properties", err.Error())
		return
	}
	m.Deduplication, m.Checksum = rootPropsToValues(&rp)
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
	r.applyRootProps(ctx, state.Name.ValueString(), &state, &resp.Diagnostics)
	// force_destroy is provider-only: pre-1.6.2 state and imports have it null,
	// which reads as false (an import never grants it).
	state.ForceDestroy = dataset.ForceDestroyOrFalse(state.ForceDestroy)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueInt64())...)
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

	// deduplication / checksum are not pool.update fields — they live on the
	// pool's root dataset, so apply changes there.
	rootUpdate := map[string]any{}
	if !plan.Deduplication.IsNull() && !plan.Deduplication.IsUnknown() {
		rootUpdate["deduplication"] = strings.ToUpper(plan.Deduplication.ValueString())
	}
	if !plan.Checksum.IsNull() && !plan.Checksum.IsUnknown() {
		rootUpdate["checksum"] = strings.ToUpper(plan.Checksum.ValueString())
	}
	if len(rootUpdate) > 0 {
		if _, err := r.client.Call(ctx, "pool.dataset.update", plan.Name.ValueString(), rootUpdate); err != nil {
			resp.Diagnostics.AddError("Update pool root-dataset properties failed", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PoolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state PoolModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Data-safety guard (see destroy_guard.go): without force_destroy the
	// destroy is refused while anything lives on the pool.
	if !state.ForceDestroy.ValueBool() {
		contents, err := poolContents(ctx, r.client, state.Name.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Destroy pool refused", fmt.Sprintf("could not verify that pool %q is empty: %s", state.Name.ValueString(), err))
			return
		}
		if len(contents) > 0 {
			resp.Diagnostics.AddError("Destroy pool refused", refusalMessage(state.Name.ValueString(), contents))
			return
		}
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
	// Import by pool id (integer), pool name, or resource identity — the id
	// form is what ImportStateVerify uses (the resource id is the numeric
	// pool id), while a name is friendlier for a manual `terraform import`.
	var filter [][]any
	var lookup string
	if req.ID != "" {
		lookup = req.ID
		if id, err := strconv.ParseInt(req.ID, 10, 64); err == nil {
			filter = [][]any{{"id", "=", id}}
		} else {
			filter = [][]any{{"name", "=", req.ID}}
		}
	} else {
		id, idDiags := listing.ImportInt64(ctx, req)
		resp.Diagnostics.Append(idDiags...)
		if resp.Diagnostics.HasError() {
			return
		}
		lookup = strconv.FormatInt(id, 10)
		filter = [][]any{{"id", "=", id}}
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
			fmt.Sprintf("no pool with id or name %q found on TrueNAS", lookup))
		return
	}

	var state PoolModel
	resp.Diagnostics.Append(responseToModel(ctx, &pools[0], &state, r.diskResolver(ctx))...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.applyRootProps(ctx, state.Name.ValueString(), &state, &resp.Diagnostics)
	// force_destroy is provider-only: pre-1.6.2 state and imports have it null,
	// which reads as false (an import never grants it).
	state.ForceDestroy = dataset.ForceDestroyOrFalse(state.ForceDestroy)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

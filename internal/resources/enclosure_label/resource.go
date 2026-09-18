// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package enclosure_label

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// privateSetter is the subset of *privatestate.ProviderData (an internal
// terraform-plugin-framework type, not importable directly by provider
// code) that captureOriginalLabel needs. resource.CreateResponse.Private
// and resource.ImportStateResponse.Private both satisfy this structurally —
// Go does not require naming the concrete type to call its exported methods
// through an interface.
type privateSetter interface {
	SetKey(ctx context.Context, key string, value []byte) diag.Diagnostics
}

var _ resource.Resource = &EnclosureLabelResource{}
var _ resource.ResourceWithImportState = &EnclosureLabelResource{}

// EnclosureLabelResource implements the truenas_enclosure_label resource:
// the user-settable "label" of one pre-existing TrueNAS storage enclosure.
// See schema.go's resourceSchema Description for the full restore-on-destroy
// and import safety contract.
type EnclosureLabelResource struct{ client *client.Client }

// NewResource returns a new EnclosureLabelResource.
func NewResource() resource.Resource { return &EnclosureLabelResource{} }

func (r *EnclosureLabelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_enclosure_label"
}

func (r *EnclosureLabelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *EnclosureLabelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// lookupEnclosure finds an enclosure's current record via enclosure2.query
// (enclosure.get_instance does not exist — confirmed live, see model.go's
// sibling package internal/resources/enclosure's doc comments for the full
// probe evidence). Mirrors ipmi_lan's lookupChannel pattern: a filtered
// query plus a synthesized client.IsNotFound-compatible error when the
// enclosure is absent, since a query returning zero results is not itself
// an API error (probed live: this is exactly what a box with no enclosure
// hardware/license returns for every id).
func (r *EnclosureLabelResource) lookupEnclosure(ctx context.Context, id string) (*enclosureLabelAPI, error) {
	raw, err := r.client.CallRead(ctx, "enclosure2.query", enclosureQueryArgs(id)...)
	if err != nil {
		return nil, err
	}
	var results []enclosureLabelAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, &client.APIError{Code: 2, Message: fmt.Sprintf("enclosure %q not found", id)}
	}
	return &results[0], nil
}

// captureOriginalLabel reads an enclosure's CURRENT label and stashes it in
// private state under originalLabelPrivateKey, for Delete to restore later.
// Shared by Create and ImportState — see both callers' doc comments, and
// schema.go's resourceSchema Description ("RESTORE ON DESTROY" / "IMPORT"),
// for why capturing at either of those two points (never anywhere else)
// gives the same "restore to what this resource found when it started
// managing the enclosure" guarantee. Private state values must be valid
// JSON (framework requirement), hence json.Marshal rather than storing the
// raw string bytes directly.
func captureOriginalLabel(ctx context.Context, private privateSetter, api *enclosureLabelAPI) error {
	origBytes, err := json.Marshal(api.Label)
	if err != nil {
		return fmt.Errorf("encode original label: %w", err)
	}
	diags := private.SetKey(ctx, originalLabelPrivateKey, origBytes)
	if diags.HasError() {
		return fmt.Errorf("store original label in private state: %s", diags.Errors()[0].Detail())
	}
	return nil
}

func (r *EnclosureLabelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan EnclosureLabelModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// "id" and "label" are both Required (no Optional+Computed ambiguity
	// here, unlike ipmi_lan/failover_config), so req.Config and req.Plan
	// necessarily agree on Create; req.Config is used regardless, matching
	// this provider's config-driven-payload convention.
	var config EnclosureLabelModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := config.ID.ValueString()

	// Capture the ORIGINAL label BEFORE ever calling enclosure.label.set —
	// this is the one and only value Delete will restore.
	before, err := r.lookupEnclosure(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Enclosure lookup failed", err.Error())
		return
	}
	if err := captureOriginalLabel(ctx, resp.Private, before); err != nil {
		resp.Diagnostics.AddError("Capture original label failed", err.Error())
		return
	}

	if _, err := r.client.Call(ctx, "enclosure.label.set", id, config.Label.ValueString()); err != nil {
		resp.Diagnostics.AddError("Set enclosure label failed", err.Error())
		return
	}

	after, err := r.lookupEnclosure(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	responseToModel(after, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *EnclosureLabelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state EnclosureLabelModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	api, err := r.lookupEnclosure(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read enclosure label failed", err.Error())
		return
	}

	// req.Private is automatically carried through to resp.Private by the
	// framework unless this method explicitly changes it — nothing to do
	// here, the originally captured label stays intact across refreshes.
	responseToModel(api, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *EnclosureLabelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan EnclosureLabelModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// "id" is RequiresReplace, so plan.ID always equals the prior state's ID
	// here.

	var config EnclosureLabelModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := config.ID.ValueString()
	if _, err := r.client.Call(ctx, "enclosure.label.set", id, config.Label.ValueString()); err != nil {
		resp.Diagnostics.AddError("Set enclosure label failed", err.Error())
		return
	}

	api, err := r.lookupEnclosure(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	// req.Private -> resp.Private carries the original-label capture from
	// Create forward untouched (same framework auto-copy behavior as Read);
	// Update never re-captures it, since "original" always means "as found
	// before this resource instance started managing the enclosure", not
	// "as found before the most recent Update".
	responseToModel(api, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete restores the ORIGINAL label captured at Create/ImportState time
// (see schema.go's resourceSchema Description, "RESTORE ON DESTROY"), then
// lets Terraform remove the resource from state as usual. If no captured
// value is found (should not happen in ordinary use), it emits a WARNING
// rather than an error: Terraform must always be able to complete a
// destroy, and leaving the label at its last-managed value is a safe,
// non-blocking fallback.
func (r *EnclosureLabelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state EnclosureLabelModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	origBytes, diags := req.Private.GetKey(ctx, originalLabelPrivateKey)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(origBytes) == 0 {
		resp.Diagnostics.AddWarning(
			"Original enclosure label not restored",
			"No original label was found captured in this resource's private state (unexpected in ordinary "+
				"use — every Create and Import populates it), so the enclosure's label was left as its "+
				"last-managed value rather than restored. If this enclosure's label should be reset, set it "+
				"directly (TrueNAS UI, API, or midclient).",
		)
		return
	}

	var original string
	if err := json.Unmarshal(origBytes, &original); err != nil {
		resp.Diagnostics.AddError("Decode original label failed", err.Error())
		return
	}

	id := state.ID.ValueString()
	if _, err := r.client.Call(ctx, "enclosure.label.set", id, original); err != nil {
		resp.Diagnostics.AddError(
			"Restore original enclosure label failed",
			fmt.Sprintf("enclosure.label.set(%q, %q): %v", id, original, err),
		)
		return
	}
}

// ImportState captures the enclosure's label AS FOUND AT IMPORT TIME into
// private state, via the identical mechanism Create uses (captureOriginalLabel)
// — see schema.go's resourceSchema Description, "IMPORT", for the full
// justification: the framework pre-populates ImportStateResponse.Private
// with a valid (empty) instance before this method runs (confirmed against
// terraform-plugin-framework v1.19.0's fwserver/server_importresourcestate.go
// — despite ImportStateResponse.Private's doc comment saying it is "not
// pre-populated", that refers to prior DATA, not instantiation), so this is
// no different, mechanically, from what Create already does.
func (r *EnclosureLabelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID
	api, err := r.lookupEnclosure(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Import enclosure label failed", err.Error())
		return
	}

	if err := captureOriginalLabel(ctx, resp.Private, api); err != nil {
		resp.Diagnostics.AddError("Capture original label failed", err.Error())
		return
	}

	var state EnclosureLabelModel
	responseToModel(api, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

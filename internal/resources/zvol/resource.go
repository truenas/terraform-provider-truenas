// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
	"github.com/truenas/terraform-provider-truenas/internal/resources/dataset"
)

var _ resource.Resource = &ZvolResource{}
var _ resource.ResourceWithImportState = &ZvolResource{}
var _ resource.ResourceWithIdentity = &ZvolResource{}
var _ resource.ResourceWithValidateConfig = &ZvolResource{}
var _ resource.ResourceWithModifyPlan = &ZvolResource{}

// ValidateConfig rejects setting `encryption` together with
// `inherit_encryption = true`: with inheritance the parent determines
// encryption, so an explicit value is ambiguous (#31, mirrors truenas_dataset).
func (r *ZvolResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg zvolResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	inherit := !cfg.InheritEncryption.IsNull() && !cfg.InheritEncryption.IsUnknown() && cfg.InheritEncryption.ValueBool()
	encryptionSet := !cfg.Encryption.IsNull() && !cfg.Encryption.IsUnknown()
	if inherit && encryptionSet {
		resp.Diagnostics.AddAttributeError(
			path.Root("encryption"),
			"Conflicting encryption configuration",
			"encryption cannot be set when inherit_encryption = true: the zvol inherits "+
				"its encryption from the parent, which determines the value. Remove encryption, "+
				"or set inherit_encryption = false to manage encryption on this zvol.",
		)
	}
	// A sparse zvol has no refreservation: the server overrides a configured
	// one with 0, which then reads back different from the plan.
	if !cfg.Sparse.IsNull() && !cfg.Sparse.IsUnknown() && cfg.Sparse.ValueBool() &&
		!cfg.RefReservation.IsNull() && !cfg.RefReservation.IsUnknown() && cfg.RefReservation.ValueInt64() > 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("refreservation"),
			"Conflicting provisioning configuration",
			"refreservation cannot be above 0 when sparse = true: a sparse zvol has no refreservation. "+
				"Remove refreservation or sparse.",
		)
	}
}

// ModifyPlan rejects at plan time what cannot be applied: an encryption
// algorithm the release cannot create (dataset.CheckEncryptionAlgorithm), and a
// change of sparse on an existing zvol.
//
// sparse only takes effect at creation (pool.dataset.update has no sparse, and
// making a thin zvol thick needs the reservation ZFS computes at create). A
// change used to be planned as an in-place update that changed nothing on the
// server. (B-109) It is now an error that says how to change provisioning
// in place.
func (r *ZvolResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	resp.Diagnostics.Append(dataset.CheckEncryptionAlgorithm(ctx, r.client, req)...)
	// A zvol being replaced anyway is created with the configured sparse.
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() || len(resp.RequiresReplace) > 0 {
		return
	}
	var cfg, state zvolResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || cfg.Sparse.IsNull() || cfg.Sparse.IsUnknown() {
		return
	}
	current := state.Sparse
	if current.IsNull() || current.IsUnknown() {
		// State from before sparse was read back: derive it the way an
		// import does, from the refreservation.
		current = types.BoolValue(state.RefReservation.IsNull() || state.RefReservation.ValueInt64() == 0)
	}
	if cfg.Sparse.ValueBool() == current.ValueBool() {
		return
	}
	resp.Diagnostics.AddAttributeError(
		path.Root("sparse"),
		"sparse cannot be changed on an existing zvol",
		fmt.Sprintf("sparse is applied only when a zvol is created; TrueNAS cannot change it later. %s was "+
			"created %s, so keep sparse = %t. To change its provisioning in place, set refreservation (0 = "+
			"thin, or the number of bytes to guarantee); to recreate it with the other provisioning (its data "+
			"is lost), use terraform apply -replace.",
			state.Name.ValueString(), map[bool]string{true: "sparse", false: "thick"}[current.ValueBool()],
			current.ValueBool()),
	)
}

type ZvolResource struct {
	client *client.Client
}

func NewResource() resource.Resource { return &ZvolResource{} }

func (r *ZvolResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zvol"
}

func (r *ZvolResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *ZvolResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.StringIDIdentitySchema()
}

func (r *ZvolResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ZvolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan zvolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := plan.createAPIPayload()
	// encryption_passphrase / encryption_key are write-only: read from config
	// (null in plan/state) and inject into encryption_options.
	var cfg zvolResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	injectEncryptionSecrets(payload, &cfg.ZvolModel)
	// TrueNAS 27.0 removed encryption_options.algorithm (fixed server-side);
	// sending it there fails with "Extra inputs are not permitted". The algorithm
	// still reads back via the computed encryption_algorithm.
	if ok, verr := r.client.VersionAtLeast(ctx, 27, 0); verr == nil && ok {
		if eo, isMap := payload["encryption_options"].(map[string]any); isMap {
			delete(eo, "algorithm")
			if len(eo) == 0 {
				delete(payload, "encryption_options")
			}
		}
	}

	_, err := r.client.Call(ctx, "pool.dataset.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create zvol failed", err.Error())
		return
	}

	// Read back via get_instance for canonical state (create response omits some properties).
	raw, err := r.client.CallRead(ctx, "pool.dataset.get_instance", plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read after create failed", err.Error())
		return
	}

	var apiResp zvolAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse read response", err.Error())
		return
	}

	// TrueNAS 25.10 creates a thick zvol with the refreservation ZFS computes
	// for its volsize whatever refreservation was sent, so a configured one
	// read back different and failed the apply. Set it with an update. (found
	// while verifying B-038)
	if want := plan.RefReservation; !want.IsNull() && !want.IsUnknown() {
		got, _ := apiResp.RefReservP.parsedInt()
		if got != want.ValueInt64() {
			if _, err := r.client.Call(ctx, "pool.dataset.update", plan.Name.ValueString(),
				map[string]any{"refreservation": want.ValueInt64()}); err != nil {
				resp.Diagnostics.AddError("Set refreservation after create failed", err.Error())
				return
			}
			raw, err = r.client.CallRead(ctx, "pool.dataset.get_instance", plan.Name.ValueString())
			if err == nil {
				apiResp = zvolAPI{}
				err = json.Unmarshal(raw, &apiResp)
			}
			if err != nil {
				resp.Diagnostics.AddError("Read after create failed", err.Error())
				return
			}
		}
	}

	// pool.dataset.create returns before udev has created the zvol's
	// /dev/zvol/<name> device node, so a dependent resource created in the same
	// apply — an iSCSI extent (disk = zvol/<name>) or an NVMe-oF namespace — can
	// fail its server-side device-exists check with "Device ... does not exist"
	// and only succeed on a re-apply once the node has appeared. Block here until
	// the node is present so every consumer is safe. (#40)
	if !waitForZvolDevice(ctx, r.client, plan.Name.ValueString()) {
		resp.Diagnostics.AddWarning("Zvol device node not ready",
			fmt.Sprintf("%s did not appear within %s after the zvol was created. A resource that uses the "+
				"device in this apply (iSCSI extent, NVMe-oF namespace, VM disk) may fail; re-run the apply.",
				zvolDevicePath(plan.Name.ValueString()), zvolDeviceWaitTimeout))
	}

	responseToModel(&apiResp, &plan.ZvolModel)
	if plan.Sparse.IsUnknown() || plan.Sparse.IsNull() {
		plan.Sparse = types.BoolValue(sparseFromAPI(&apiResp))
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// deviceStater is the slice of the client waitForZvolDevice needs: a stat of a
// server path. *client.Client satisfies it; a fake drives the unit test.
type deviceStater interface {
	CallRead(ctx context.Context, method string, params ...any) (json.RawMessage, error)
}

// zvolDeviceWaitTimeout and zvolDevicePollInterval bound the device-settle
// wait. They are package vars (not consts) so the test can shrink them to run
// without real-time sleeps.
var (
	zvolDeviceWaitTimeout  = 30 * time.Second
	zvolDevicePollInterval = 500 * time.Millisecond
)

// waitForZvolDevice polls filesystem.stat on the zvol's /dev/zvol/<name> device
// node until it exists or a short timeout elapses. pool.dataset.create returns
// before udev has created that node, and under concurrent creation the node can
// lag the create by several seconds (measured ~4-7s at 10-20x concurrency), so
// a dependent resource — an iSCSI extent (disk = zvol/<name>), an NVMe-oF
// namespace, or a VM DISK device (path = /dev/zvol/<name>) — can fail its
// server-side device-exists check. The wait is producer-side, so it protects
// every consumer of the node, not only the ones named here. It is best effort:
// on timeout (or a persistent stat error) it returns without failing, so a
// genuinely slow device still lets the create finish rather than orphaning the
// just-created zvol — the dependent resource would at worst retry on the next
// apply, as it did before this wait existed. (#40)
//
// It reports whether the node appeared, so Create can warn when it did not.
func waitForZvolDevice(ctx context.Context, c deviceStater, name string) bool {
	devPath := zvolDevicePath(name)
	deadline := time.Now().Add(zvolDeviceWaitTimeout)
	for {
		if _, err := c.CallRead(ctx, "filesystem.stat", devPath); err == nil {
			return true
		}
		if ctx.Err() != nil || time.Now().After(deadline) {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(zvolDevicePollInterval):
		}
	}
}

// zvolDevicePath is the /dev/zvol node udev creates for a zvol. Spaces in the
// name become "+" (the middleware's zvol_name_to_path); without that mapping
// a zvol whose name has a space waited the full timeout for a node that never
// appears under that path. (B-160)
func zvolDevicePath(name string) string {
	return "/dev/zvol/" + strings.ReplaceAll(name, " ", "+")
}

func (r *ZvolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state zvolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "pool.dataset.get_instance", state.Name.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read zvol failed", err.Error())
		return
	}

	var apiResp zvolAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse read response", err.Error())
		return
	}

	responseToModel(&apiResp, &state.ZvolModel)
	// State written before sparse was Computed has it null when the
	// configuration never set it; fill it from the zvol so the upgrade does not
	// plan sparse as (known after apply).
	if state.Sparse.IsNull() {
		state.Sparse = types.BoolValue(sparseFromAPI(&apiResp))
	}
	// State written before force_destroy existed has it null; read it as the
	// default so the upgrade plans no change.
	state.ForceDestroy = dataset.ForceDestroyOrFalse(state.ForceDestroy)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ZvolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state zvolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := plan.updateAPIPayload(&state.ZvolModel)
	// Skip the API call when nothing the server stores changed: only
	// state-only attributes changed (force_destroy, a letter-case change).
	if len(payload) > 0 {
		_, err := r.client.Call(ctx, "pool.dataset.update", plan.Name.ValueString(), payload)
		if err != nil {
			resp.Diagnostics.AddError("Update zvol failed", err.Error())
			return
		}
	}

	// Re-read to get computed fields
	raw, err := r.client.CallRead(ctx, "pool.dataset.get_instance", plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read after update failed", err.Error())
		return
	}

	var apiResp zvolAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse update response", err.Error())
		return
	}

	responseToModel(&apiResp, &plan.ZvolModel)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ZvolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state zvolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Non-recursive unless force_destroy, with the same pre-check and error as
	// truenas_dataset: a zvol with snapshots is refused with a message naming
	// them instead of the server's version-dependent ENOTEMPTY text.
	force := !state.ForceDestroy.IsNull() && state.ForceDestroy.ValueBool()
	// Without force_destroy also refuse to destroy a zvol that holds data: a
	// removed resource or the destroy half of a replacement (a rename, a pool
	// move) must not wipe a LUN silently. "Cannot tell" does not block.
	if !force {
		if has, lr, err := dataset.ZvolHoldsData(ctx, r.client, state.Name.ValueString()); err == nil && has {
			resp.Diagnostics.AddError("Delete zvol refused",
				fmt.Sprintf("zvol %q holds data (%d bytes referenced). Set force_destroy = true to destroy it together "+
					"with its contents, or copy the data off first.", state.Name.ValueString(), lr))
			return
		}
	}
	resp.Diagnostics.Append(dataset.DeleteDataset(ctx, r.client, state.Name.ValueString(), "zvol", force)...)
}

func (r *ZvolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, idDiags := listing.ImportString(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state zvolResourceModel
	state.Name = types.StringValue(id)
	state.ID = types.StringValue(id)

	raw, err := r.client.CallRead(ctx, "pool.dataset.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import zvol failed", err.Error())
		return
	}

	var apiResp zvolAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	responseToModel(&apiResp, &state.ZvolModel)
	// sparse is not returned by the API; derive it from the refreservation so
	// an imported zvol matches a configuration that sets it. (B-109)
	state.Sparse = types.BoolValue(sparseFromAPI(&apiResp))
	state.ForceDestroy = types.BoolValue(false)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

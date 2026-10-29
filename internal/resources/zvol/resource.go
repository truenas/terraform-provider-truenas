// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
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

	payload := plan.apiPayload()
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

	// pool.dataset.create returns before udev has created the zvol's
	// /dev/zvol/<name> device node, so a dependent resource created in the same
	// apply — an iSCSI extent (disk = zvol/<name>) or an NVMe-oF namespace — can
	// fail its server-side device-exists check with "Device ... does not exist"
	// and only succeed on a re-apply once the node has appeared. Block here until
	// the node is present so every consumer is safe. (#40)
	waitForZvolDevice(ctx, r.client, plan.Name.ValueString())

	responseToModel(&apiResp, &plan.ZvolModel)
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
func waitForZvolDevice(ctx context.Context, c deviceStater, name string) {
	devPath := "/dev/zvol/" + name
	deadline := time.Now().Add(zvolDeviceWaitTimeout)
	for {
		if _, err := c.CallRead(ctx, "filesystem.stat", devPath); err == nil {
			return
		}
		if ctx.Err() != nil || time.Now().After(deadline) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(zvolDevicePollInterval):
		}
	}
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

	payload := plan.updateAPIPayload()
	// Skip the API call when the payload is what prior state would send: only
	// state-only attributes changed (force_destroy, sparse, a letter-case
	// change). Sending it anyway would rewrite properties for nothing.
	if !reflect.DeepEqual(payload, state.updateAPIPayload()) {
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
	state.ForceDestroy = types.BoolValue(false)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

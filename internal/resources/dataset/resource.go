// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
)

var _ resource.Resource = &DatasetResource{}
var _ resource.ResourceWithImportState = &DatasetResource{}
var _ resource.ResourceWithIdentity = &DatasetResource{}
var _ resource.ResourceWithValidateConfig = &DatasetResource{}

// ValidateConfig rejects configurations that set `encryption` together with
// `inherit_encryption = true`. With inheritance the parent dataset determines
// whether this dataset is encrypted, so an explicit encryption value is
// ambiguous and, when it disagrees with the parent, produced an "inconsistent
// result after apply" error at create (#31). Failing here gives a clear message
// at plan time instead.
func (r *DatasetResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg DatasetModel
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
			"encryption cannot be set when inherit_encryption = true: the dataset inherits "+
				"its encryption from the parent, which determines the value. Remove encryption, "+
				"or set inherit_encryption = false to manage encryption on this dataset.",
		)
	}
}

type DatasetResource struct {
	client *client.Client
}

func NewResource() resource.Resource { return &DatasetResource{} }

func (r *DatasetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataset"
}

func (r *DatasetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *DatasetResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.StringIDIdentitySchema()
}

func (r *DatasetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DatasetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DatasetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := plan.apiPayload()
	// encryption_passphrase / encryption_key are write-only: read them from the
	// config (they are null in the plan/state) and inject into encryption_options.
	var cfg DatasetModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	injectEncryptionSecrets(payload, &cfg)
	// TrueNAS 27.0 removed encryption_options.algorithm (the algorithm is fixed
	// server-side); sending it there fails with "Extra inputs are not permitted".
	// The algorithm still reads back via the computed encryption_algorithm.
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
		resp.Diagnostics.AddError("Create dataset failed", err.Error())
		return
	}

	// Read back via get_instance for canonical state (create response omits some properties).
	raw, err := r.client.CallRead(ctx, "pool.dataset.get_instance", plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read after create failed", err.Error())
		return
	}

	var apiResp apiResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse read response", err.Error())
		return
	}

	resp.Diagnostics.Append(r.responseToModel(&apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DatasetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DatasetModel
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
		resp.Diagnostics.AddError("Read dataset failed", err.Error())
		return
	}

	var apiResp apiResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse read response", err.Error())
		return
	}

	resp.Diagnostics.Append(r.responseToModel(&apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DatasetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DatasetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := plan.updateAPIPayload()

	_, err := r.client.Call(ctx, "pool.dataset.update", plan.Name.ValueString(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update dataset failed", err.Error())
		return
	}

	// Re-read to get computed fields
	raw, err := r.client.CallRead(ctx, "pool.dataset.get_instance", plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read after update failed", err.Error())
		return
	}

	var apiResp apiResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse update response", err.Error())
		return
	}

	resp.Diagnostics.Append(r.responseToModel(&apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DatasetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DatasetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.CallJob(ctx, "pool.dataset.delete", state.Name.ValueString(),
		map[string]any{"recursive": true})
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete dataset failed", err.Error())
	}
}

func (r *DatasetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by dataset name (e.g. "terraform import truenas_dataset.example tank/mydata")
	id, idDiags := listing.ImportString(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state DatasetModel
	state.Name = types.StringValue(id)
	state.ID = types.StringValue(id)

	raw, err := r.client.CallRead(ctx, "pool.dataset.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import dataset failed", err.Error())
		return
	}

	var apiResp apiResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	resp.Diagnostics.Append(r.responseToModel(&apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DatasetResource) responseToModel(api *apiResponse, m *DatasetModel) diag.Diagnostics {
	m.ID = types.StringValue(api.Name)
	m.Name = types.StringValue(api.Name)
	m.Type = preserveCase(m.Type, api.Type)
	m.MountPoint = types.StringValue(api.MountPoint)
	m.Encrypted = types.BoolValue(api.Encrypted)
	m.Encryption = types.BoolValue(api.Encrypted)
	m.Locked = types.BoolValue(api.Locked)
	if api.EncryptionAlgorithm.Value != nil && *api.EncryptionAlgorithm.Value != "" {
		m.EncryptionAlgorithm = types.StringValue(*api.EncryptionAlgorithm.Value)
	} else {
		m.EncryptionAlgorithm = types.StringNull()
	}
	if api.KeyFormat.Value != nil && *api.KeyFormat.Value != "" {
		m.KeyFormat = types.StringValue(*api.KeyFormat.Value)
	} else {
		m.KeyFormat = types.StringNull()
	}
	// inherit_encryption is reconciled from encryption_root: a dataset whose
	// encryption key is owned by an ancestor (root != self) inherits it; one that
	// owns its key (root == self) does not; an unencrypted dataset is not
	// inheriting encryption. Computed, so it round-trips on refresh even when the
	// user never set it (#32) and an inherited dataset created with
	// encryption=false does not read back a contradictory value (#31).
	m.InheritEncryption = types.BoolValue(api.Encrypted && api.EncryptionRoot != nil && *api.EncryptionRoot != api.Name)
	// encryption_generate_key and the write-only passphrase/key are not returned
	// by the API; keep the config/plan values.
	m.Pool = types.StringValue(api.Pool)
	// Source-aware compression: report "INHERIT" when it is not set LOCAL
	// (inherited/default), so compression = "inherit" round-trips instead of
	// resolving to the inherited value and failing with an inconsistent result. (#38)
	if api.Compression.Source == "LOCAL" {
		m.Compression = preserveCompression(m.Compression, api.Compression.Parsed)
	} else {
		m.Compression = preserveCase(m.Compression, "INHERIT")
	}
	// acltype is source-aware like compression: a non-LOCAL (inherited/default)
	// acltype reads back "INHERIT" so acltype = "inherit" round-trips instead of
	// resolving to the pool default (e.g. "posix") and failing with an
	// inconsistent result. (#38 class)
	if api.AClType.Source == "LOCAL" {
		m.AClType = preserveCase(m.AClType, api.AClType.Parsed)
	} else {
		m.AClType = preserveCase(m.AClType, "INHERIT")
	}
	m.Comments = types.StringValue(api.UserProperties.Comments.Value)
	// ShareType is write-only (not returned by API); preserve plan/state value as-is.

	if api.Quota.Parsed != nil {
		m.Quota = types.Int64Value(*api.Quota.Parsed)
	} else {
		m.Quota = types.Int64Value(0)
	}
	if api.RefQuota.Parsed != nil {
		m.RefQuota = types.Int64Value(*api.RefQuota.Parsed)
	} else {
		m.RefQuota = types.Int64Value(0)
	}
	if api.Reservation.Parsed != nil {
		m.Reservation = types.Int64Value(*api.Reservation.Parsed)
	} else {
		m.Reservation = types.Int64Value(0)
	}
	m.VolSize = types.Int64Value(api.VolSize.Parsed)

	// Source-aware ZFS tuning properties (coverage audit): record a value only when the
	// property is set LOCAL on this dataset; inherited/default reads back null.
	m.ACLMode = localStringOrInherit(api.ACLModeP)
	m.ATime = localStringOrInherit(api.ATimeP)
	m.Exec = localStringOrInherit(api.ExecP)
	m.ReadOnly = localStringOrInherit(api.ReadOnlyP)
	m.Sync = localStringOrInherit(api.SyncP)
	m.Checksum = localStringOrInherit(api.ChecksumP)
	m.Snapdir = localStringOrInherit(api.SnapdirP)
	m.Dedup = localStringOrInherit(api.DedupP)
	m.RecordSize = localString(api.RecordSizeP)
	m.Copies = localInt(api.CopiesP)
	m.SpecialSmallBlockSize = localInt(api.SSBSP)
	m.RefReservation = localInt(api.RefResP)

	// xattr is read-only: expose the effective value (not source-aware, since
	// it is never written).
	if api.XAttrP.Value != nil {
		m.XAttr = types.StringValue(*api.XAttrP.Value)
	} else {
		m.XAttr = types.StringNull()
	}
	return nil
}

// preserveCase returns current if it matches apiVal case-insensitively (preserving
// the user's chosen casing), or a lowercased apiVal otherwise (drift or first read).
func preserveCase(current types.String, apiVal string) types.String {
	if current.IsNull() || current.IsUnknown() {
		return types.StringValue(strings.ToLower(apiVal))
	}
	if strings.EqualFold(current.ValueString(), apiVal) {
		return current
	}
	return types.StringValue(strings.ToLower(apiVal))
}

// compressionCanon folds a compression value to the form ZFS actually stores,
// so an input alias the API canonicalizes away still round-trips. ZFS treats
// zstd-fast-1 as plain zstd-fast (level 1 is the default fast level) and reports
// it back as "zstd-fast"; without folding, compression = "ZSTD-FAST-1" would
// read back "zstd-fast" and fail with an inconsistent result. Every other level
// (zstd-fast-10, zstd-5, gzip-9) is stored verbatim. Case-insensitive.
func compressionCanon(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	if s == "zstd-fast-1" {
		return "zstd-fast"
	}
	return s
}

// preserveCompression is preserveCase for the compression property: current is
// kept when it is the same compression as apiVal under ZFS's own folding
// (compressionCanon), not merely equal-fold, so an alias like ZSTD-FAST-1 that
// the API reports back as ZSTD-FAST still round-trips. Real drift (a different
// algorithm) still overwrites state with the lower-cased API value.
func preserveCompression(current types.String, apiVal string) types.String {
	if current.IsNull() || current.IsUnknown() {
		return types.StringValue(strings.ToLower(apiVal))
	}
	if compressionCanon(current.ValueString()) == compressionCanon(apiVal) {
		return current
	}
	return types.StringValue(strings.ToLower(apiVal))
}

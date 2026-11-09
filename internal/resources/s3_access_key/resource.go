// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package s3_access_key

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
	"github.com/truenas/terraform-provider-truenas/internal/resources/s3_config"
)

var accessKeyPattern = regexp.MustCompile(`^[A-Z0-9]{16,128}$`)

var _ resource.Resource = &S3AccessKeyResource{}
var _ resource.ResourceWithImportState = &S3AccessKeyResource{}
var _ resource.ResourceWithIdentity = &S3AccessKeyResource{}

// S3AccessKeyResource implements truenas_s3_access_key.
type S3AccessKeyResource struct{ client *client.Client }

// NewResource returns a new S3AccessKeyResource.
func NewResource() resource.Resource { return &S3AccessKeyResource{} }

func (r *S3AccessKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_access_key"
}

func (r *S3AccessKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *S3AccessKeyResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.IntIDIdentitySchema()
}

func (r *S3AccessKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

// errText maps a missing S3 API to the clear message.
func errText(err error) string {
	if s3_config.IsUnavailable(err) {
		return s3_config.UnavailableMessage
	}
	return err.Error()
}

func (r *S3AccessKeyResource) getInstance(ctx context.Context, id int64) (*accessKeyAPI, error) {
	raw, err := r.client.CallRead(ctx, "s3.accesskey.get_instance", id)
	if err != nil {
		return nil, err
	}
	var api accessKeyAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

func (r *S3AccessKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, cfg S3AccessKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	secretWO := ""
	if !cfg.SecretWO.IsNull() && !cfg.SecretWO.IsUnknown() {
		secretWO = cfg.SecretWO.ValueString()
	}
	payload, d := plan.createPayload(secretWO)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	raw, err := r.client.Call(ctx, "s3.accesskey.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create S3 access key failed", errText(err))
		return
	}
	var api accessKeyAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}
	resp.Diagnostics.Append(responseToModel(&api, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// A generated secret is stored once, from this response; a supplied one
	// (secret_wo) never reaches state.
	plan.Secret = types.StringNull()
	if secretWO == "" && api.Secret != nil && *api.Secret != "" {
		plan.Secret = types.StringValue(*api.Secret)
	}
	plan.SecretWO = types.StringNull()
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3AccessKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state S3AccessKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	api, err := r.getInstance(ctx, state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read S3 access key failed", errText(err))
		return
	}
	// responseToModel leaves Secret as it is in state (never re-read).
	resp.Diagnostics.Append(responseToModel(api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *S3AccessKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state S3AccessKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	rotate := !plan.RotateVersion.Equal(state.RotateVersion) && !plan.RotateVersion.IsNull()
	payload, d := plan.updatePayload(rotate)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	raw, err := r.client.Call(ctx, "s3.accesskey.update", plan.ID.ValueInt64(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update S3 access key failed", errText(err))
		return
	}
	var api accessKeyAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse update response", err.Error())
		return
	}
	resp.Diagnostics.Append(responseToModel(&api, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// plan.Secret carries the state value (UseStateForUnknown); a rotation
	// replaces it with the newly generated secret.
	if rotate {
		plan.Secret = types.StringNull()
		if api.Secret != nil && *api.Secret != "" {
			plan.Secret = types.StringValue(*api.Secret)
		}
	}
	plan.SecretWO = types.StringNull()
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3AccessKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state S3AccessKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.Call(ctx, "s3.accesskey.delete", state.ID.ValueInt64()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete S3 access key failed", errText(err))
	}
}

func (r *S3AccessKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, idDiags := listing.ImportInt64(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	api, err := r.getInstance(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Import S3 access key failed", errText(err))
		return
	}
	var state S3AccessKeyModel
	resp.Diagnostics.Append(responseToModel(api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// The secret is deliberately not imported (it would land in state from a
	// read); secret_wo / versions are configuration-only.
	state.Secret = types.StringNull()
	state.SecretWO = types.StringNull()
	state.SecretWOVersion = types.Int64Null()
	state.RotateVersion = types.Int64Null()
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, id)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

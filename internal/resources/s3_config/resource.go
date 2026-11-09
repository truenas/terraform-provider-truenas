// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package s3_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
)

var _ resource.Resource = &S3ConfigResource{}
var _ resource.ResourceWithImportState = &S3ConfigResource{}
var _ resource.ResourceWithIdentity = &S3ConfigResource{}

// S3ConfigResource implements the truenas_s3_config singleton resource.
type S3ConfigResource struct{ client *client.Client }

// NewResource returns a new S3ConfigResource.
func NewResource() resource.Resource { return &S3ConfigResource{} }

// stringTypeAlias returns the string element type (kept in a function so the
// schema file stays free of a types import cycle in tests).
func stringTypeAlias() attr.Type { return types.StringType }

func (r *S3ConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_config"
}

func (r *S3ConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *S3ConfigResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.StringIDIdentitySchema()
}

func (r *S3ConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// FetchConfig calls s3.config and unmarshals the response.
func FetchConfig(ctx context.Context, c *client.Client) (*s3ConfigAPI, error) {
	raw, err := c.CallRead(ctx, "s3.config")
	if err != nil {
		return nil, err
	}
	var api s3ConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

func (r *S3ConfigResource) apply(ctx context.Context, plan, prior *S3ConfigModel, verb string) diag.Diagnostics {
	var diags diag.Diagnostics
	payload, d := plan.updatePayload(ctx, prior)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	if _, err := r.client.Call(ctx, "s3.update", payload); err != nil {
		diags.AddError(verb+" S3 configuration failed", wrapUnavailable(err))
		return diags
	}
	api, err := FetchConfig(ctx, r.client)
	if err != nil {
		diags.AddError("Read-back after "+verb+" failed", wrapUnavailable(err))
		return diags
	}
	diags.Append(responseToModel(ctx, api, plan)...)
	return diags
}

func (r *S3ConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan S3ConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.apply(ctx, &plan, nil, "Create")...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, s3ConfigResourceID)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3ConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state S3ConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Singleton: s3.config always exists on a release that has the S3 API,
	// so Read never removes the resource from state.
	api, err := FetchConfig(ctx, r.client)
	if err != nil {
		resp.Diagnostics.AddError("Read S3 configuration failed", wrapUnavailable(err))
		return
	}
	resp.Diagnostics.Append(responseToModel(ctx, api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, s3ConfigResourceID)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *S3ConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state S3ConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.apply(ctx, &plan, &state, "Update")...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, s3ConfigResourceID)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete makes no client calls: removing this resource from state never
// rewrites the box's S3 configuration.
func (r *S3ConfigResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(deleteWarningDiagnostics()...)
}

func (r *S3ConfigResource) ImportState(ctx context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Any import id is accepted and normalized to the singleton id; Read
	// fills the rest from s3.config.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), s3ConfigResourceID)...)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, s3ConfigResourceID)...)
}

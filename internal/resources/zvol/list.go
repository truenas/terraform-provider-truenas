// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
	"github.com/truenas/terraform-provider-truenas/internal/resources/dataset"
)

var _ list.ListResource = &ZvolListResource{}
var _ list.ListResourceWithConfigure = &ZvolListResource{}

// ZvolListResource implements the truenas_zvol list resource (terraform query).
type ZvolListResource struct{ client *client.Client }

// NewListResource returns a new ZvolListResource.
func NewListResource() list.ListResource { return &ZvolListResource{} }

func (l *ZvolListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zvol"
}

func (l *ZvolListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{}
}

// Configure satisfies list.ListResourceWithConfigure, whose Configure method
// signature uses resource.ConfigureRequest/resource.ConfigureResponse (not
// list.ConfigureRequest/list.ConfigureResponse).
func (l *ZvolListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	l.client = c
}

// List queries pool.dataset.query (shared with the dataset resource) filtered
// to type=VOLUME, so only zvols are listed here -- the complement of
// dataset's own list.go, which filters to type=FILESYSTEM.
func (l *ZvolListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	listing.StreamCollectionFiltered(ctx, l.client, "pool.dataset.query", [][]any{{"type", "=", "VOLUME"}}, req, stream, l.mapRow)
}

func (l *ZvolListResource) mapRow(ctx context.Context, req list.ListRequest, raw json.RawMessage) list.ListResult {
	res := req.NewListResult(ctx)
	var api zvolAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		res.Diagnostics.AddError("Parse zvol row", err.Error())
		return res
	}
	res.DisplayName = api.Name
	res.Diagnostics.Append(listing.SetIdentity(ctx, res.Identity, api.Name)...)
	if req.IncludeResource {
		var m zvolResourceModel
		responseToModel(&api, &m.ZvolModel)
		m.Sparse = types.BoolValue(sparseFromAPI(&api))
		m.ForceDestroy = dataset.ForceDestroyOrFalse(m.ForceDestroy)
		res.Diagnostics.Append(res.Resource.Set(ctx, &m)...)
	}
	return res
}

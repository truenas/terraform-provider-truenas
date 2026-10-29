// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
)

var _ list.ListResource = &DatasetListResource{}
var _ list.ListResourceWithConfigure = &DatasetListResource{}

// DatasetListResource implements the truenas_dataset list resource (terraform query).
type DatasetListResource struct{ client *client.Client }

// NewListResource returns a new DatasetListResource.
func NewListResource() list.ListResource { return &DatasetListResource{} }

func (l *DatasetListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataset"
}

func (l *DatasetListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{}
}

// Configure satisfies list.ListResourceWithConfigure, whose Configure method
// signature uses resource.ConfigureRequest/resource.ConfigureResponse (not
// list.ConfigureRequest/list.ConfigureResponse).
func (l *DatasetListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (l *DatasetListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	listing.StreamCollectionFiltered(ctx, l.client, "pool.dataset.query", [][]any{{"type", "=", "FILESYSTEM"}}, req, stream, l.mapRow)
}

func (l *DatasetListResource) mapRow(ctx context.Context, req list.ListRequest, raw json.RawMessage) list.ListResult {
	res := req.NewListResult(ctx)
	var api apiResponse
	if err := json.Unmarshal(raw, &api); err != nil {
		res.Diagnostics.AddError("Parse dataset row", err.Error())
		return res
	}
	res.DisplayName = api.Name
	res.Diagnostics.Append(listing.SetIdentity(ctx, res.Identity, api.Name)...)
	if req.IncludeResource {
		var m datasetResourceModel
		var dr DatasetResource
		res.Diagnostics.Append(dr.responseToModel(&api, &m.DatasetModel)...)
		m.ForceDestroy = ForceDestroyOrFalse(m.ForceDestroy)
		res.Diagnostics.Append(res.Resource.Set(ctx, &m)...)
	}
	return res
}

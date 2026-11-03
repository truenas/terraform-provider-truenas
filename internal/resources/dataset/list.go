// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
	filter := [][]any{{"type", "=", "FILESYSTEM"}}
	// The identity is the ZFS guid, which the default row set does not carry
	// (extra.properties replaces that set), so the guids come from a second
	// query joined on name.
	guids, err := GuidsByName(ctx, l.client, filter)
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError("List query failed", fmt.Sprintf("pool.dataset.query guid: %v", err))
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}
	listing.StreamCollectionFiltered(ctx, l.client, "pool.dataset.query", filter, req, stream,
		func(ctx context.Context, req list.ListRequest, raw json.RawMessage) list.ListResult {
			return l.mapRow(ctx, req, raw, guids)
		})
}

func (l *DatasetListResource) mapRow(ctx context.Context, req list.ListRequest, raw json.RawMessage, guids map[string]string) list.ListResult {
	res := req.NewListResult(ctx)
	var api apiResponse
	if err := json.Unmarshal(raw, &api); err != nil {
		res.Diagnostics.AddError("Parse dataset row", err.Error())
		return res
	}
	res.DisplayName = api.Name
	res.Diagnostics.Append(SetGuidIdentity(ctx, res.Identity, guids[api.Name])...)
	if req.IncludeResource {
		var m datasetResourceModel
		var dr DatasetResource
		res.Diagnostics.Append(dr.responseToModel(&api, &m.DatasetModel)...)
		m.Guid = types.StringValue(guids[api.Name])
		m.ForceDestroy = ForceDestroyOrFalse(m.ForceDestroy)
		res.Diagnostics.Append(res.Resource.Set(ctx, &m)...)
	}
	return res
}

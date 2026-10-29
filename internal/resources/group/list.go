// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package group

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

var _ list.ListResource = &GroupListResource{}
var _ list.ListResourceWithConfigure = &GroupListResource{}

// GroupListResource implements the truenas_group list resource (terraform query).
type GroupListResource struct{ client *client.Client }

// NewListResource returns a new GroupListResource.
func NewListResource() list.ListResource { return &GroupListResource{} }

func (l *GroupListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (l *GroupListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{}
}

// Configure satisfies list.ListResourceWithConfigure. NOTE: its signature
// uses resource.ConfigureRequest/resource.ConfigureResponse (not
// list.ConfigureRequest/list.ConfigureResponse) — see
// list.ListResourceWithConfigure in terraform-plugin-framework@v1.19.0.
func (l *GroupListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (l *GroupListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	listing.StreamCollection(ctx, l.client, "group.query", req, stream, l.mapRow)
}

func (l *GroupListResource) mapRow(ctx context.Context, req list.ListRequest, raw json.RawMessage) list.ListResult {
	res := req.NewListResult(ctx)
	var api groupAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		res.Diagnostics.AddError("Parse group row", err.Error())
		return res
	}
	res.DisplayName = api.Group
	res.Diagnostics.Append(listing.SetIdentity(ctx, res.Identity, api.ID)...)
	if req.IncludeResource {
		var m GroupModel
		res.Diagnostics.Append(responseToModel(ctx, &api, nil, &m)...)
		res.Diagnostics.Append(res.Resource.Set(ctx, &m)...)
	}
	return res
}

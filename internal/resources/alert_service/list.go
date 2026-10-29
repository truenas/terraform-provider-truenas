// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package alert_service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

var _ list.ListResource = &AlertServiceListResource{}
var _ list.ListResourceWithConfigure = &AlertServiceListResource{}

// AlertServiceListResource implements the truenas_alert_service list resource (terraform query).
type AlertServiceListResource struct{ client *client.Client }

// NewListResource returns a new AlertServiceListResource.
func NewListResource() list.ListResource { return &AlertServiceListResource{} }

func (l *AlertServiceListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alert_service"
}

func (l *AlertServiceListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{}
}

// Configure satisfies list.ListResourceWithConfigure. NOTE: its signature
// uses resource.ConfigureRequest/resource.ConfigureResponse (not
// list.ConfigureRequest/list.ConfigureResponse) — see
// list.ListResourceWithConfigure in terraform-plugin-framework@v1.19.0.
func (l *AlertServiceListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (l *AlertServiceListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	listing.StreamCollection(ctx, l.client, "alertservice.query", req, stream, l.mapRow)
}

func (l *AlertServiceListResource) mapRow(ctx context.Context, req list.ListRequest, raw json.RawMessage) list.ListResult {
	res := req.NewListResult(ctx)
	var api alertServiceAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		res.Diagnostics.AddError("Parse alert service row", err.Error())
		return res
	}
	res.DisplayName = api.Name
	res.Diagnostics.Append(listing.SetIdentity(ctx, res.Identity, api.ID)...)
	if req.IncludeResource {
		var m AlertServiceModel
		responseToModel(&api, &m)
		// responseToModel deliberately leaves Attributes untouched; populate
		// it from the API response here, mirroring ImportState.
		attrsJSON, diags := apiAttributesJSON(&api)
		res.Diagnostics.Append(diags...)
		if res.Diagnostics.HasError() {
			return res
		}
		m.Attributes = normalize.NewJSONSubsetValue(attrsJSON)
		res.Diagnostics.Append(res.Resource.Set(ctx, &m)...)
	}
	return res
}

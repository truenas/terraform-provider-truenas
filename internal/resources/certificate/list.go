// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package certificate

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
)

var _ list.ListResource = &CertificateListResource{}
var _ list.ListResourceWithConfigure = &CertificateListResource{}

// CertificateListResource implements the truenas_certificate list resource (terraform query).
type CertificateListResource struct{ client *client.Client }

// NewListResource returns a new CertificateListResource.
func NewListResource() list.ListResource { return &CertificateListResource{} }

func (l *CertificateListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_certificate"
}

func (l *CertificateListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{}
}

// Configure satisfies list.ListResourceWithConfigure. NOTE: its signature
// uses resource.ConfigureRequest/resource.ConfigureResponse (not
// list.ConfigureRequest/list.ConfigureResponse) — see
// list.ListResourceWithConfigure in terraform-plugin-framework@v1.19.0.
func (l *CertificateListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (l *CertificateListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	listing.StreamCollection(ctx, l.client, "certificate.query", req, stream, l.mapRow)
}

func (l *CertificateListResource) mapRow(ctx context.Context, req list.ListRequest, raw json.RawMessage) list.ListResult {
	res := req.NewListResult(ctx)
	var api certificateAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		res.Diagnostics.AddError("Parse certificate row", err.Error())
		return res
	}
	res.DisplayName = api.Name
	res.Diagnostics.Append(listing.SetIdentity(ctx, res.Identity, api.ID)...)
	if req.IncludeResource {
		// dns_mapping is never returned by the API and has no zero value for
		// a typed Map (see ImportState); type it null up front so
		// responseToModel's untouched field still converts cleanly.
		m := CertificateModel{DNSMapping: types.MapNull(types.Int64Type), CreateType: importedCreateType(&api), StorePrivatekey: types.BoolValue(true)}
		res.Diagnostics.Append(responseToModel(ctx, &api, &m)...)
		res.Diagnostics.Append(res.Resource.Set(ctx, &m)...)
	}
	return res
}

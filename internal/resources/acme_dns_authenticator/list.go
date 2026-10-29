// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acme_dns_authenticator

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

var _ list.ListResource = &AcmeDnsAuthenticatorListResource{}
var _ list.ListResourceWithConfigure = &AcmeDnsAuthenticatorListResource{}

// AcmeDnsAuthenticatorListResource implements the
// truenas_acme_dns_authenticator list resource (terraform query).
type AcmeDnsAuthenticatorListResource struct{ client *client.Client }

// NewListResource returns a new AcmeDnsAuthenticatorListResource.
func NewListResource() list.ListResource { return &AcmeDnsAuthenticatorListResource{} }

func (l *AcmeDnsAuthenticatorListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_acme_dns_authenticator"
}

func (l *AcmeDnsAuthenticatorListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{}
}

// Configure satisfies list.ListResourceWithConfigure. NOTE: its signature
// uses resource.ConfigureRequest/resource.ConfigureResponse (not
// list.ConfigureRequest/list.ConfigureResponse) — see
// list.ListResourceWithConfigure in terraform-plugin-framework@v1.19.0.
func (l *AcmeDnsAuthenticatorListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (l *AcmeDnsAuthenticatorListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	listing.StreamCollection(ctx, l.client, "acme.dns.authenticator.query", req, stream, l.mapRow)
}

func (l *AcmeDnsAuthenticatorListResource) mapRow(ctx context.Context, req list.ListRequest, raw json.RawMessage) list.ListResult {
	res := req.NewListResult(ctx)
	var api acmeDnsAuthenticatorAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		res.Diagnostics.AddError("Parse ACME DNS authenticator row", err.Error())
		return res
	}
	res.DisplayName = api.Name
	res.Diagnostics.Append(listing.SetIdentity(ctx, res.Identity, api.ID)...)
	if req.IncludeResource {
		var m AcmeDnsAuthenticatorModel
		responseToModel(&api, &m)
		// responseToModel deliberately leaves Attributes untouched (see its
		// doc comment); populate it from the API response here, mirroring
		// ImportState.
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

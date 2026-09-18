// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package catalog_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &CatalogConfigDataSource{}

// CatalogConfigDataSource implements the truenas_catalog_config data source.
type CatalogConfigDataSource struct{ client *client.Client }

// NewDataSource returns a new CatalogConfigDataSource.
func NewDataSource() datasource.DataSource { return &CatalogConfigDataSource{} }

func (d *CatalogConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_catalog_config"
}

func (d *CatalogConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS app catalog configuration. Takes no arguments: there is " +
			"exactly one app catalog per TrueNAS system.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"catalog_config\".",
			},
			"label": dschema.StringAttribute{
				Computed:    true,
				Description: "The catalog's identifier/label (e.g. \"TRUENAS\").",
			},
			"location": dschema.StringAttribute{
				Computed:    true,
				Description: "The git repository URL or local filesystem path backing the catalog.",
			},
			"preferred_trains": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Train names preferred when browsing and installing applications from this catalog.",
			},
			"trains": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "All train names the catalog currently knows about (catalog.trains), independent " +
					"of which ones are preferred. Can be empty if the catalog has not yet been synced (probed " +
					"live: returned empty on a TrueNAS 25.10 box where Docker/apps had never been configured, " +
					"versus a full list of trains on a TrueNAS 26.0 box with apps running — see task-3-report.md).",
			},
		},
	}
}

func (d *CatalogConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data",
			fmt.Sprintf("expected *client.Client, got %T", req.ProviderData),
		)
		return
	}
	d.client = c
}

// Read calls both catalog.config and catalog.trains. The second call was
// judged trivially cheap enough to include (job:false, no arguments, same
// class of call as catalog.config itself) rather than omitted — see
// task-3-report.md for the full justification.
func (d *CatalogConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state CatalogConfigDataSourceModel

	rawConfig, err := d.client.CallRead(ctx, "catalog.config")
	if err != nil {
		resp.Diagnostics.AddError("Read catalog configuration failed", err.Error())
		return
	}
	var api catalogConfigAPI
	if err := json.Unmarshal(rawConfig, &api); err != nil {
		resp.Diagnostics.AddError("Parse catalog.config response", err.Error())
		return
	}

	rawTrains, err := d.client.CallRead(ctx, "catalog.trains")
	if err != nil {
		resp.Diagnostics.AddError("Read catalog trains failed", err.Error())
		return
	}
	var trains []string
	if err := json.Unmarshal(rawTrains, &trains); err != nil {
		resp.Diagnostics.AddError("Parse catalog.trains response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &api, trains, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &AppDataSource{}

// AppDataSource reads a TrueNAS app by name.
type AppDataSource struct{ client *client.Client }

// NewDataSource returns a new AppDataSource.
func NewDataSource() datasource.DataSource { return &AppDataSource{} }

func (d *AppDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app"
}

func (d *AppDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches the current state of a TrueNAS app by name.",
		Attributes: map[string]dschema.Attribute{
			"id":                dschema.StringAttribute{Computed: true, Description: "App name (used as Terraform ID)."},
			"name":              dschema.StringAttribute{Required: true, Description: "Application name."},
			"train":             dschema.StringAttribute{Computed: true, Description: "Catalog train: stable, community, enterprise."},
			"version":           dschema.StringAttribute{Computed: true, Description: "Installed app version."},
			"custom_app":        dschema.BoolAttribute{Computed: true, Description: "True for custom (compose-based) apps."},
			"state":             dschema.StringAttribute{Computed: true, Description: "Current app state (RUNNING, STOPPED, DEPLOYING, ...)."},
			"human_version":     dschema.StringAttribute{Computed: true},
			"upgrade_available": dschema.BoolAttribute{Computed: true},
		},
	}
}

func (d *AppDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *AppDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state AppDatasourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "app.query", [][]any{{"name", "=", state.Name.ValueString()}})
	if err != nil {
		resp.Diagnostics.AddError("Read app failed", err.Error())
		return
	}

	var results []appAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}
	if len(results) == 0 {
		resp.Diagnostics.AddError("App not found", fmt.Sprintf("app %q not found", state.Name.ValueString()))
		return
	}

	responseToDatasourceModel(&results[0], &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

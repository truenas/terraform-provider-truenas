// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package resilver_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &ResilverConfigDataSource{}

// ResilverConfigDataSource implements the truenas_resilver_config data
// source.
type ResilverConfigDataSource struct{ client *client.Client }

// NewDataSource returns a new ResilverConfigDataSource.
func NewDataSource() datasource.DataSource { return &ResilverConfigDataSource{} }

func (d *ResilverConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resilver_config"
}

func (d *ResilverConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS pool resilver priority schedule. Takes no arguments: " +
			"there is exactly one resilver schedule per TrueNAS system.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"resilver_config\".",
			},
			"begin": dschema.StringAttribute{
				Computed:    true,
				Description: "Time when the resilver operations window begins, 24-hour \"HH:MM\" format.",
			},
			"end": dschema.StringAttribute{
				Computed:    true,
				Description: "Time when the resilver operations window ends, 24-hour \"HH:MM\" format.",
			},
			"enabled": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether the resilver schedule is enabled.",
			},
			"weekday": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.Int64Type,
				Description: "Weekdays when resilver operations are allowed, crontab(5) values " +
					"1 (Monday) through 7 (Sunday).",
			},
		},
	}
}

func (d *ResilverConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ResilverConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state ResilverConfigDataSourceModel

	raw, err := d.client.CallRead(ctx, "pool.resilver.config")
	if err != nil {
		resp.Diagnostics.AddError("Read resilver configuration failed", err.Error())
		return
	}

	var api resilverConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse pool.resilver.config response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_dataset

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &SystemDatasetDataSource{}

// SystemDatasetDataSource implements the truenas_system_dataset data
// source.
type SystemDatasetDataSource struct{ client *client.Client }

// NewDataSource returns a new SystemDatasetDataSource.
func NewDataSource() datasource.DataSource { return &SystemDatasetDataSource{} }

func (d *SystemDatasetDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_system_dataset"
}

func (d *SystemDatasetDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS system dataset configuration (the dataset that holds " +
			"core system state such as logs, reporting, syslog, and samba4 data). Takes no arguments: there is " +
			"exactly one system dataset configuration per TrueNAS system.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"system_dataset\".",
			},
			"pool": dschema.StringAttribute{
				Computed:    true,
				Description: "Name of the pool the system dataset lives on.",
			},
			"basename": dschema.StringAttribute{
				Computed:    true,
				Description: "Base path of the system dataset (e.g. \"tank/.system\").",
			},
			"path": dschema.StringAttribute{
				Computed:    true,
				Description: "Filesystem path the system dataset is mounted at (e.g. \"/var/db/system\").",
			},
			"uuid": dschema.StringAttribute{
				Computed:    true,
				Description: "UUID of the system dataset.",
			},
			"pool_set": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether the system dataset pool has been explicitly set.",
			},
		},
	}
}

func (d *SystemDatasetDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SystemDatasetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state SystemDatasetDataSourceModel

	raw, err := d.client.CallRead(ctx, "systemdataset.config")
	if err != nil {
		resp.Diagnostics.AddError("Read system dataset configuration failed", err.Error())
		return
	}

	var api systemDatasetAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse systemdataset.config response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

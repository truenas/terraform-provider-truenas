// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container_device

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &ContainerDeviceDataSource{}

// ContainerDeviceDataSource implements the truenas_container_device data
// source.
type ContainerDeviceDataSource struct{ client *client.Client }

// NewDataSource returns a new ContainerDeviceDataSource.
func NewDataSource() datasource.DataSource { return &ContainerDeviceDataSource{} }

func (d *ContainerDeviceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_container_device"
}

func (d *ContainerDeviceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS container device by ID. Requires TrueNAS 26.0 or later — see the " +
			"truenas_container_device resource's schema description for the version-gate details.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.Int64Attribute{
				Required:    true,
				Description: "Numeric container device ID to look up.",
			},
			"container": dschema.Int64Attribute{
				Computed:    true,
				Description: "ID of the truenas_container this device belongs to.",
			},
			"attributes": dschema.StringAttribute{
				Computed:    true,
				Description: "JSON document of device attributes.",
			},
		},
	}
}

func (d *ContainerDeviceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ContainerDeviceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state ContainerDeviceDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	version, err := d.client.ServerVersion(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Version probe failed", err.Error())
		return
	}
	resp.Diagnostics.Append(versionGateDiagnostics(version)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "container.device.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError(
				"Container device not found",
				fmt.Sprintf("No container device with ID %d was found.", state.ID.ValueInt64()),
			)
			return
		}
		resp.Diagnostics.AddError("Read container device failed", err.Error())
		return
	}

	var apiResp containerDeviceAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(&apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

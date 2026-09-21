// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_device

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &VMDeviceDataSource{}

// VMDeviceDataSource implements the truenas_vm_device data source.
type VMDeviceDataSource struct{ client *client.Client }

// NewDataSource returns a new VMDeviceDataSource.
func NewDataSource() datasource.DataSource { return &VMDeviceDataSource{} }

func (d *VMDeviceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm_device"
}

func (d *VMDeviceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS VM device by ID.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.Int64Attribute{
				Required:    true,
				Description: "Numeric VM device ID to look up.",
			},
			"vm": dschema.Int64Attribute{
				Computed:    true,
				Description: "ID of the VM this device belongs to.",
			},
			"attributes": dschema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "JSON document of device attributes.",
			},
			"order": dschema.Int64Attribute{
				Computed:    true,
				Description: "Boot/attach order.",
			},
		},
	}
}

func (d *VMDeviceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *VMDeviceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state VMDeviceDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "vm.device.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError(
				"VM device not found",
				fmt.Sprintf("No VM device with ID %d was found.", state.ID.ValueInt64()),
			)
			return
		}
		resp.Diagnostics.AddError("Read VM device failed", err.Error())
		return
	}

	var apiResp deviceAPI
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

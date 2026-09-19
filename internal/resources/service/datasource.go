// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &ServiceDataSource{}

// ServiceDataSource reads a TrueNAS service by name.
type ServiceDataSource struct{ client *client.Client }

// NewDataSource returns a new ServiceDataSource.
func NewDataSource() datasource.DataSource { return &ServiceDataSource{} }

func (d *ServiceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service"
}

func (d *ServiceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches the current state of a TrueNAS service by name.",
		Attributes: map[string]dschema.Attribute{
			"id":      dschema.StringAttribute{Computed: true, Description: "Service name (used as Terraform ID)."},
			"name":    dschema.StringAttribute{Required: true, Description: "Service name: nfs, cifs, ssh, ftp, iscsitarget, snmp, ups, nvmet, webshare."},
			"enabled": dschema.BoolAttribute{Computed: true, Description: "Whether the service starts automatically on boot."},
			"running": dschema.BoolAttribute{Computed: true, Description: "Whether the service is currently running."},
		},
	}
}

func (d *ServiceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ServiceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state ServiceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "service.query", [][]any{{"service", "=", state.Name.ValueString()}})
	if err != nil {
		resp.Diagnostics.AddError("Read service failed", err.Error())
		return
	}

	var results []serviceAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}
	if len(results) == 0 {
		resp.Diagnostics.AddError("Service not found", fmt.Sprintf("service %q not found", state.Name.ValueString()))
		return
	}

	responseToModel(&results[0], &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &DatasetDataSource{}

type DatasetDataSource struct {
	client *client.Client
}

func NewDataSource() datasource.DataSource { return &DatasetDataSource{} }

func (d *DatasetDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataset"
}

func (d *DatasetDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS dataset by name.",
		Attributes: map[string]dschema.Attribute{
			"id":          dschema.StringAttribute{Computed: true},
			"name":        dschema.StringAttribute{Required: true, Description: "Full dataset path, e.g. tank/mydata."},
			"type":        dschema.StringAttribute{Computed: true},
			"compression": dschema.StringAttribute{Computed: true},
			"acltype":     dschema.StringAttribute{Computed: true},
			"share_type":  dschema.StringAttribute{Computed: true},
			"comments":    dschema.StringAttribute{Computed: true},
			"quota":       dschema.Int64Attribute{Computed: true},
			"refquota":    dschema.Int64Attribute{Computed: true},
			"reservation": dschema.Int64Attribute{Computed: true},
			"volsize":     dschema.Int64Attribute{Computed: true},
			"mountpoint":  dschema.StringAttribute{Computed: true},
			"encrypted":   dschema.BoolAttribute{Computed: true},
			"pool":        dschema.StringAttribute{Computed: true},
		},
	}
}

func (d *DatasetDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DatasetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state DatasetModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "pool.dataset.get_instance", state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read dataset failed", err.Error())
		return
	}

	var apiResp apiResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	r := &DatasetResource{}
	resp.Diagnostics.Append(r.responseToModel(&apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

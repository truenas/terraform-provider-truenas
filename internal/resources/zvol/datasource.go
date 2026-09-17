// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &ZvolDataSource{}

type ZvolDataSource struct {
	client *client.Client
}

func NewDataSource() datasource.DataSource { return &ZvolDataSource{} }

func (d *ZvolDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zvol"
}

func (d *ZvolDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS zvol by name.",
		Attributes: map[string]dschema.Attribute{
			"id":           dschema.StringAttribute{Computed: true},
			"name":         dschema.StringAttribute{Required: true, Description: "Full zvol path, e.g. tank/myvol."},
			"volsize":      dschema.Int64Attribute{Computed: true},
			"volblocksize": dschema.Int64Attribute{Computed: true},
			"compression":  dschema.StringAttribute{Computed: true},
			"sync":         dschema.StringAttribute{Computed: true},
			"dedup":        dschema.StringAttribute{Computed: true},
			"sparse":       dschema.BoolAttribute{Computed: true},
			"comments":     dschema.StringAttribute{Computed: true},
			"pool":         dschema.StringAttribute{Computed: true},
			"encrypted":    dschema.BoolAttribute{Computed: true},
		},
	}
}

func (d *ZvolDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ZvolDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state ZvolModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "pool.dataset.get_instance", state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read zvol failed", err.Error())
		return
	}

	var apiResp zvolAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	responseToModel(&apiResp, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

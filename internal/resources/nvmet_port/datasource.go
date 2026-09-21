// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_port

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &NVMetPortDataSource{}

// NVMetPortDataSource implements the truenas_nvmet_port data source.
type NVMetPortDataSource struct{ client *client.Client }

// NewDataSource returns a new NVMetPortDataSource.
func NewDataSource() datasource.DataSource { return &NVMetPortDataSource{} }

func (d *NVMetPortDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nvmet_port"
}

func (d *NVMetPortDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS NVMe-oF transport port by ID.",
		Attributes: map[string]dschema.Attribute{
			"id":               dschema.Int64Attribute{Required: true, Description: "Numeric NVMe-oF port ID to look up."},
			"addr_trtype":      dschema.StringAttribute{Computed: true, Description: "TCP or RDMA."},
			"addr_traddr":      dschema.StringAttribute{Computed: true, Description: "Transport address (IP address) the port listens on."},
			"addr_trsvcid":     dschema.Int64Attribute{Computed: true, Description: "Transport service ID (TCP/RDMA port number)."},
			"enabled":          dschema.BoolAttribute{Computed: true, Description: "Whether the port is enabled."},
			"inline_data_size": dschema.Int64Attribute{Computed: true, Description: "Inline data size (bytes) for this port."},
			"max_queue_size":   dschema.Int64Attribute{Computed: true, Description: "Maximum queue size for this port."},
			"pi_enable":        dschema.BoolAttribute{Computed: true, Description: "Whether end-to-end data protection (PI) is enabled for this port."},
			"index":            dschema.Int64Attribute{Computed: true, Description: "Numeric index assigned by TrueNAS for this port."},
			"addr_adrfam":      dschema.StringAttribute{Computed: true, Description: "Address family (e.g. IPV4) derived from addr_traddr."},
		},
	}
}

func (d *NVMetPortDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *NVMetPortDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state NVMetPortDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "nvmet.port.get_instance", state.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read NVMe-oF port failed", err.Error())
		return
	}

	var apiResp nvmetPortAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse get_instance response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

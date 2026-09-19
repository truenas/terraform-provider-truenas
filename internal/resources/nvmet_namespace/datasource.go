// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_namespace

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &NVMetNamespaceDataSource{}

// NVMetNamespaceDataSource implements the truenas_nvmet_namespace data
// source.
type NVMetNamespaceDataSource struct{ client *client.Client }

// NewDataSource returns a new NVMetNamespaceDataSource.
func NewDataSource() datasource.DataSource { return &NVMetNamespaceDataSource{} }

func (d *NVMetNamespaceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nvmet_namespace"
}

func (d *NVMetNamespaceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS NVMe-oF namespace by device path.",
		Attributes: map[string]dschema.Attribute{
			"id":           dschema.Int64Attribute{Computed: true, Description: "Numeric NVMe-oF namespace ID."},
			"subsys_id":    dschema.Int64Attribute{Computed: true, Description: "ID of the NVMe-oF subsystem this namespace belongs to."},
			"device_path":  dschema.StringAttribute{Required: true, Description: "Path of the backing device to look up, e.g. \"zvol/tank/vms/vm1\"."},
			"device_type":  dschema.StringAttribute{Computed: true, Description: "Backing device type: ZVOL or FILE."},
			"enabled":      dschema.BoolAttribute{Computed: true, Description: "Whether the namespace is enabled."},
			"filesize":     dschema.Int64Attribute{Computed: true, Description: "Size in bytes of the backing file. Only applicable for FILE device_type."},
			"nsid":         dschema.Int64Attribute{Computed: true, Description: "Namespace ID within the subsystem."},
			"device_nguid": dschema.StringAttribute{Computed: true, Description: "NVMe Globally Unique Identifier for the namespace's backing device."},
			"device_uuid":  dschema.StringAttribute{Computed: true, Description: "UUID for the namespace's backing device."},
			"locked":       dschema.BoolAttribute{Computed: true, Description: "Whether the namespace's backing device is locked (e.g. an encrypted dataset)."},
		},
	}
}

func (d *NVMetNamespaceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *NVMetNamespaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state NVMetNamespaceDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Query by device_path: nvmet.namespace.query([["device_path","=","<path>"]])
	queryFilters := []any{[]any{"device_path", "=", state.DevicePath.ValueString()}}
	raw, err := d.client.CallRead(ctx, "nvmet.namespace.query", queryFilters)
	if err != nil {
		resp.Diagnostics.AddError("Query NVMe-oF namespaces failed", err.Error())
		return
	}

	var results []nvmetNamespaceAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"NVMe-oF namespace not found",
			fmt.Sprintf("No NVMe-oF namespace with device_path %q was found.", state.DevicePath.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

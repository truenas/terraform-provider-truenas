// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package docker_network

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &DockerNetworkDataSource{}

// DockerNetworkDataSource implements the truenas_docker_network data
// source. docker_network is a DATASOURCE-ONLY namespace: see schema.go's
// datasourceSchema doc comment for why there is no corresponding resource.
type DockerNetworkDataSource struct{ client *client.Client }

// NewDataSource returns a new DockerNetworkDataSource.
func NewDataSource() datasource.DataSource { return &DockerNetworkDataSource{} }

func (d *DockerNetworkDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_docker_network"
}

func (d *DockerNetworkDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceSchema()
}

func (d *DockerNetworkDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DockerNetworkDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state DockerNetworkDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Query by name: docker.network.query([["name", "=", "<name>"]])
	queryFilters := []any{[]any{"name", "=", state.Name.ValueString()}}
	raw, err := d.client.CallRead(ctx, "docker.network.query", queryFilters)
	if err != nil {
		resp.Diagnostics.AddError("Query Docker networks failed", err.Error())
		return
	}

	var results []dockerNetworkAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Docker network not found",
			fmt.Sprintf("No Docker network named %q was found.", state.Name.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

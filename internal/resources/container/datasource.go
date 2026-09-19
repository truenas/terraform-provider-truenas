// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &ContainerDataSource{}

// ContainerDataSource implements the truenas_container data source.
type ContainerDataSource struct{ client *client.Client }

// NewDataSource returns a new ContainerDataSource.
func NewDataSource() datasource.DataSource { return &ContainerDataSource{} }

func (d *ContainerDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_container"
}

func (d *ContainerDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Looks up an LXC container on TrueNAS by name (container.query). Requires " +
			"TrueNAS 26.0 or later (see the truenas_container resource's schema description for the version-gate " +
			"details). Note \"pool\" is recovered from the container's dataset path rather than returned " +
			"directly by the API; \"image\" is not exposed here at all — the API never echoes back the image a " +
			"container was created from (see truenas_container's schema description).",
		Attributes: map[string]dschema.Attribute{
			"id":                  dschema.Int64Attribute{Computed: true, Description: "Numeric container ID."},
			"uuid":                dschema.StringAttribute{Computed: true, Description: "Container UUID."},
			"name":                dschema.StringAttribute{Required: true, Description: "Container name to look up."},
			"pool":                dschema.StringAttribute{Computed: true, Description: "ZFS pool backing the container's root dataset, recovered from \"dataset\"."},
			"description":         dschema.StringAttribute{Computed: true},
			"autostart":           dschema.BoolAttribute{Computed: true},
			"cpuset":              dschema.StringAttribute{Computed: true},
			"time":                dschema.StringAttribute{Computed: true},
			"shutdown_timeout":    dschema.Int64Attribute{Computed: true},
			"init":                dschema.StringAttribute{Computed: true},
			"initdir":             dschema.StringAttribute{Computed: true},
			"initenv":             dschema.MapAttribute{Computed: true, ElementType: types.StringType},
			"inituser":            dschema.StringAttribute{Computed: true},
			"initgroup":           dschema.StringAttribute{Computed: true},
			"idmap":               dschema.StringAttribute{Computed: true, Description: "Idmap configuration as a JSON-encoded string."},
			"capabilities_policy": dschema.StringAttribute{Computed: true},
			"capabilities_state":  dschema.MapAttribute{Computed: true, ElementType: types.BoolType},
			"running":             dschema.BoolAttribute{Computed: true},
			"dataset":             dschema.StringAttribute{Computed: true, Description: "ZFS dataset backing the container's root filesystem."},
			"default_network":     dschema.StringAttribute{Computed: true},
			"status":              dschema.StringAttribute{Computed: true, Description: "Current container status: RUNNING, STOPPED."},
		},
	}
}

func (d *ContainerDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ContainerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state ContainerDataSourceModel
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

	queryFilters := []any{[]any{"name", "=", state.Name.ValueString()}}
	raw, err := d.client.CallRead(ctx, "container.query", queryFilters)
	if err != nil {
		resp.Diagnostics.AddError("Query containers failed", err.Error())
		return
	}

	var results []containerAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Container not found",
			fmt.Sprintf("No container named %q was found.", state.Name.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

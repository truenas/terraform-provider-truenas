// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container_image

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &ContainerImageDataSource{}

// ContainerImageDataSource implements the truenas_container_image data
// source. container_image is DATASOURCE-ONLY: see schema.go's
// datasourceSchema doc comment for why there is no corresponding resource.
type ContainerImageDataSource struct{ client *client.Client }

// NewDataSource returns a new ContainerImageDataSource.
func NewDataSource() datasource.DataSource { return &ContainerImageDataSource{} }

func (d *ContainerImageDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_container_image"
}

func (d *ContainerImageDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceSchema()
}

func (d *ContainerImageDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ContainerImageDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state ContainerImageDataSourceModel
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

	// container.image.query_registry takes NO arguments (probed live: a
	// query-filter argument is rejected as "Too many arguments") — it
	// always returns every image in the registry, so filtering by name
	// happens client-side (see findRegistryImage).
	raw, err := d.client.CallRead(ctx, "container.image.query_registry")
	if err != nil {
		resp.Diagnostics.AddError("Query container image registry failed", err.Error())
		return
	}

	var results []registryImageAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query_registry response", err.Error())
		return
	}

	name := state.Name.ValueString()
	entry, ok := findRegistryImage(results, name)
	if !ok {
		resp.Diagnostics.AddError(
			"Container image not found",
			fmt.Sprintf("No image named %q was found in the registry.", name),
		)
		return
	}

	versions := versionStrings(entry.Versions)
	versionsList, diags := types.ListValueFrom(ctx, types.StringType, versions)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.ID = types.StringValue(name)
	state.Versions = versionsList
	if latest, ok := latestVersion(versions); ok {
		state.LatestVersion = types.StringValue(latest)
	} else {
		state.LatestVersion = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

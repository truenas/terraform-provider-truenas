// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package docker_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &DockerConfigDataSource{}

// DockerConfigDataSource implements the truenas_docker_config data source.
type DockerConfigDataSource struct{ client *client.Client }

// NewDataSource returns a new DockerConfigDataSource.
func NewDataSource() datasource.DataSource { return &DockerConfigDataSource{} }

func (d *DockerConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_docker_config"
}

func (d *DockerConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS Docker service configuration. Takes no arguments: there " +
			"is exactly one Docker configuration per TrueNAS system.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"docker_config\".",
			},
			"enable_image_updates": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether TrueNAS periodically checks for and downloads updates to Docker images used by installed applications.",
			},
			"pool": dschema.StringAttribute{
				Computed:    true,
				Description: "ZFS storage pool backing Docker, or null if Docker has not been configured.",
			},
			"dataset": dschema.StringAttribute{
				Computed:    true,
				Description: "ZFS dataset TrueNAS created under \"pool\" for Docker data storage, or null if Docker is unconfigured.",
			},
			"nvidia": dschema.BoolAttribute{
				Computed: true,
				Description: "Whether NVIDIA GPU support is enabled for containers. Readable on every probed " +
					"release; writable (via the truenas_docker_config resource) only on TrueNAS 25.10 " +
					"and earlier.",
			},
			"address_pools": dschema.ListNestedAttribute{
				Computed:    true,
				Description: "Network address pools Docker allocates per-container-network subnets from.",
				NestedObject: dschema.NestedAttributeObject{
					Attributes: map[string]dschema.Attribute{
						"base": dschema.StringAttribute{Computed: true, Description: "Base network address with prefix for the pool."},
						"size": dschema.Int64Attribute{Computed: true, Description: "Subnet size for networks allocated from this pool."},
					},
				},
			},
			"cidr_v6": dschema.StringAttribute{
				Computed:    true,
				Description: "IPv6 CIDR block for Docker container networking.",
			},
			"registry_mirrors": dschema.ListNestedAttribute{
				Computed: true,
				Description: "Registry mirror URLs Docker pulls images through, unified across both probed " +
					"releases (see the resource schema description for the TrueNAS 25.10 wire-shape translation).",
				NestedObject: dschema.NestedAttributeObject{
					Attributes: map[string]dschema.Attribute{
						"url":      dschema.StringAttribute{Computed: true, Description: "Registry mirror URL."},
						"insecure": dschema.BoolAttribute{Computed: true, Description: "Whether this registry mirror uses an insecure (HTTP) connection."},
					},
				},
			},
		},
	}
}

func (d *DockerConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DockerConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state DockerConfigDataSourceModel

	raw, err := d.client.CallRead(ctx, "docker.config")
	if err != nil {
		resp.Diagnostics.AddError("Read Docker configuration failed", err.Error())
		return
	}

	var api dockerConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse docker.config response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

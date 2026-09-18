// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package boot_environment

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &BootEnvironmentDataSource{}

// BootEnvironmentDataSource reads a TrueNAS boot environment by name.
type BootEnvironmentDataSource struct{ client *client.Client }

// NewDataSource returns a new BootEnvironmentDataSource.
func NewDataSource() datasource.DataSource { return &BootEnvironmentDataSource{} }

func (d *BootEnvironmentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_boot_environment"
}

func (d *BootEnvironmentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches information about a TrueNAS boot environment by name.",
		Attributes: map[string]dschema.Attribute{
			"id":         dschema.StringAttribute{Computed: true, Description: "Boot environment name (used as Terraform ID)."},
			"name":       dschema.StringAttribute{Required: true, Description: "Boot environment name to look up."},
			"activated":  dschema.BoolAttribute{Computed: true, Description: "Whether this boot environment is activated (will be booted next)."},
			"keep":       dschema.BoolAttribute{Computed: true, Description: "Whether this boot environment is protected from automatic pruning."},
			"dataset":    dschema.StringAttribute{Computed: true, Description: "ZFS dataset backing this boot environment."},
			"active":     dschema.BoolAttribute{Computed: true, Description: "Whether this boot environment is currently booted."},
			"used_bytes": dschema.Int64Attribute{Computed: true, Description: "Space used by this boot environment, in bytes."},
		},
	}
}

func (d *BootEnvironmentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *BootEnvironmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config BootEnvironmentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := config.Name.ValueString()
	raw, err := d.client.CallRead(ctx, "boot.environment.query", [][]any{{"id", "=", name}})
	if err != nil {
		resp.Diagnostics.AddError("Read boot environment failed", err.Error())
		return
	}

	var results []bootEnvAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}
	if len(results) == 0 {
		resp.Diagnostics.AddError("Boot environment not found", fmt.Sprintf("boot environment %q not found", name))
		return
	}

	state := responseToDataSourceModel(&results[0])
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

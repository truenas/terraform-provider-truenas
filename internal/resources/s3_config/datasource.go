// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package s3_config

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &S3ConfigDataSource{}

// S3ConfigDataSource implements the truenas_s3_config data source.
type S3ConfigDataSource struct{ client *client.Client }

// NewDataSource returns a new S3ConfigDataSource.
func NewDataSource() datasource.DataSource { return &S3ConfigDataSource{} }

func (d *S3ConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_config"
}

func (d *S3ConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the TrueNAS S3 service configuration (s3.config; TrueNAS 27.0 and later). Takes no arguments: " +
			"there is exactly one S3 configuration per system.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{Computed: true, Description: "Fixed identifier for this singleton: always \"s3_config\"."},
			"listeners": dschema.ListNestedAttribute{
				Computed:    true,
				Description: "Where the S3 service listens.",
				NestedObject: dschema.NestedAttributeObject{Attributes: map[string]dschema.Attribute{
					"address": dschema.StringAttribute{Computed: true, Description: "IP address listened on."},
					"port":    dschema.Int64Attribute{Computed: true, Description: "TCP port."},
					"tls":     dschema.BoolAttribute{Computed: true, Description: "Whether this listener serves TLS."},
				}},
			},
			"servers":                dschema.Int64Attribute{Computed: true, Description: "Reactor threads serving the listen addresses."},
			"certificate":            dschema.Int64Attribute{Computed: true, Description: "ID of the certificate TLS listeners serve; null for the UI certificate."},
			"region":                 dschema.StringAttribute{Computed: true, Description: "Region name echoed to clients."},
			"log_level":              dschema.StringAttribute{Computed: true, Description: "Least serious log record kept."},
			"default_audit":          dschema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "Actions audited on buckets without their own audit list; [\"ALL\"] for everything."},
			"default_audit_overflow": dschema.StringAttribute{Computed: true, Description: "DROP or BACKPRESSURE."},
			"global_grants": dschema.ListNestedAttribute{
				Computed:    true,
				Description: "Grants that apply to every bucket.",
				NestedObject: dschema.NestedAttributeObject{Attributes: map[string]dschema.Attribute{
					"principal_type": dschema.StringAttribute{Computed: true, Description: "USER, GROUP or EVERYONE."},
					"xid":            dschema.Int64Attribute{Computed: true, Description: "uid or gid; null for EVERYONE."},
					"access":         dschema.StringAttribute{Computed: true, Description: "READONLY, WRITEONLY, READWRITE or DENY."},
				}},
			},
			"managed_root_dataset": dschema.StringAttribute{Computed: true, Description: "Dataset under which S3-created buckets get their datasets; null when buckets cannot be created through S3."},
		},
	}
}

func (d *S3ConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *S3ConfigDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state S3ConfigModel
	api, err := FetchConfig(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Read S3 configuration failed", wrapUnavailable(err))
		return
	}
	resp.Diagnostics.Append(responseToModel(ctx, api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

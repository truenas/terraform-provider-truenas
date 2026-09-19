// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package audit_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &AuditConfigDataSource{}

// AuditConfigDataSource implements the truenas_audit_config data source.
type AuditConfigDataSource struct{ client *client.Client }

// NewDataSource returns a new AuditConfigDataSource.
func NewDataSource() datasource.DataSource { return &AuditConfigDataSource{} }

func (d *AuditConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_audit_config"
}

func (d *AuditConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS audit configuration. Takes no arguments: there is " +
			"exactly one audit configuration per TrueNAS system.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"audit_config\".",
			},
			"retention": dschema.Int64Attribute{
				Computed:    true,
				Description: "Number of days to retain local audit messages.",
			},
			"reservation": dschema.Int64Attribute{
				Computed: true,
				Description: "Size in GiB of refreservation set on the ZFS dataset where the audit databases " +
					"are stored.",
			},
			"quota": dschema.Int64Attribute{
				Computed: true,
				Description: "Size in GiB of the maximum amount of space that may be consumed by the dataset " +
					"where the audit databases are stored.",
			},
			"quota_fill_warning": dschema.Int64Attribute{
				Computed:    true,
				Description: "Percentage used of dataset quota at which to generate a warning alert.",
			},
			"quota_fill_critical": dschema.Int64Attribute{
				Computed:    true,
				Description: "Percentage used of dataset quota at which to generate a critical alert.",
			},
			"remote_logging_enabled": dschema.BoolAttribute{
				Computed: true,
				Description: "Whether logging to a remote syslog server is enabled on TrueNAS, and audit " +
					"logs are included in what is sent remotely.",
			},
			"space": dschema.SingleNestedAttribute{
				Computed:    true,
				Description: "ZFS dataset space accounting for where the audit databases are stored, in bytes.",
				Attributes: map[string]dschema.Attribute{
					"used":                dschema.Int64Attribute{Computed: true, Description: "Total space used by the audit dataset, in bytes."},
					"used_by_dataset":     dschema.Int64Attribute{Computed: true, Description: "Space used by the dataset itself, in bytes."},
					"used_by_reservation": dschema.Int64Attribute{Computed: true, Description: "Space reserved for the dataset, in bytes."},
					"used_by_snapshots":   dschema.Int64Attribute{Computed: true, Description: "Space used by snapshots of the audit dataset, in bytes."},
					"available":           dschema.Int64Attribute{Computed: true, Description: "Available space remaining for the audit dataset, in bytes."},
				},
			},
			"enabled_services": dschema.SingleNestedAttribute{
				Computed:    true,
				Description: "Summary of what is currently being audited per service.",
				Attributes: map[string]dschema.Attribute{
					"middleware": dschema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "Middleware audit event types that are currently enabled."},
					"smb":        dschema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "SMB share names for which auditing is currently enabled."},
					"sudo":       dschema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "Sudo commands or users currently being audited."},
				},
			},
		},
	}
}

func (d *AuditConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AuditConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state AuditConfigDataSourceModel

	raw, err := d.client.CallRead(ctx, "audit.config")
	if err != nil {
		resp.Diagnostics.AddError("Read audit configuration failed", err.Error())
		return
	}

	var api auditConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse audit.config response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package s3_access_key

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &S3AccessKeyDataSource{}

// S3AccessKeyDataSource implements the truenas_s3_access_key data source
// (lookup by name; the secret is not exposed).
type S3AccessKeyDataSource struct{ client *client.Client }

// NewDataSource returns a new S3AccessKeyDataSource.
func NewDataSource() datasource.DataSource { return &S3AccessKeyDataSource{} }

func (d *S3AccessKeyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_access_key"
}

func (d *S3AccessKeyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Looks up a TrueNAS S3 access key by name (s3.accesskey.query; TrueNAS 27.0 and later). The secret is not exposed.",
		Attributes: map[string]dschema.Attribute{
			"id":              dschema.Int64Attribute{Computed: true, Description: "Numeric identifier of the access key."},
			"name":            dschema.StringAttribute{Required: true, Description: "Name of the access key to look up."},
			"username":        dschema.StringAttribute{Computed: true, Description: "Account the key belongs to."},
			"access_key":      dschema.StringAttribute{Computed: true, Description: "Access key id."},
			"enabled":         dschema.BoolAttribute{Computed: true, Description: "Whether the key may be used."},
			"expires_at":      dschema.StringAttribute{Computed: true, Description: "RFC3339 expiry, null when the key never expires."},
			"manage_buckets":  dschema.BoolAttribute{Computed: true, Description: "Whether the key may create and delete buckets."},
			"user_identifier": dschema.StringAttribute{Computed: true, Description: "uid (local) or SID (directory services) of the account."},
			"local":           dschema.BoolAttribute{Computed: true, Description: "Whether the account is local."},
			"created_at":      dschema.StringAttribute{Computed: true, Description: "RFC3339 UTC creation time."},
			"last_used_at":    dschema.StringAttribute{Computed: true, Description: "RFC3339 UTC time of the last accepted request; null when never used."},
			"status":          dschema.StringAttribute{Computed: true, Description: "ENABLED, DISABLED or EXPIRED."},
		},
	}
}

func (d *S3AccessKeyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *S3AccessKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state S3AccessKeyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	raw, err := d.client.CallRead(ctx, "s3.accesskey.query", [][]any{{"name", "=", state.Name.ValueString()}})
	if err != nil {
		resp.Diagnostics.AddError("Read S3 access key failed", errText(err))
		return
	}
	var rows []accessKeyAPI
	if err := json.Unmarshal(raw, &rows); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}
	if len(rows) != 1 {
		resp.Diagnostics.AddError("S3 access key not found", fmt.Sprintf("%d access keys named %q", len(rows), state.Name.ValueString()))
		return
	}
	resp.Diagnostics.Append(responseToDataSourceModel(&rows[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

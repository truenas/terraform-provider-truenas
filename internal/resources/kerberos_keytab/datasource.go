// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_keytab

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &KerberosKeytabDataSource{}

// KerberosKeytabDataSource implements the truenas_kerberos_keytab data
// source.
type KerberosKeytabDataSource struct{ client *client.Client }

// NewDataSource returns a new instance of KerberosKeytabDataSource.
func NewDataSource() datasource.DataSource { return &KerberosKeytabDataSource{} }

func (d *KerberosKeytabDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kerberos_keytab"
}

func (d *KerberosKeytabDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS Kerberos keytab entry by name.",
		Attributes: map[string]dschema.Attribute{
			"id":   dschema.Int64Attribute{Computed: true, Description: "Numeric identifier of the Kerberos keytab entry."},
			"name": dschema.StringAttribute{Required: true, Description: "Name of the Kerberos keytab entry to look up."},
			"file": dschema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "Base64-encoded Kerberos keytab data merged into the system keytab.",
			},
		},
	}
}

func (d *KerberosKeytabDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *KerberosKeytabDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state KerberosKeytabDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "kerberos.keytab.query",
		[]any{[]any{"name", "=", state.Name.ValueString()}})
	if err != nil {
		resp.Diagnostics.AddError("Query Kerberos keytabs failed", err.Error())
		return
	}

	var results []kerberosKeytabAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Kerberos keytab not found",
			fmt.Sprintf("no Kerberos keytab entry found with name %q", state.Name.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(&results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

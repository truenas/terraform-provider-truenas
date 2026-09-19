// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &KerberosConfigDataSource{}

// KerberosConfigDataSource implements the truenas_kerberos_config data
// source.
type KerberosConfigDataSource struct{ client *client.Client }

// NewDataSource returns a new KerberosConfigDataSource.
func NewDataSource() datasource.DataSource { return &KerberosConfigDataSource{} }

func (d *KerberosConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kerberos_config"
}

func (d *KerberosConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS Kerberos configuration (kerberos.config). Takes no " +
			"arguments: there is exactly one Kerberos configuration per TrueNAS system.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"kerberos_config\".",
			},
			"appdefaults_aux": dschema.StringAttribute{
				Computed: true,
				Description: "Advanced field for manually setting additional parameters inside the appdefaults " +
					"section of krb5.conf.",
			},
			"libdefaults_aux": dschema.StringAttribute{
				Computed: true,
				Description: "Advanced field for manually setting additional parameters inside the libdefaults " +
					"section of krb5.conf.",
			},
		},
	}
}

func (d *KerberosConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *KerberosConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state KerberosConfigDataSourceModel

	raw, err := d.client.CallRead(ctx, "kerberos.config")
	if err != nil {
		resp.Diagnostics.AddError("Read Kerberos configuration failed", err.Error())
		return
	}

	var api kerberosConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse kerberos.config response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

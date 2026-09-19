// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_realm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &KerberosRealmDataSource{}

// KerberosRealmDataSource implements the truenas_kerberos_realm data
// source.
type KerberosRealmDataSource struct{ client *client.Client }

// NewDataSource returns a new instance of KerberosRealmDataSource.
func NewDataSource() datasource.DataSource { return &KerberosRealmDataSource{} }

func (d *KerberosRealmDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kerberos_realm"
}

func (d *KerberosRealmDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS Kerberos realm by name.",
		Attributes: map[string]dschema.Attribute{
			"id":    dschema.Int64Attribute{Computed: true},
			"realm": dschema.StringAttribute{Required: true, Description: "Kerberos realm name to look up."},
			"primary_kdc": dschema.StringAttribute{
				Computed:    true,
				Description: "The master Kerberos domain controller for this realm.",
			},
			"kdc": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of Kerberos domain controllers for this realm.",
			},
			"admin_server": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of Kerberos admin servers for this realm.",
			},
			"kpasswd_server": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of Kerberos kpasswd servers for this realm.",
			},
		},
	}
}

func (d *KerberosRealmDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *KerberosRealmDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state KerberosRealmDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "kerberos.realm.query",
		[]any{[]any{"realm", "=", state.Realm.ValueString()}})
	if err != nil {
		resp.Diagnostics.AddError("Query Kerberos realms failed", err.Error())
		return
	}

	var results []kerberosRealmAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Kerberos realm not found",
			fmt.Sprintf("no Kerberos realm found with name %q", state.Realm.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

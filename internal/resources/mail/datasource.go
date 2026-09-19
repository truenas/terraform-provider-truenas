// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package mail

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &MailDataSource{}

// MailDataSource implements the truenas_mail data source.
type MailDataSource struct{ client *client.Client }

// NewDataSource returns a new MailDataSource.
func NewDataSource() datasource.DataSource { return &MailDataSource{} }

func (d *MailDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mail"
}

func (d *MailDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS system mail (email) configuration. Takes no arguments: " +
			"there is exactly one mail configuration per TrueNAS system. Does not expose the SMTP password: " +
			"mail.config never returns it.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"mail\".",
			},
			"fromemail": dschema.StringAttribute{
				Computed:    true,
				Description: "\"From\" address used on outgoing mail.",
			},
			"fromname": dschema.StringAttribute{
				Computed:    true,
				Description: "\"From\" display name used on outgoing mail.",
			},
			"outgoingserver": dschema.StringAttribute{
				Computed:    true,
				Description: "Outgoing SMTP server hostname.",
			},
			"port": dschema.Int64Attribute{
				Computed:    true,
				Description: "Outgoing SMTP server port.",
			},
			"security": dschema.StringAttribute{
				Computed:    true,
				Description: "SMTP transport security: one of PLAIN, SSL, TLS.",
			},
			"smtp": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether SMTP authentication is enabled.",
			},
			"user": dschema.StringAttribute{
				Computed:    true,
				Description: "SMTP authentication username.",
			},
		},
	}
}

func (d *MailDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *MailDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state MailDataSourceModel

	raw, err := d.client.CallRead(ctx, "mail.config")
	if err != nil {
		resp.Diagnostics.AddError("Read mail configuration failed", err.Error())
		return
	}

	var api mailAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse mail.config response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(&api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

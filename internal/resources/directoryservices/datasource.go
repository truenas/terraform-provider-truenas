package directoryservices

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &DirectoryServicesDataSource{}

// DirectoryServicesDataSource implements the truenas_directoryservices data
// source.
type DirectoryServicesDataSource struct{ client *client.Client }

// NewDataSource returns a new DirectoryServicesDataSource.
func NewDataSource() datasource.DataSource { return &DirectoryServicesDataSource{} }

func (d *DirectoryServicesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_directoryservices"
}

func (d *DirectoryServicesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS SCALE directory services configuration " +
			"(directoryservices.config) and health (directoryservices.status). Takes no arguments: there is " +
			"exactly one directory services configuration per TrueNAS system. Does not expose \"credential\": " +
			"see the truenas_directoryservices resource documentation for why the credential used to join a " +
			"domain is never readable back from TrueNAS.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"directoryservices\".",
			},
			"service_type": dschema.StringAttribute{
				Computed:    true,
				Description: "The directory service type currently configured, or null if never joined.",
			},
			"enable": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether directory services are currently enabled.",
			},
			"enable_account_cache": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether backend caching for user and group lists is enabled.",
			},
			"enable_dns_updates": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether automatic DNS updates via nsupdate/GSSAPI/TSIG are enabled.",
			},
			"timeout": dschema.Int64Attribute{
				Computed:    true,
				Description: "Timeout (in seconds) for DNS queries and LDAP network requests.",
			},
			"kerberos_realm": dschema.StringAttribute{
				Computed:    true,
				Description: "Name of the Kerberos realm used for authentication to the directory service.",
			},
			"configuration_activedirectory": dschema.SingleNestedAttribute{
				Computed:    true,
				Description: "Active Directory join configuration, or null if not configured for Active Directory.",
				Attributes: map[string]dschema.Attribute{
					"hostname": dschema.StringAttribute{
						Computed:    true,
						Description: "Hostname of the TrueNAS server registered in Active Directory.",
					},
					"domain": dschema.StringAttribute{
						Computed:    true,
						Description: "Full DNS domain name of the joined Active Directory domain.",
					},
					"site": dschema.StringAttribute{
						Computed:    true,
						Description: "The Active Directory site where the TrueNAS server is located.",
					},
					"computer_account_ou": dschema.StringAttribute{
						Computed:    true,
						Description: "Organizational unit (OU) containing the TrueNAS computer account.",
					},
					"use_default_domain": dschema.BoolAttribute{
						Computed:    true,
						Description: "Whether the domain prefix is removed from Active Directory user/group names.",
					},
					"enable_trusted_domains": dschema.BoolAttribute{
						Computed:    true,
						Description: "Whether support for trusted domains is enabled.",
					},
				},
			},
			"status": dschema.StringAttribute{
				Computed: true,
				Description: "Directory service status from the last health check: DISABLED, FAULTED, " +
					"LEAVING, JOINING, or HEALTHY. Null if directory services are disabled.",
			},
			"status_msg": dschema.StringAttribute{
				Computed: true,
				Description: "Reason the directory service is FAULTED after a failed health check. Null if " +
					"not faulted.",
			},
		},
	}
}

func (d *DirectoryServicesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DirectoryServicesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state DirectoryServicesDataSourceModel

	raw, err := d.client.CallRead(ctx, "directoryservices.config")
	if err != nil {
		resp.Diagnostics.AddError("Read directory services configuration failed", err.Error())
		return
	}
	var api directoryServicesAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse directoryservices.config response", err.Error())
		return
	}

	rawStatus, err := d.client.CallRead(ctx, "directoryservices.status")
	if err != nil {
		resp.Diagnostics.AddError("Read directory services status failed", err.Error())
		return
	}
	var status directoryServicesStatusAPI
	if err := json.Unmarshal(rawStatus, &status); err != nil {
		resp.Diagnostics.AddError("Parse directoryservices.status response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &api, &status, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

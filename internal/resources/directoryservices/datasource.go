// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package directoryservices

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
		Description: "Reads the current TrueNAS directory services configuration " +
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
					"idmap": dschema.SingleNestedAttribute{
						Computed:    true,
						Description: "Active Directory idmap configuration (UID/GID mapping), or null if not configured for Active Directory.",
						Attributes: map[string]dschema.Attribute{
							"builtin": dschema.SingleNestedAttribute{
								Computed:    true,
								Description: "UID/GID range for automatically generated well-known/BUILTIN accounts.",
								Attributes: map[string]dschema.Attribute{
									"name":       dschema.StringAttribute{Computed: true, Description: "Short name for the domain."},
									"range_low":  dschema.Int64Attribute{Computed: true, Description: "Lowest UID/GID the idmap backend can assign."},
									"range_high": dschema.Int64Attribute{Computed: true, Description: "Highest UID/GID the idmap backend can assign."},
								},
							},
							"idmap_domain": dschema.SingleNestedAttribute{
								Computed:    true,
								Description: "How domain accounts joined to TrueNAS are mapped to Unix UIDs/GIDs.",
								Attributes: map[string]dschema.Attribute{
									"idmap_backend":      dschema.StringAttribute{Computed: true, Description: "Idmap backend type (\"AD\" or \"RID\" for this provider version)."},
									"name":               dschema.StringAttribute{Computed: true, Description: "Short name for the domain."},
									"range_low":          dschema.Int64Attribute{Computed: true, Description: "Lowest UID/GID the idmap backend can assign."},
									"range_high":         dschema.Int64Attribute{Computed: true, Description: "Highest UID/GID the idmap backend can assign."},
									"schema_mode":        dschema.StringAttribute{Computed: true, Description: "AD backend schema mode."},
									"unix_primary_group": dschema.BoolAttribute{Computed: true, Description: "AD backend only: primary group source."},
									"unix_nss_info":      dschema.BoolAttribute{Computed: true, Description: "AD backend only: shell/home directory source."},
									"sssd_compat":        dschema.BoolAttribute{Computed: true, Description: "RID backend only: SSSD-compatible low range."},
								},
							},
						},
					},
				},
			},
			"configuration_ldap": dschema.SingleNestedAttribute{
				Computed:    true,
				Description: "Plain LDAP directory configuration, or null if not configured for LDAP.",
				Attributes: map[string]dschema.Attribute{
					"server_urls": dschema.ListAttribute{
						Computed:    true,
						ElementType: types.StringType,
						Description: "List of LDAP server URIs used for LDAP binds.",
					},
					"basedn":                dschema.StringAttribute{Computed: true, Description: "The base DN used for LDAP operations."},
					"starttls":              dschema.BoolAttribute{Computed: true, Description: "Whether StartTLS is used."},
					"validate_certificates": dschema.BoolAttribute{Computed: true, Description: "Whether remote LDAP certificates are validated."},
					"schema":                dschema.StringAttribute{Computed: true, Description: "The LDAP attribute schema in use."},
					"auxiliary_parameters":  dschema.StringAttribute{Computed: true, Description: "Additional SSSD configuration parameters."},
					"search_bases": dschema.SingleNestedAttribute{
						Computed:    true,
						Description: "Alternative LDAP search base settings.",
						Attributes: map[string]dschema.Attribute{
							"base_user":     dschema.StringAttribute{Computed: true, Description: "Base DN for LDAP user searches."},
							"base_group":    dschema.StringAttribute{Computed: true, Description: "Base DN for LDAP group searches."},
							"base_netgroup": dschema.StringAttribute{Computed: true, Description: "Base DN for LDAP netgroup searches."},
						},
					},
					"attribute_maps": dschema.SingleNestedAttribute{
						Computed:    true,
						Description: "Non-standard LDAP attribute mapping overrides.",
						Attributes: map[string]dschema.Attribute{
							"passwd": dschema.SingleNestedAttribute{
								Computed:    true,
								Description: "LDAP attribute mappings for user (passwd) entries.",
								Attributes: map[string]dschema.Attribute{
									"user_object_class":   dschema.StringAttribute{Computed: true, Description: "User entry object class."},
									"user_name":           dschema.StringAttribute{Computed: true, Description: "Login name attribute."},
									"user_uid":            dschema.StringAttribute{Computed: true, Description: "User id attribute."},
									"user_gid":            dschema.StringAttribute{Computed: true, Description: "Primary group id attribute."},
									"user_gecos":          dschema.StringAttribute{Computed: true, Description: "Gecos field attribute."},
									"user_home_directory": dschema.StringAttribute{Computed: true, Description: "Home directory attribute."},
									"user_shell":          dschema.StringAttribute{Computed: true, Description: "Default shell attribute."},
								},
							},
							"shadow": dschema.SingleNestedAttribute{
								Computed:    true,
								Description: "LDAP attribute mappings for shadow password entries.",
								Attributes: map[string]dschema.Attribute{
									"shadow_last_change": dschema.StringAttribute{Computed: true, Description: "Last password change attribute."},
									"shadow_min":         dschema.StringAttribute{Computed: true, Description: "Minimum password age attribute."},
									"shadow_max":         dschema.StringAttribute{Computed: true, Description: "Maximum password age attribute."},
									"shadow_warning":     dschema.StringAttribute{Computed: true, Description: "Password warning period attribute."},
									"shadow_inactive":    dschema.StringAttribute{Computed: true, Description: "Password inactivity period attribute."},
									"shadow_expire":      dschema.StringAttribute{Computed: true, Description: "Account expiration date attribute."},
								},
							},
							"group": dschema.SingleNestedAttribute{
								Computed:    true,
								Description: "LDAP attribute mappings for group entries.",
								Attributes: map[string]dschema.Attribute{
									"group_object_class": dschema.StringAttribute{Computed: true, Description: "Group entry object class."},
									"group_gid":          dschema.StringAttribute{Computed: true, Description: "Group id attribute."},
									"group_member":       dschema.StringAttribute{Computed: true, Description: "Group member names attribute."},
								},
							},
							"netgroup": dschema.SingleNestedAttribute{
								Computed:    true,
								Description: "LDAP attribute mappings for netgroup entries.",
								Attributes: map[string]dschema.Attribute{
									"netgroup_object_class": dschema.StringAttribute{Computed: true, Description: "Netgroup entry object class."},
									"netgroup_member":       dschema.StringAttribute{Computed: true, Description: "Netgroup members attribute."},
									"netgroup_triple":       dschema.StringAttribute{Computed: true, Description: "Netgroup triple attribute."},
								},
							},
						},
					},
				},
			},
			"configuration_ipa": dschema.SingleNestedAttribute{
				Computed:    true,
				Description: "IPA (FreeIPA) join configuration, or null if not configured for IPA.",
				Attributes: map[string]dschema.Attribute{
					"target_server":         dschema.StringAttribute{Computed: true, Description: "The IPA server used to build join/leave URLs."},
					"hostname":              dschema.StringAttribute{Computed: true, Description: "Hostname of the TrueNAS server registered in IPA."},
					"domain":                dschema.StringAttribute{Computed: true, Description: "The domain of the IPA server."},
					"basedn":                dschema.StringAttribute{Computed: true, Description: "The base DN used for LDAP operations."},
					"validate_certificates": dschema.BoolAttribute{Computed: true, Description: "Whether remote LDAP certificates are validated."},
					"smb_domain": dschema.SingleNestedAttribute{
						Computed:    true,
						Description: "IPA SMB domain settings, detected during the IPA join, or null if not configured.",
						Attributes: map[string]dschema.Attribute{
							"name":        dschema.StringAttribute{Computed: true, Description: "Short name for the SMB domain."},
							"range_low":   dschema.Int64Attribute{Computed: true, Description: "Lowest UID/GID the idmap backend can assign."},
							"range_high":  dschema.Int64Attribute{Computed: true, Description: "Highest UID/GID the idmap backend can assign."},
							"domain_name": dschema.StringAttribute{Computed: true, Description: "Name of the SMB domain per the IPA configuration."},
							"domain_sid":  dschema.StringAttribute{Computed: true, Description: "The domain SID for the joined IPA domain."},
						},
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

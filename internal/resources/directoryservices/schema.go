// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package directoryservices

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// idmapRangeValidators are the shared min/max validators for every idmap
// UID/GID range field (builtin and idmap_domain alike): probed minimum
// 1000, maximum 2147000000.
func idmapRangeValidators() []validator.Int64 {
	return []validator.Int64{int64validator.Between(1000, 2147000000)}
}

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the TrueNAS directory services configuration (directoryservices.config) — " +
			"joining the system to an Active Directory domain, an IPA (FreeIPA) domain, or binding to a plain " +
			"LDAP directory. This is a singleton resource: there is exactly one directory services configuration " +
			"per TrueNAS system, so it is never created or deleted on TrueNAS; Terraform create/update calls " +
			"directoryservices.update (a job — joining or leaving a domain can take a while), and after an " +
			"enabling update this provider additionally polls directoryservices.status until it reports HEALTHY " +
			"(up to 5 minutes) before considering the apply successful. DELETE NEVER CALLS " +
			"directoryservices.leave: destroying this resource calls directoryservices.update with enable=false " +
			"only, which disables directory services locally without leaving the domain or removing the TrueNAS " +
			"computer account from the domain controller. This is deliberate — leaving a domain requires a " +
			"domain administrator credential that is not necessarily available (or desirable to hold) at destroy " +
			"time.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"directoryservices\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"service_type": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "The directory service type to join: \"ACTIVEDIRECTORY\", \"LDAP\", or \"IPA\".",
				Validators:    []validator.String{stringvalidator.OneOf("ACTIVEDIRECTORY", "LDAP", "IPA")},
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"enable": schema.BoolAttribute{
				Required: true,
				Description: "Enable the directory service. Setting this to true when TrueNAS has never joined " +
					"the configured domain causes TrueNAS to attempt to join it — creating a computer account " +
					"and DNS records on the domain controller (ACTIVEDIRECTORY/IPA) or binding to the configured " +
					"LDAP server (LDAP). Setting this to false disables directory services locally without " +
					"leaving the domain. Destroying this resource always sends enable=false (see the " +
					"resource-level description); it never leaves the domain.",
			},
			"enable_account_cache": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Enable backend caching for user and group lists from the directory service. " +
					"Defaults to true on TrueNAS.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"enable_dns_updates": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Enable automatic DNS updates for the TrueNAS server in the domain via nsupdate " +
					"and GSSAPI/TSIG. Defaults to true on TrueNAS.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"timeout": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Timeout (in seconds, 5-60) for DNS queries performed during the join process and " +
					"for LDAP network requests. Defaults to 10 on TrueNAS.",
				Validators:    []validator.Int64{int64validator.Between(5, 60)},
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"kerberos_realm": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Name of the Kerberos realm used for authentication to the directory service. " +
					"When joining Active Directory or IPA for the first time, the realm is normally detected and " +
					"configured automatically if left unset; not used for LDAP. An empty string clears the " +
					"assigned realm (null on the wire).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"credential": schema.SingleNestedAttribute{
				Optional: true,
				Description: "Credential used to join the domain or bind to LDAP. REQUIRED the first time " +
					"\"enable\" is set to true. For ACTIVEDIRECTORY/IPA it may be omitted from configuration on " +
					"later applies: once joined, TrueNAS itself swaps the KERBEROS_USER credential used for the " +
					"join for a keytab-backed KERBEROS_PRINCIPAL machine-account credential internally, and this " +
					"provider automatically resends THAT (not the original password) on every subsequent update " +
					"that keeps directory services enabled, including the eventual disable. LDAP credentials " +
					"(LDAP_PLAIN/LDAP_ANONYMOUS/LDAP_MTLS) have no such swap and must be resupplied (or left in " +
					"HCL) on every apply. NOTE: credential_type, username, binddn, client_certificate, and " +
					"principal ARE persisted in Terraform state (none are sensitive on their own); password and " +
					"bindpw are WriteOnly (Sensitive, requires Terraform >= 1.11) and are NEVER read back from " +
					"TrueNAS or stored in state.",
				Attributes: map[string]schema.Attribute{
					"credential_type": schema.StringAttribute{
						Required: true,
						Description: "Credential type. One of \"KERBEROS_USER\" (username+password — " +
							"ACTIVEDIRECTORY/IPA join), \"KERBEROS_PRINCIPAL\" (principal — reuse of an existing " +
							"machine-account keytab), \"LDAP_PLAIN\" (binddn+bindpw), \"LDAP_ANONYMOUS\" (no " +
							"fields), or \"LDAP_MTLS\" (client_certificate).",
						Validators: []validator.String{stringvalidator.OneOf(
							"KERBEROS_USER", "KERBEROS_PRINCIPAL", "LDAP_PLAIN", "LDAP_ANONYMOUS", "LDAP_MTLS",
						)},
					},
					"username": schema.StringAttribute{
						Optional: true,
						Description: "Username of the domain account used to obtain a Kerberos ticket for " +
							"joining/authenticating to the directory service. Required when credential_type is " +
							"\"KERBEROS_USER\"; unused otherwise. This account must exist on the domain " +
							"controller and (for the initial join) typically needs domain administrator " +
							"privileges.",
					},
					"password": schema.StringAttribute{
						Optional:  true,
						Sensitive: true,
						WriteOnly: true,
						Description: "Password for the account named in \"username\". Required when " +
							"credential_type is \"KERBEROS_USER\"; unused otherwise. Write-only: never stored " +
							"in Terraform state. Requires Terraform >= 1.11.",
					},
					"binddn": schema.StringAttribute{
						Optional: true,
						Description: "Bind DN used to authenticate to the LDAP server. Required when " +
							"credential_type is \"LDAP_PLAIN\"; unused otherwise. Example: " +
							"\"cn=admin,dc=example,dc=internal\". Not sensitive on its own and persists normally " +
							"in Terraform state.",
					},
					"bindpw": schema.StringAttribute{
						Optional:  true,
						Sensitive: true,
						WriteOnly: true,
						Description: "Password for the account named in \"binddn\". Required when " +
							"credential_type is \"LDAP_PLAIN\"; unused otherwise. Write-only: never stored in " +
							"Terraform state. Requires Terraform >= 1.11.",
					},
					"client_certificate": schema.StringAttribute{
						Optional: true,
						Description: "Name of the TrueNAS certificate resource used to authenticate to the LDAP " +
							"server via mutual TLS. Required when credential_type is \"LDAP_MTLS\"; unused " +
							"otherwise.",
					},
					"principal": schema.StringAttribute{
						Optional: true,
						Description: "Kerberos principal for an existing machine-account keytab. Required when " +
							"credential_type is \"KERBEROS_PRINCIPAL\"; unused otherwise. This provider sets this " +
							"automatically on every update after a successful ACTIVEDIRECTORY/IPA join — supplying " +
							"it directly is normally only needed when re-adopting a join TrueNAS already " +
							"performed outside Terraform.",
					},
				},
			},
			"configuration_activedirectory": schema.SingleNestedAttribute{
				Optional: true,
				Computed: true,
				Description: "Active Directory join configuration. REQUIRED whenever \"enable\" is true and " +
					"service_type is \"ACTIVEDIRECTORY\" — see \"credential\" for why this must be supplied on " +
					"every apply for as long as enable stays true, not only the first time. \"trusted_domains\" " +
					"is not exposed by this provider version — TrueNAS applies its standard default (no trusted " +
					"domains) on first join, and this provider preserves whatever value TrueNAS has already " +
					"assigned on every subsequent update.",
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"hostname": schema.StringAttribute{
						Required:    true,
						Description: "Hostname of the TrueNAS server to register in Active Directory.",
					},
					"domain": schema.StringAttribute{
						Required: true,
						Description: "Full DNS domain name of the Active Directory domain to join (not a " +
							"domain controller's hostname). Example: \"mydomain.internal\".",
					},
					"site": schema.StringAttribute{
						Optional: true,
						Computed: true,
						Description: "The Active Directory site where the TrueNAS server is located. " +
							"TrueNAS detects this automatically during the join process when left unset. An " +
							"empty string clears the assigned site (null on the wire).",
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
					},
					"computer_account_ou": schema.StringAttribute{
						Optional: true,
						Computed: true,
						Description: "Overrides the default organizational unit (OU) in which the TrueNAS " +
							"computer account is created during the domain join. Example: " +
							"\"TRUENAS_SERVERS/NYC\". An empty string clears the override (null on the wire).",
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
					},
					"use_default_domain": schema.BoolAttribute{
						Optional: true,
						Computed: true,
						Description: "Controls whether the domain prefix is removed from Active Directory " +
							"user and group names (e.g. \"administrator\" instead of " +
							"\"EXAMPLE\\administrator\"). Defaults to false on TrueNAS.",
						PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
					},
					"enable_trusted_domains": schema.BoolAttribute{
						Optional: true,
						Computed: true,
						Description: "Enable support for trusted domains. Defaults to false on TrueNAS. This " +
							"provider version does not expose per-trusted-domain idmap configuration.",
						PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
					},
					"idmap": schema.SingleNestedAttribute{
						Optional: true,
						Computed: true,
						Description: "Configuration for mapping Active Directory accounts to accounts on the " +
							"TrueNAS server. Defaults are suitable for new deployments: when left unset, TrueNAS " +
							"assigns its own default builtin/idmap_domain ranges (RID backend) on first join, and " +
							"this provider preserves whatever value TrueNAS has already assigned on every " +
							"subsequent update. Only the \"AD\" and \"RID\" idmap_domain backends are exposed by " +
							"this provider version — the \"LDAP\" and \"RFC2307\" backends (which need a secret " +
							"LDAP bind password) are deferred.",
						PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
						Attributes: map[string]schema.Attribute{
							"builtin": schema.SingleNestedAttribute{
								Optional: true,
								Computed: true,
								Description: "UID and GID range configuration for automatically generated " +
									"accounts linked to well-known and BUILTIN accounts on Windows servers. " +
									"Defaults to range 90000001-100000000 on TrueNAS.",
								PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Optional: true,
										Computed: true,
										Description: "Short name for the domain. Should match the NetBIOS " +
											"domain name. Null if the domain is configured as the base idmap.",
										PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
									},
									"range_low": schema.Int64Attribute{
										Optional:      true,
										Computed:      true,
										Description:   "The lowest UID or GID that the idmap backend can assign.",
										Validators:    idmapRangeValidators(),
										PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
									},
									"range_high": schema.Int64Attribute{
										Optional:      true,
										Computed:      true,
										Description:   "The highest UID or GID that the idmap backend can assign.",
										Validators:    idmapRangeValidators(),
										PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
									},
								},
							},
							"idmap_domain": schema.SingleNestedAttribute{
								Optional: true,
								Computed: true,
								Description: "Defines how domain accounts joined to TrueNAS are mapped to Unix " +
									"UIDs and GIDs. Defaults to the \"RID\" backend on TrueNAS.",
								PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
								Attributes: map[string]schema.Attribute{
									"idmap_backend": schema.StringAttribute{
										Required: true,
										Description: "Idmap backend type. Only \"AD\" and \"RID\" are supported " +
											"by this provider version (\"LDAP\" and \"RFC2307\", which require a " +
											"secret LDAP bind password, are deferred).",
										Validators: []validator.String{stringvalidator.OneOf("AD", "RID")},
									},
									"name": schema.StringAttribute{
										Optional: true,
										Computed: true,
										Description: "Short name for the domain. Should match the NetBIOS " +
											"domain name. Null if the domain is configured as the base idmap.",
										PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
									},
									"range_low": schema.Int64Attribute{
										Optional:      true,
										Computed:      true,
										Description:   "The lowest UID or GID that the idmap backend can assign.",
										Validators:    idmapRangeValidators(),
										PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
									},
									"range_high": schema.Int64Attribute{
										Optional:      true,
										Computed:      true,
										Description:   "The highest UID or GID that the idmap backend can assign.",
										Validators:    idmapRangeValidators(),
										PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
									},
									"schema_mode": schema.StringAttribute{
										Optional: true,
										Computed: true,
										Description: "Schema mode used to query Active Directory for user/group " +
											"information. Required when idmap_backend is \"AD\"; unused " +
											"otherwise.",
										Validators:    []validator.String{stringvalidator.OneOf("RFC2307", "SFU", "SFU20")},
										PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
									},
									"unix_primary_group": schema.BoolAttribute{
										Optional: true,
										Computed: true,
										Description: "AD backend only: if true, the user's primary group is " +
											"fetched via the gidNumber LDAP attribute rather than the Active " +
											"Directory primaryGroupID attribute. Defaults to false.",
										PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
									},
									"unix_nss_info": schema.BoolAttribute{
										Optional: true,
										Computed: true,
										Description: "AD backend only: if true, login shell and home directory " +
											"are retrieved from LDAP SFU attributes. Defaults to false.",
										PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
									},
									"sssd_compat": schema.BoolAttribute{
										Optional: true,
										Computed: true,
										Description: "RID backend only: generate an idmap low range using the " +
											"SSSD algorithm. Defaults to false.",
										PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
									},
								},
							},
						},
					},
				},
			},
			"configuration_ldap": schema.SingleNestedAttribute{
				Optional: true,
				Computed: true,
				Description: "Plain LDAP directory configuration. REQUIRED whenever \"enable\" is true and " +
					"service_type is \"LDAP\".",
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"server_urls": schema.ListAttribute{
						Required:    true,
						ElementType: types.StringType,
						Description: "List of LDAP server URIs used for LDAP binds. Each URI must begin with " +
							"ldap:// or ldaps:// and may use either a DNS name or an IP address. Example: " +
							"[\"ldaps://myldap.domain.internal\"].",
					},
					"basedn": schema.StringAttribute{
						Required:    true,
						Description: "The base DN to use when performing LDAP operations. Example: \"dc=domain,dc=internal\".",
					},
					"starttls": schema.BoolAttribute{
						Optional: true,
						Computed: true,
						Description: "Establish TLS by transmitting a StartTLS request to the server. Defaults " +
							"to false on TrueNAS.",
						PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
					},
					"validate_certificates": schema.BoolAttribute{
						Optional: true,
						Computed: true,
						Description: "If false, TrueNAS does not validate certificates from the remote LDAP " +
							"server. Defaults to true on TrueNAS.",
						PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
					},
					"schema": schema.StringAttribute{
						Optional: true,
						Computed: true,
						Description: "The type of LDAP attribute schema the remote LDAP server uses. Defaults " +
							"to \"RFC2307\" on TrueNAS.",
						Validators:    []validator.String{stringvalidator.OneOf("RFC2307", "RFC2307BIS")},
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
					},
					"auxiliary_parameters": schema.StringAttribute{
						Optional: true,
						Computed: true,
						Description: "Additional parameters to add to the SSSD configuration. WARNING: TrueNAS " +
							"does not check the validity of these parameters. An empty string clears the value " +
							"(null on the wire).",
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
					},
					"search_bases": schema.SingleNestedAttribute{
						Optional: true,
						Computed: true,
						Description: "Alternative LDAP search base settings, defining where to find user, " +
							"group, and netgroup entries. If unspecified (the default), TrueNAS uses \"basedn\" " +
							"for all three. Use only if the LDAP server uses a non-standard schema.",
						PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
						Attributes: map[string]schema.Attribute{
							"base_user": schema.StringAttribute{
								Optional: true, Computed: true,
								Description:   "Base DN to limit LDAP user searches. Null (default) uses \"basedn\".",
								PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
							},
							"base_group": schema.StringAttribute{
								Optional: true, Computed: true,
								Description:   "Base DN to limit LDAP group searches. Null (default) uses \"basedn\".",
								PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
							},
							"base_netgroup": schema.StringAttribute{
								Optional: true, Computed: true,
								Description:   "Base DN to limit LDAP netgroup searches. Null (default) uses \"basedn\".",
								PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
							},
						},
					},
					"attribute_maps": schema.SingleNestedAttribute{
						Optional: true,
						Computed: true,
						Description: "Optional LDAP attribute mapping for LDAP servers that do not follow " +
							"RFC2307/RFC2307BIS. Use only if the LDAP server is non-standard.",
						PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
						Attributes: map[string]schema.Attribute{
							"passwd": schema.SingleNestedAttribute{
								Optional: true, Computed: true,
								Description:   "LDAP attribute mappings for user (passwd) entries.",
								PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
								Attributes: map[string]schema.Attribute{
									"user_object_class":   ldapAttrMapLeaf("The user entry object class in LDAP."),
									"user_name":           ldapAttrMapLeaf("The LDAP attribute for the user's login name."),
									"user_uid":            ldapAttrMapLeaf("The LDAP attribute for the user's id."),
									"user_gid":            ldapAttrMapLeaf("The LDAP attribute for the user's primary group id."),
									"user_gecos":          ldapAttrMapLeaf("The LDAP attribute for the user's gecos field."),
									"user_home_directory": ldapAttrMapLeaf("The LDAP attribute for the user's home directory."),
									"user_shell":          ldapAttrMapLeaf("The LDAP attribute for the user's default shell path."),
								},
							},
							"shadow": schema.SingleNestedAttribute{
								Optional: true, Computed: true,
								Description:   "LDAP attribute mappings for shadow password entries.",
								PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
								Attributes: map[string]schema.Attribute{
									"shadow_last_change": ldapAttrMapLeaf("shadow(5) date of the last password change."),
									"shadow_min":         ldapAttrMapLeaf("shadow(5) minimum password age."),
									"shadow_max":         ldapAttrMapLeaf("shadow(5) maximum password age."),
									"shadow_warning":     ldapAttrMapLeaf("shadow(5) password warning period."),
									"shadow_inactive":    ldapAttrMapLeaf("shadow(5) password inactivity period."),
									"shadow_expire":      ldapAttrMapLeaf("shadow(5) account expiration date."),
								},
							},
							"group": schema.SingleNestedAttribute{
								Optional: true, Computed: true,
								Description:   "LDAP attribute mappings for group entries.",
								PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
								Attributes: map[string]schema.Attribute{
									"group_object_class": ldapAttrMapLeaf("The LDAP object class for group entries."),
									"group_gid":          ldapAttrMapLeaf("The LDAP attribute for the group's id."),
									"group_member":       ldapAttrMapLeaf("The LDAP attribute for the names of the group's members."),
								},
							},
							"netgroup": schema.SingleNestedAttribute{
								Optional: true, Computed: true,
								Description:   "LDAP attribute mappings for netgroup entries.",
								PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
								Attributes: map[string]schema.Attribute{
									"netgroup_object_class": ldapAttrMapLeaf("The LDAP object class for netgroup entries."),
									"netgroup_member":       ldapAttrMapLeaf("The LDAP attribute for the netgroup's members."),
									"netgroup_triple":       ldapAttrMapLeaf("The LDAP attribute for netgroup triples (host, user, domain)."),
								},
							},
						},
					},
				},
			},
			"configuration_ipa": schema.SingleNestedAttribute{
				Optional: true,
				Computed: true,
				Description: "IPA (FreeIPA) join configuration. REQUIRED whenever \"enable\" is true and " +
					"service_type is \"IPA\".",
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"target_server": schema.StringAttribute{
						Required: true,
						Description: "The name of the IPA server TrueNAS uses to build URLs when it joins or " +
							"leaves the IPA domain. Example: \"ipa.example.internal\".",
					},
					"hostname": schema.StringAttribute{
						Required:    true,
						Description: "Hostname of the TrueNAS server to register in IPA during the join process.",
					},
					"domain": schema.StringAttribute{
						Required:    true,
						Description: "The domain of the IPA server. Example: \"ipa.internal\".",
					},
					"basedn": schema.StringAttribute{
						Required:    true,
						Description: "The base DN to use when performing LDAP operations. Example: \"dc=example,dc=internal\".",
					},
					"validate_certificates": schema.BoolAttribute{
						Optional: true,
						Computed: true,
						Description: "If false, TrueNAS does not validate certificates from the remote LDAP " +
							"server. Defaults to true on TrueNAS.",
						PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
					},
					"smb_domain": schema.SingleNestedAttribute{
						Optional: true,
						Computed: true,
						Description: "Settings for the IPA SMB domain. TrueNAS detects these automatically " +
							"during the IPA join process; some IPA domains do not include SMB schema " +
							"configuration, in which case this remains null.",
						PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
						Attributes: map[string]schema.Attribute{
							"name": schema.StringAttribute{
								Optional: true, Computed: true,
								Description:   "Short name for the SMB domain.",
								PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
							},
							"range_low": schema.Int64Attribute{
								Optional: true, Computed: true,
								Description:   "The lowest UID or GID that the idmap backend can assign.",
								Validators:    idmapRangeValidators(),
								PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
							},
							"range_high": schema.Int64Attribute{
								Optional: true, Computed: true,
								Description:   "The highest UID or GID that the idmap backend can assign.",
								Validators:    idmapRangeValidators(),
								PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
							},
							"domain_name": schema.StringAttribute{
								Optional: true, Computed: true,
								Description:   "Name of the SMB domain as defined in the IPA configuration.",
								PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
							},
							"domain_sid": schema.StringAttribute{
								Optional: true, Computed: true,
								Description:   "The domain SID for the IPA domain to which TrueNAS is joined.",
								PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
							},
						},
					},
				},
			},
		},
	}
}

// ldapAttrMapLeaf builds the (repeated, identically-shaped) schema for a
// single configuration_ldap.attribute_maps.* leaf attribute: an
// Optional+Computed nullable string overriding one non-standard LDAP
// attribute name, empty string clearing it back to null (see
// setThreeWayString).
func ldapAttrMapLeaf(description string) schema.StringAttribute {
	return schema.StringAttribute{
		Optional:      true,
		Computed:      true,
		Description:   description + " Null (default) uses the RFC2307/RFC2307BIS standard attribute name. An empty string clears an override back to null.",
		PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
	}
}

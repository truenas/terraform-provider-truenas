package directoryservices

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the TrueNAS SCALE directory services configuration (directoryservices.config) — " +
			"joining the system to an Active Directory domain. This is a singleton resource: there is exactly " +
			"one directory services configuration per TrueNAS system, so it is never created or deleted on " +
			"TrueNAS; Terraform create/update calls directoryservices.update (a job — joining or leaving a " +
			"domain can take a while), and after an enabling update this provider additionally polls " +
			"directoryservices.status until it reports HEALTHY (up to 5 minutes) before considering the apply " +
			"successful. DELETE NEVER CALLS directoryservices.leave: destroying this resource calls " +
			"directoryservices.update with enable=false only, which disables directory services locally without " +
			"leaving the domain or removing the TrueNAS computer account from the domain controller. This is " +
			"deliberate — leaving a domain requires a domain administrator credential that is not necessarily " +
			"available (or desirable to hold) at destroy time. Only Active Directory (service_type = " +
			"\"ACTIVEDIRECTORY\") is supported by this resource; IPA and LDAP directory services are not yet " +
			"implemented.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"directoryservices\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"service_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "The directory service type to join. Only \"ACTIVEDIRECTORY\" is supported by this " +
					"provider version; IPA and LDAP are deferred to a future release.",
				Validators:    []validator.String{stringvalidator.OneOf("ACTIVEDIRECTORY")},
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"enable": schema.BoolAttribute{
				Required: true,
				Description: "Enable the directory service. Setting this to true when TrueNAS has never joined " +
					"the configured domain causes TrueNAS to attempt to join it — creating a computer account " +
					"and DNS records on the domain controller. Setting this to false disables directory " +
					"services locally without leaving the domain. Destroying this resource always sends " +
					"enable=false (see the resource-level description); it never leaves the domain.",
			},
			"enable_account_cache": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Enable backend caching for user and group lists from the directory service. " +
					"Defaults to true on TrueNAS.",
			},
			"enable_dns_updates": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Enable automatic DNS updates for the TrueNAS server in the domain via nsupdate " +
					"and GSSAPI/TSIG. Defaults to true on TrueNAS.",
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
					"When joining Active Directory for the first time, the realm is normally detected and " +
					"configured automatically if left unset. An empty string clears the assigned realm (null " +
					"on the wire).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"credential": schema.SingleNestedAttribute{
				Optional: true,
				Description: "Credential used to join the domain. REQUIRED the first time \"enable\" is set to " +
					"true — TrueNAS needs real domain administrator credentials to perform the join. It may be " +
					"omitted from configuration on later applies: once joined, TrueNAS itself swaps this " +
					"credential for a keytab-backed machine-account credential internally, and this provider " +
					"automatically resends THAT (not the original password) on every subsequent update that " +
					"keeps directory services enabled, including the eventual disable. NOTE: credential_type " +
					"and username ARE persisted in Terraform state (they are not sensitive on their own); " +
					"password is WriteOnly (Sensitive, requires Terraform >= 1.11) and is NEVER read back from " +
					"TrueNAS or stored in state.",
				Attributes: map[string]schema.Attribute{
					"credential_type": schema.StringAttribute{
						Required: true,
						Description: "Credential type. Only \"KERBEROS_USER\" is supported by this provider " +
							"version (KERBEROS_PRINCIPAL, and the LDAP-only credential types, are deferred).",
						Validators: []validator.String{stringvalidator.OneOf("KERBEROS_USER")},
					},
					"username": schema.StringAttribute{
						Required: true,
						Description: "Username of the domain account used to obtain a Kerberos ticket for " +
							"joining/authenticating to the directory service. This account must exist on the " +
							"domain controller and (for the initial join) typically needs domain administrator " +
							"privileges.",
					},
					"password": schema.StringAttribute{
						Required:  true,
						Sensitive: true,
						WriteOnly: true,
						Description: "Password for the account named in \"username\". Write-only: never stored " +
							"in Terraform state. Requires Terraform >= 1.11.",
					},
				},
			},
			"configuration_activedirectory": schema.SingleNestedAttribute{
				Optional: true,
				Computed: true,
				Description: "Active Directory join configuration. REQUIRED whenever \"enable\" is true — see " +
					"\"credential\" for why this must be supplied on every apply for as long as enable stays " +
					"true, not only the first time. \"idmap\" and \"trusted_domains\" are not exposed by this " +
					"provider version — TrueNAS applies its standard defaults (RID idmap backend with the " +
					"built-in range; no trusted domains) on first join, and this provider preserves whatever " +
					"values TrueNAS has already assigned on every subsequent update.",
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
					},
					"enable_trusted_domains": schema.BoolAttribute{
						Optional: true,
						Computed: true,
						Description: "Enable support for trusted domains. Defaults to false on TrueNAS. This " +
							"provider version does not expose per-trusted-domain idmap configuration.",
					},
				},
			},
		},
	}
}

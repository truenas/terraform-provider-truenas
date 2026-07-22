package directoryservices

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// directoryServicesResourceID is the fixed Terraform ID for this singleton
// resource: there is exactly one directory services configuration per
// TrueNAS system (directoryservices.config always returns a single record),
// and it is never created or deleted on TrueNAS itself — only enabled or
// disabled via directoryservices.update. The API's own numeric "id" (probed:
// always 1) is an internal implementation detail and is intentionally not
// surfaced in the model, mirroring the kerberos_config/network_config
// singleton pattern.
const directoryServicesResourceID = "directoryservices"

// credentialAttrTypes describes the attribute types of the nested
// "credential" object. Only the KERBEROS_USER shape is modeled in v1
// (LDAP_PLAIN/LDAP_ANONYMOUS/LDAP_MTLS/KERBEROS_PRINCIPAL are deferred along
// with LDAP/IPA service_type support).
var credentialAttrTypes = map[string]attr.Type{
	"credential_type": types.StringType,
	"username":        types.StringType,
	"password":        types.StringType,
}

// CredentialModel maps to the nested "credential" attribute. Password is
// WriteOnly (see schema.go): it is sourced from req.Config in Create/Update
// (never req.Plan, which the framework nulls for WriteOnly attributes) and
// is never read back from the API into state — see responseToModel and
// updatePayload.
type CredentialModel struct {
	CredentialType types.String `tfsdk:"credential_type"`
	Username       types.String `tfsdk:"username"`
	Password       types.String `tfsdk:"password"`
}

// adConfigAttrTypes describes the attribute types of the nested
// "configuration_activedirectory" object.
var adConfigAttrTypes = map[string]attr.Type{
	"hostname":               types.StringType,
	"domain":                 types.StringType,
	"site":                   types.StringType,
	"computer_account_ou":    types.StringType,
	"use_default_domain":     types.BoolType,
	"enable_trusted_domains": types.BoolType,
}

// ActiveDirectoryConfigModel maps to the nested
// "configuration_activedirectory" attribute. This is a deliberately partial
// mirror of the probed ActiveDirectoryConfig wire shape: "idmap" and
// "trusted_domains" are NOT modeled in v1 — both have sane server-side
// defaults (idmap: RID backend with the standard TrueNAS-assigned range;
// trusted_domains: empty list) that apply automatically whenever
// "configuration" is sent without them, so omitting them from the Terraform
// schema simply means "use the TrueNAS defaults" rather than losing
// functionality. A future schema version can add them.
type ActiveDirectoryConfigModel struct {
	Hostname             types.String `tfsdk:"hostname"`
	Domain               types.String `tfsdk:"domain"`
	Site                 types.String `tfsdk:"site"`
	ComputerAccountOU    types.String `tfsdk:"computer_account_ou"`
	UseDefaultDomain     types.Bool   `tfsdk:"use_default_domain"`
	EnableTrustedDomains types.Bool   `tfsdk:"enable_trusted_domains"`
}

// DirectoryServicesModel is the Terraform state model for
// truenas_directoryservices.
type DirectoryServicesModel struct {
	ID                           types.String `tfsdk:"id"` // fixed: "directoryservices"
	ServiceType                  types.String `tfsdk:"service_type"`
	Enable                       types.Bool   `tfsdk:"enable"`
	EnableAccountCache           types.Bool   `tfsdk:"enable_account_cache"`
	EnableDNSUpdates             types.Bool   `tfsdk:"enable_dns_updates"`
	Timeout                      types.Int64  `tfsdk:"timeout"`
	KerberosRealm                types.String `tfsdk:"kerberos_realm"`
	Credential                   types.Object `tfsdk:"credential"`                    // CredentialModel; WriteOnly password inside
	ConfigurationActiveDirectory types.Object `tfsdk:"configuration_activedirectory"` // ActiveDirectoryConfigModel
}

// DirectoryServicesDataSourceModel is the read-only model for the
// truenas_directoryservices datasource. It omits "credential" entirely
// (there is no non-sensitive substitute worth exposing: credential_type and
// username carry little value without ever being writable here, and the
// password must never be readable) and adds "status"/"status_msg" from
// directoryservices.status.
type DirectoryServicesDataSourceModel struct {
	ID                           types.String `tfsdk:"id"`
	ServiceType                  types.String `tfsdk:"service_type"`
	Enable                       types.Bool   `tfsdk:"enable"`
	EnableAccountCache           types.Bool   `tfsdk:"enable_account_cache"`
	EnableDNSUpdates             types.Bool   `tfsdk:"enable_dns_updates"`
	Timeout                      types.Int64  `tfsdk:"timeout"`
	KerberosRealm                types.String `tfsdk:"kerberos_realm"`
	ConfigurationActiveDirectory types.Object `tfsdk:"configuration_activedirectory"`
	Status                       types.String `tfsdk:"status"`
	StatusMsg                    types.String `tfsdk:"status_msg"`
}

// directoryServicesConfigAPI is the JSON wire format of the nested
// "configuration" object when service_type is ACTIVEDIRECTORY (the
// ActiveDirectoryConfig variant of the discriminated union probed on
// directoryservices.config/update). hostname/domain are non-nullable
// required strings on this variant; site/computer_account_ou are nullable;
// use_default_domain/enable_trusted_domains are non-nullable booleans with
// server-side defaults of false.
//
// Idmap/TrustedDomains are decoded ONLY as opaque json.RawMessage: they are
// never surfaced in ActiveDirectoryConfigModel/Terraform state (v1 scope),
// but MUST be round-tripped verbatim into any outgoing "configuration" the
// provider sends while directory services are already joined — see
// updatePayload's doc comment for why (TrueNAS rejects an update whose
// configuration differs at all from what's currently persisted while
// enabled, and an outgoing payload that omits idmap/trusted_domains is
// treated as differing, since it would fall back to their JSON-schema
// defaults instead of the real, TrueNAS-assigned values).
type directoryServicesConfigAPI struct {
	ServiceType          string          `json:"service_type"`
	Hostname             string          `json:"hostname"`
	Domain               string          `json:"domain"`
	Site                 *string         `json:"site"`
	ComputerAccountOU    *string         `json:"computer_account_ou"`
	UseDefaultDomain     bool            `json:"use_default_domain"`
	EnableTrustedDomains bool            `json:"enable_trusted_domains"`
	Idmap                json.RawMessage `json:"idmap"`
	TrustedDomains       json.RawMessage `json:"trusted_domains"`
}

// directoryServicesCredentialAPI is the JSON wire format of the nested
// "credential" object, decoded ONLY for its non-secret credential_type/
// principal fields (the KERBEROS_PRINCIPAL shape). Confirmed live: TrueNAS
// itself swaps a raw KERBEROS_USER admin credential supplied for a join
// into this machine-account-keytab-backed KERBEROS_PRINCIPAL form
// immediately after the join succeeds, and requires it (not the original
// admin credential) on every subsequent update that keeps directory
// services enabled — see updatePayload's credentialPayload helper.
//
// This struct deliberately has NO field for "username"/"password": even
// though a live response CAN include them (observed after an update whose
// job itself failed, an anomalous non-swapped state — see
// directoryServicesAPI's doc comment), decoding them here would create a
// path for a real admin password to flow through provider Go values, which
// this resource must never do regardless of whether it ends up in
// Terraform state.
type directoryServicesCredentialAPI struct {
	CredentialType string  `json:"credential_type"`
	Principal      *string `json:"principal"`
}

// directoryServicesAPI mirrors the JSON object returned by
// directoryservices.config and (as its job result) directoryservices.update.
// Probed against a live TrueNAS SCALE 25.10 box (`core.get_methods` for
// directoryservices.config/update): "id" and "enable" are always present as
// non-nullable int/bool; "service_type", "kerberos_realm", "credential", and
// "configuration" are nullable (null on a never-configured box, as
// confirmed live: {"configuration":null,"credential":null,"enable":false,
// "enable_account_cache":true,"enable_dns_updates":true,"id":1,
// "kerberos_realm":null,"service_type":null,"timeout":10}).
type directoryServicesAPI struct {
	ID                 int64                           `json:"id"`
	ServiceType        *string                         `json:"service_type"`
	Enable             bool                            `json:"enable"`
	EnableAccountCache bool                            `json:"enable_account_cache"`
	EnableDNSUpdates   bool                            `json:"enable_dns_updates"`
	Timeout            int64                           `json:"timeout"`
	KerberosRealm      *string                         `json:"kerberos_realm"`
	Credential         *directoryServicesCredentialAPI `json:"credential"`
	Configuration      *directoryServicesConfigAPI     `json:"configuration"`
}

// directoryServicesStatusAPI mirrors the JSON object returned by
// directoryservices.status: {"type": ..., "status": ..., "status_msg": ...}.
// "type" is required but nullable (null when disabled); "status" and
// "status_msg" are both nullable, confirmed live on a disabled box:
// {"status":null,"status_msg":null,"type":null}.
type directoryServicesStatusAPI struct {
	Type      *string `json:"type"`
	Status    *string `json:"status"`
	StatusMsg *string `json:"status_msg"`
}

// stringPtrOrEmpty maps a nullable wire string to a non-null Terraform
// string: nil -> "", otherwise the pointed-to value. Mirrors
// network_config's stringOrEmpty helper; see updatePayload for the
// corresponding three-way write-side convention (empty string clears the
// value on TrueNAS).
func stringPtrOrEmpty(v *string) types.String {
	if v == nil {
		return types.StringValue("")
	}
	return types.StringValue(*v)
}

// existingKerberosPrincipal extracts a reusable KERBEROS_PRINCIPAL from a
// previously-fetched directoryServicesAPI, or nil if none is available
// (never joined, or the stored credential is some other shape — e.g. a
// raw, non-swapped KERBEROS_USER, which is never reused; see
// directoryServicesCredentialAPI's doc comment).
func existingKerberosPrincipal(existing *directoryServicesAPI) *string {
	if existing == nil || existing.Credential == nil {
		return nil
	}
	if existing.Credential.CredentialType != "KERBEROS_PRINCIPAL" {
		return nil
	}
	if existing.Credential.Principal == nil || *existing.Credential.Principal == "" {
		return nil
	}
	return existing.Credential.Principal
}

// responseToModel maps a directoryServicesAPI response onto a
// DirectoryServicesModel. Credential is NEVER set here — it is write-only
// and must retain whatever value was already present in m (the plan/prior
// state), not a value derived from the API response.
func responseToModel(ctx context.Context, api *directoryServicesAPI, m *DirectoryServicesModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(directoryServicesResourceID)

	if api.ServiceType != nil {
		m.ServiceType = types.StringValue(*api.ServiceType)
	} else {
		m.ServiceType = types.StringNull()
	}

	m.Enable = types.BoolValue(api.Enable)
	m.EnableAccountCache = types.BoolValue(api.EnableAccountCache)
	m.EnableDNSUpdates = types.BoolValue(api.EnableDNSUpdates)
	m.Timeout = types.Int64Value(api.Timeout)
	m.KerberosRealm = stringPtrOrEmpty(api.KerberosRealm)

	if api.ServiceType != nil && *api.ServiceType == "ACTIVEDIRECTORY" && api.Configuration != nil {
		adModel := ActiveDirectoryConfigModel{
			Hostname:             types.StringValue(api.Configuration.Hostname),
			Domain:               types.StringValue(api.Configuration.Domain),
			Site:                 stringPtrOrEmpty(api.Configuration.Site),
			ComputerAccountOU:    stringPtrOrEmpty(api.Configuration.ComputerAccountOU),
			UseDefaultDomain:     types.BoolValue(api.Configuration.UseDefaultDomain),
			EnableTrustedDomains: types.BoolValue(api.Configuration.EnableTrustedDomains),
		}
		obj, d := types.ObjectValueFrom(ctx, adConfigAttrTypes, adModel)
		diags.Append(d...)
		m.ConfigurationActiveDirectory = obj
	} else {
		m.ConfigurationActiveDirectory = types.ObjectNull(adConfigAttrTypes)
	}

	return diags
}

// responseToDataSourceModel maps a directoryServicesAPI response plus a
// directoryServicesStatusAPI response onto a DirectoryServicesDataSourceModel.
func responseToDataSourceModel(ctx context.Context, api *directoryServicesAPI, status *directoryServicesStatusAPI, m *DirectoryServicesDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(directoryServicesResourceID)

	if api.ServiceType != nil {
		m.ServiceType = types.StringValue(*api.ServiceType)
	} else {
		m.ServiceType = types.StringNull()
	}

	m.Enable = types.BoolValue(api.Enable)
	m.EnableAccountCache = types.BoolValue(api.EnableAccountCache)
	m.EnableDNSUpdates = types.BoolValue(api.EnableDNSUpdates)
	m.Timeout = types.Int64Value(api.Timeout)
	m.KerberosRealm = stringPtrOrEmpty(api.KerberosRealm)

	if api.ServiceType != nil && *api.ServiceType == "ACTIVEDIRECTORY" && api.Configuration != nil {
		adModel := ActiveDirectoryConfigModel{
			Hostname:             types.StringValue(api.Configuration.Hostname),
			Domain:               types.StringValue(api.Configuration.Domain),
			Site:                 stringPtrOrEmpty(api.Configuration.Site),
			ComputerAccountOU:    stringPtrOrEmpty(api.Configuration.ComputerAccountOU),
			UseDefaultDomain:     types.BoolValue(api.Configuration.UseDefaultDomain),
			EnableTrustedDomains: types.BoolValue(api.Configuration.EnableTrustedDomains),
		}
		obj, d := types.ObjectValueFrom(ctx, adConfigAttrTypes, adModel)
		diags.Append(d...)
		m.ConfigurationActiveDirectory = obj
	} else {
		m.ConfigurationActiveDirectory = types.ObjectNull(adConfigAttrTypes)
	}

	if status.Status != nil {
		m.Status = types.StringValue(*status.Status)
	} else {
		m.Status = types.StringNull()
	}
	if status.StatusMsg != nil {
		m.StatusMsg = types.StringValue(*status.StatusMsg)
	} else {
		m.StatusMsg = types.StringNull()
	}

	return diags
}

// updatePayload builds the directoryservices.update argument from a plan
// that has already had its write-only "credential" sourced from req.Config
// (see resource.go Create/Update). Its shape is dictated entirely by
// m.Enable, per live testing against a SCALE 25.10 box that repeatedly
// contradicted the JSON schema's own "_required_": false on every
// top-level field:
//
//   - m.Enable == false: TrueNAS accepts (and this resource's Delete relies
//     on) a minimal payload of JUST enable/enable_account_cache/
//     enable_dns_updates/timeout — service_type, kerberos_realm,
//     credential, and configuration are always omitted. Confirmed live:
//     {"enable": false} alone cleanly disables an active join.
//   - m.Enable == true: the middleware has model-level validators (not
//     visible in the JSON schema) that cascade: any payload with enable
//     true requires "service_type" ("[EINVAL] directoryservices_update:
//     Value error, service_type is required in update payloads"), which in
//     turn requires BOTH "credential" ("Explicit credential configuration
//     is required when service_type is specified") AND "configuration"
//     ("Explicit configuration is required when service_type is
//     specified") to be present and non-null — even for an in-place update
//     that only intends to bump "timeout" while already joined. In other
//     words, TrueNAS re-validates the full join identity on every call
//     that keeps directory services enabled, not just the first one.
//     "configuration" must otherwise be byte-for-byte the currently
//     persisted value for any field the caller isn't intentionally
//     changing — including "idmap"/"trusted_domains", which this
//     provider's schema does not expose — or the job fails with
//     "[EINVAL] directoryservices.update.configuration: Permitted changes
//     while directory services are enabled are limited to account
//     caching, DNS updates, and timeouts." See the existing parameter.
//     "credential" has the same "must match" behavior but with a twist:
//     TrueNAS swaps a raw KERBEROS_USER admin credential used for the
//     initial join into a KERBEROS_PRINCIPAL backed by the new machine
//     account's own keytab, and then REJECTS resending the raw
//     KERBEROS_USER credential on a later call — confirmed live, the
//     disabling call specifically fails with "[EINVAL]
//     directoryservices.update.credential.credential_type: Kerberos user
//     credentials may not be stored for disabled directory services..."
//     if the last update re-stored a raw KERBEROS_USER. So once joined,
//     every subsequent call (including eventually disabling) must resend
//     the swapped KERBEROS_PRINCIPAL, not m.Credential — see
//     existingKerberosPrincipal.
//
// existing is the currently-persisted directoryServicesAPI (nil, or with a
// nil Configuration, on a fresh join where none exists yet) — the FULL
// top-level object, not just its nested Configuration: the nested
// "configuration" object returned by directoryservices.config/update does
// NOT echo back its own "service_type" discriminator key (confirmed live —
// directoryServicesConfigAPI.ServiceType decodes to "" from a real
// response), so the top-level ServiceType is the only reliable signal for
// "was this an Active Directory configuration." When existing/
// existing.Configuration is usable and existing's top-level ServiceType is
// ACTIVEDIRECTORY, Configuration's Idmap/TrustedDomains raw JSON is copied
// verbatim into the outgoing "configuration" map so an in-place update
// (e.g. bumping timeout) doesn't inadvertently look like an idmap/
// trusted_domains change. kerberos_realm uses the same three-way guard as
// network_config's nameserver1-3: omitted when null/unknown, sent as JSON
// nil when the model holds an explicit "" (clearing the realm), sent as
// its string value otherwise.
//
// credential is the trickiest of the three: TrueNAS swaps a raw
// KERBEROS_USER admin credential into a machine-account-keytab-backed
// KERBEROS_PRINCIPAL immediately after a successful join, and REJECTS
// resending the raw KERBEROS_USER credential on later update calls with
// "[EINVAL] directoryservices.update.credential.credential_type: Kerberos
// user credentials may not be stored for disabled directory services..." —
// confirmed live specifically on the disabling call, but the swap means
// the raw admin password shouldn't be resent at all once a
// KERBEROS_PRINCIPAL exists. So: when existing has a usable
// KERBEROS_PRINCIPAL credential, that principal is resent verbatim
// (harmless — it carries no secret) instead of m.Credential, and
// m.Credential is only REQUIRED when no such principal exists yet (a
// fresh join, which needs real admin credentials to authenticate).
//
// Returns an error diagnostic (no client call should be attempted) if
// m.Enable is true but ServiceType is null/unknown (TrueNAS itself requires
// it on every enabling update — "[EINVAL] directoryservices_update: Value
// error, service_type is required in update payloads"), if
// ConfigurationActiveDirectory is null/unknown (always mandatory), or if
// Credential is null/unknown AND there's no existing KERBEROS_PRINCIPAL to
// reuse (mandatory only for the first join) — failing fast here gives a
// much clearer message than TrueNAS's own EINVAL.
func (m *DirectoryServicesModel) updatePayload(ctx context.Context, existing *directoryServicesAPI) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{}

	if !m.EnableAccountCache.IsNull() && !m.EnableAccountCache.IsUnknown() {
		p["enable_account_cache"] = m.EnableAccountCache.ValueBool()
	}
	if !m.EnableDNSUpdates.IsNull() && !m.EnableDNSUpdates.IsUnknown() {
		p["enable_dns_updates"] = m.EnableDNSUpdates.ValueBool()
	}
	if !m.Timeout.IsNull() && !m.Timeout.IsUnknown() {
		p["timeout"] = m.Timeout.ValueInt64()
	}

	enable := m.Enable.ValueBool()
	p["enable"] = enable

	if !enable {
		return p, diags
	}

	existingPrincipal := existingKerberosPrincipal(existing)

	if m.ServiceType.IsNull() || m.ServiceType.IsUnknown() {
		diags.AddError(
			"Missing directory services type",
			"\"service_type\" is required whenever \"enable\" is true: TrueNAS rejects an enabling update "+
				"payload that omits it.",
		)
	}
	if (m.Credential.IsNull() || m.Credential.IsUnknown()) && existingPrincipal == nil {
		diags.AddError(
			"Missing directory services credential",
			"\"credential\" is required to join the domain the first time \"enable\" is set to true (there is "+
				"no existing machine-account credential to reuse yet).",
		)
	}
	if m.ConfigurationActiveDirectory.IsNull() || m.ConfigurationActiveDirectory.IsUnknown() {
		diags.AddError(
			"Missing directory services configuration",
			"\"configuration_activedirectory\" is required whenever \"enable\" is true: TrueNAS re-validates "+
				"the domain join identity on every update call that keeps directory services enabled, not "+
				"only the first one.",
		)
	}
	if diags.HasError() {
		return p, diags
	}

	if !m.ServiceType.IsNull() && !m.ServiceType.IsUnknown() {
		p["service_type"] = m.ServiceType.ValueString()
	}
	if !m.KerberosRealm.IsNull() && !m.KerberosRealm.IsUnknown() {
		if v := m.KerberosRealm.ValueString(); v != "" {
			p["kerberos_realm"] = v
		} else {
			p["kerberos_realm"] = nil
		}
	}

	if existingPrincipal != nil {
		// Already joined: resend the machine account's own principal
		// (no secret material) rather than the original admin credential.
		p["credential"] = map[string]any{
			"credential_type": "KERBEROS_PRINCIPAL",
			"principal":       *existingPrincipal,
		}
	} else {
		var cred CredentialModel
		diags.Append(m.Credential.As(ctx, &cred, basetypes.ObjectAsOptions{})...)
		p["credential"] = map[string]any{
			"credential_type": cred.CredentialType.ValueString(),
			"username":        cred.Username.ValueString(),
			"password":        cred.Password.ValueString(),
		}
	}

	var ad ActiveDirectoryConfigModel
	diags.Append(m.ConfigurationActiveDirectory.As(ctx, &ad, basetypes.ObjectAsOptions{})...)

	adPayload := map[string]any{
		"service_type": "ACTIVEDIRECTORY",
		"hostname":     ad.Hostname.ValueString(),
		"domain":       ad.Domain.ValueString(),
	}
	if v := ad.Site.ValueString(); v != "" {
		adPayload["site"] = v
	} else {
		adPayload["site"] = nil
	}
	if v := ad.ComputerAccountOU.ValueString(); v != "" {
		adPayload["computer_account_ou"] = v
	} else {
		adPayload["computer_account_ou"] = nil
	}
	if !ad.UseDefaultDomain.IsNull() && !ad.UseDefaultDomain.IsUnknown() {
		adPayload["use_default_domain"] = ad.UseDefaultDomain.ValueBool()
	}
	if !ad.EnableTrustedDomains.IsNull() && !ad.EnableTrustedDomains.IsUnknown() {
		adPayload["enable_trusted_domains"] = ad.EnableTrustedDomains.ValueBool()
	}
	if existing != nil && existing.ServiceType != nil && *existing.ServiceType == "ACTIVEDIRECTORY" &&
		existing.Configuration != nil {
		if len(existing.Configuration.Idmap) > 0 {
			var idmap any
			if err := json.Unmarshal(existing.Configuration.Idmap, &idmap); err == nil {
				adPayload["idmap"] = idmap
			}
		}
		if len(existing.Configuration.TrustedDomains) > 0 {
			var trustedDomains any
			if err := json.Unmarshal(existing.Configuration.TrustedDomains, &trustedDomains); err == nil {
				adPayload["trusted_domains"] = trustedDomains
			}
		}
	}
	p["configuration"] = adPayload

	return p, diags
}

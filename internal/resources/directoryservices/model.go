// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package directoryservices

import (
	"context"
	"encoding/json"
	"fmt"

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
// "credential" object. This models all five credential_type variants probed
// on directoryservices.update (KERBEROS_USER, KERBEROS_PRINCIPAL, LDAP_PLAIN,
// LDAP_ANONYMOUS, LDAP_MTLS) as one flat object: every field besides
// credential_type is Optional, and updatePayload's credentialPayload/
// validateCredential enforce which ones are actually required for the
// selected credential_type (see their doc comments). password/bindpw are
// WriteOnly (never persisted to state); binddn/client_certificate/principal
// are not secrets on their own and are persisted normally.
var credentialAttrTypes = map[string]attr.Type{
	"credential_type":    types.StringType,
	"username":           types.StringType,
	"password":           types.StringType,
	"binddn":             types.StringType,
	"bindpw":             types.StringType,
	"client_certificate": types.StringType,
	"principal":          types.StringType,
}

// CredentialModel maps to the nested "credential" attribute. password and
// bindpw are WriteOnly (see schema.go): their values are sourced from
// req.Config in Create/Update (never req.Plan, which the framework nulls for
// WriteOnly attributes) and are never read back from the API into state —
// see responseToModel and updatePayload. The other fields
// (binddn/client_certificate/principal) are not sensitive on their own and
// round-trip normally, but the credential API response struct
// (directoryServicesCredentialAPI) deliberately never decodes binddn either,
// since the mapper must NEVER touch "credential" at all — its value always
// carries forward from the plan/config, never from a read.
type CredentialModel struct {
	CredentialType    types.String `tfsdk:"credential_type"`
	Username          types.String `tfsdk:"username"`
	Password          types.String `tfsdk:"password"`
	BindDN            types.String `tfsdk:"binddn"`
	BindPW            types.String `tfsdk:"bindpw"`
	ClientCertificate types.String `tfsdk:"client_certificate"`
	Principal         types.String `tfsdk:"principal"`
}

// idmapBuiltinAttrTypes describes the nested
// "configuration_activedirectory.idmap.builtin" object: the UID/GID range
// used for automatically generated accounts linked to well-known/BUILTIN
// Windows accounts. Probed shape: {name (nullable string), range_low
// (int64), range_high (int64)}, all with server-side defaults
// (90000001-100000000) applied when omitted.
var idmapBuiltinAttrTypes = map[string]attr.Type{
	"name":       types.StringType,
	"range_low":  types.Int64Type,
	"range_high": types.Int64Type,
}

// IdmapBuiltinModel maps to "configuration_activedirectory.idmap.builtin".
type IdmapBuiltinModel struct {
	Name      types.String `tfsdk:"name"`
	RangeLow  types.Int64  `tfsdk:"range_low"`
	RangeHigh types.Int64  `tfsdk:"range_high"`
}

// idmapDomainAttrTypes describes the nested
// "configuration_activedirectory.idmap.idmap_domain" object. The probed API
// shape is a discriminated union on "idmap_backend" with four variants (AD,
// LDAP, RFC2307, RID); this provider version models only AD and RID as an
// explicit Terraform block (see idmapDomainBackendValidator's doc comment
// for why LDAP/RFC2307 are deferred: both require a
// "ldap_user_dn_password" secret, and terraform-plugin-framework forbids a
// Computed nested attribute — required here for back-compat, see idmap's
// schema doc — from containing a WriteOnly descendant at any depth). Fields
// are flattened (mirroring the "credential" block's own flat-plus-validated
// style) rather than split per backend: name/range_low/range_high are
// shared by every backend; schema_mode/unix_primary_group/unix_nss_info
// apply only to AD; sssd_compat only to RID.
var idmapDomainAttrTypes = map[string]attr.Type{
	"idmap_backend":      types.StringType,
	"name":               types.StringType,
	"range_low":          types.Int64Type,
	"range_high":         types.Int64Type,
	"schema_mode":        types.StringType,
	"unix_primary_group": types.BoolType,
	"unix_nss_info":      types.BoolType,
	"sssd_compat":        types.BoolType,
}

// IdmapDomainModel maps to
// "configuration_activedirectory.idmap.idmap_domain".
type IdmapDomainModel struct {
	IdmapBackend     types.String `tfsdk:"idmap_backend"`
	Name             types.String `tfsdk:"name"`
	RangeLow         types.Int64  `tfsdk:"range_low"`
	RangeHigh        types.Int64  `tfsdk:"range_high"`
	SchemaMode       types.String `tfsdk:"schema_mode"`
	UnixPrimaryGroup types.Bool   `tfsdk:"unix_primary_group"`
	UnixNSSInfo      types.Bool   `tfsdk:"unix_nss_info"`
	SSSDCompat       types.Bool   `tfsdk:"sssd_compat"`
}

// idmapAttrTypes describes the nested
// "configuration_activedirectory.idmap" object.
var idmapAttrTypes = map[string]attr.Type{
	"builtin":      types.ObjectType{AttrTypes: idmapBuiltinAttrTypes},
	"idmap_domain": types.ObjectType{AttrTypes: idmapDomainAttrTypes},
}

// IdmapModel maps to "configuration_activedirectory.idmap".
type IdmapModel struct {
	Builtin     types.Object `tfsdk:"builtin"`
	IdmapDomain types.Object `tfsdk:"idmap_domain"`
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
	"idmap":                  types.ObjectType{AttrTypes: idmapAttrTypes},
}

// ActiveDirectoryConfigModel maps to the nested
// "configuration_activedirectory" attribute. "idmap" is now explicitly
// modeled (see IdmapModel); "trusted_domains" remains NOT modeled — it has
// a sane server-side default (empty list) that applies automatically
// whenever "configuration" is sent without it, and this provider continues
// to round-trip its raw JSON verbatim (see updatePayload's doc comment).
type ActiveDirectoryConfigModel struct {
	Hostname             types.String `tfsdk:"hostname"`
	Domain               types.String `tfsdk:"domain"`
	Site                 types.String `tfsdk:"site"`
	ComputerAccountOU    types.String `tfsdk:"computer_account_ou"`
	UseDefaultDomain     types.Bool   `tfsdk:"use_default_domain"`
	EnableTrustedDomains types.Bool   `tfsdk:"enable_trusted_domains"`
	Idmap                types.Object `tfsdk:"idmap"`
}

// ldapSearchBasesAttrTypes describes "configuration_ldap.search_bases":
// alternative LDAP search bases for users/groups/netgroups, each falling
// back to "basedn" when null (the probed default).
var ldapSearchBasesAttrTypes = map[string]attr.Type{
	"base_user":     types.StringType,
	"base_group":    types.StringType,
	"base_netgroup": types.StringType,
}

// LDAPSearchBasesModel maps to "configuration_ldap.search_bases".
type LDAPSearchBasesModel struct {
	BaseUser     types.String `tfsdk:"base_user"`
	BaseGroup    types.String `tfsdk:"base_group"`
	BaseNetgroup types.String `tfsdk:"base_netgroup"`
}

// ldapAttrMapPasswdAttrTypes describes
// "configuration_ldap.attribute_maps.passwd".
var ldapAttrMapPasswdAttrTypes = map[string]attr.Type{
	"user_object_class":   types.StringType,
	"user_name":           types.StringType,
	"user_uid":            types.StringType,
	"user_gid":            types.StringType,
	"user_gecos":          types.StringType,
	"user_home_directory": types.StringType,
	"user_shell":          types.StringType,
}

// LDAPAttrMapPasswdModel maps to
// "configuration_ldap.attribute_maps.passwd".
type LDAPAttrMapPasswdModel struct {
	UserObjectClass   types.String `tfsdk:"user_object_class"`
	UserName          types.String `tfsdk:"user_name"`
	UserUID           types.String `tfsdk:"user_uid"`
	UserGID           types.String `tfsdk:"user_gid"`
	UserGecos         types.String `tfsdk:"user_gecos"`
	UserHomeDirectory types.String `tfsdk:"user_home_directory"`
	UserShell         types.String `tfsdk:"user_shell"`
}

// ldapAttrMapShadowAttrTypes describes
// "configuration_ldap.attribute_maps.shadow".
var ldapAttrMapShadowAttrTypes = map[string]attr.Type{
	"shadow_last_change": types.StringType,
	"shadow_min":         types.StringType,
	"shadow_max":         types.StringType,
	"shadow_warning":     types.StringType,
	"shadow_inactive":    types.StringType,
	"shadow_expire":      types.StringType,
}

// LDAPAttrMapShadowModel maps to
// "configuration_ldap.attribute_maps.shadow".
type LDAPAttrMapShadowModel struct {
	ShadowLastChange types.String `tfsdk:"shadow_last_change"`
	ShadowMin        types.String `tfsdk:"shadow_min"`
	ShadowMax        types.String `tfsdk:"shadow_max"`
	ShadowWarning    types.String `tfsdk:"shadow_warning"`
	ShadowInactive   types.String `tfsdk:"shadow_inactive"`
	ShadowExpire     types.String `tfsdk:"shadow_expire"`
}

// ldapAttrMapGroupAttrTypes describes
// "configuration_ldap.attribute_maps.group".
var ldapAttrMapGroupAttrTypes = map[string]attr.Type{
	"group_object_class": types.StringType,
	"group_gid":          types.StringType,
	"group_member":       types.StringType,
}

// LDAPAttrMapGroupModel maps to "configuration_ldap.attribute_maps.group".
type LDAPAttrMapGroupModel struct {
	GroupObjectClass types.String `tfsdk:"group_object_class"`
	GroupGID         types.String `tfsdk:"group_gid"`
	GroupMember      types.String `tfsdk:"group_member"`
}

// ldapAttrMapNetgroupAttrTypes describes
// "configuration_ldap.attribute_maps.netgroup".
var ldapAttrMapNetgroupAttrTypes = map[string]attr.Type{
	"netgroup_object_class": types.StringType,
	"netgroup_member":       types.StringType,
	"netgroup_triple":       types.StringType,
}

// LDAPAttrMapNetgroupModel maps to
// "configuration_ldap.attribute_maps.netgroup".
type LDAPAttrMapNetgroupModel struct {
	NetgroupObjectClass types.String `tfsdk:"netgroup_object_class"`
	NetgroupMember      types.String `tfsdk:"netgroup_member"`
	NetgroupTriple      types.String `tfsdk:"netgroup_triple"`
}

// ldapAttributeMapsAttrTypes describes "configuration_ldap.attribute_maps":
// optional LDAP attribute mapping for LDAP servers that do not follow
// RFC2307/RFC2307BIS.
var ldapAttributeMapsAttrTypes = map[string]attr.Type{
	"passwd":   types.ObjectType{AttrTypes: ldapAttrMapPasswdAttrTypes},
	"shadow":   types.ObjectType{AttrTypes: ldapAttrMapShadowAttrTypes},
	"group":    types.ObjectType{AttrTypes: ldapAttrMapGroupAttrTypes},
	"netgroup": types.ObjectType{AttrTypes: ldapAttrMapNetgroupAttrTypes},
}

// LDAPAttributeMapsModel maps to "configuration_ldap.attribute_maps".
type LDAPAttributeMapsModel struct {
	Passwd   types.Object `tfsdk:"passwd"`
	Shadow   types.Object `tfsdk:"shadow"`
	Group    types.Object `tfsdk:"group"`
	Netgroup types.Object `tfsdk:"netgroup"`
}

// ldapConfigAttrTypes describes the attribute types of the nested
// "configuration_ldap" object (the LDAPConfig variant of the discriminated
// "configuration" union).
var ldapConfigAttrTypes = map[string]attr.Type{
	"server_urls":           types.ListType{ElemType: types.StringType},
	"basedn":                types.StringType,
	"starttls":              types.BoolType,
	"validate_certificates": types.BoolType,
	"schema":                types.StringType,
	"auxiliary_parameters":  types.StringType,
	"search_bases":          types.ObjectType{AttrTypes: ldapSearchBasesAttrTypes},
	"attribute_maps":        types.ObjectType{AttrTypes: ldapAttributeMapsAttrTypes},
}

// LDAPConfigModel maps to the nested "configuration_ldap" attribute.
// server_urls/basedn are Required (non-nullable on the wire); every other
// field is Optional+Computed with a server-side default applied whenever
// omitted from the outgoing payload (see updatePayload).
type LDAPConfigModel struct {
	ServerURLs           types.List   `tfsdk:"server_urls"`
	BaseDN               types.String `tfsdk:"basedn"`
	StartTLS             types.Bool   `tfsdk:"starttls"`
	ValidateCertificates types.Bool   `tfsdk:"validate_certificates"`
	Schema               types.String `tfsdk:"schema"`
	AuxiliaryParameters  types.String `tfsdk:"auxiliary_parameters"`
	SearchBases          types.Object `tfsdk:"search_bases"`
	AttributeMaps        types.Object `tfsdk:"attribute_maps"`
}

// ipaSMBDomainAttrTypes describes "configuration_ipa.smb_domain": settings
// for the IPA SMB domain, normally detected by TrueNAS during the IPA join
// itself rather than supplied by the caller. idmap_backend is not exposed
// here — it is always the constant "SSS" for this variant (IPA_SMBDomain in
// the probed schema) and is injected automatically by updatePayload
// whenever this block is sent.
var ipaSMBDomainAttrTypes = map[string]attr.Type{
	"name":        types.StringType,
	"range_low":   types.Int64Type,
	"range_high":  types.Int64Type,
	"domain_name": types.StringType,
	"domain_sid":  types.StringType,
}

// IPASMBDomainModel maps to "configuration_ipa.smb_domain".
type IPASMBDomainModel struct {
	Name       types.String `tfsdk:"name"`
	RangeLow   types.Int64  `tfsdk:"range_low"`
	RangeHigh  types.Int64  `tfsdk:"range_high"`
	DomainName types.String `tfsdk:"domain_name"`
	DomainSID  types.String `tfsdk:"domain_sid"`
}

// ipaConfigAttrTypes describes the attribute types of the nested
// "configuration_ipa" object (the IPAConfig variant of the discriminated
// "configuration" union).
var ipaConfigAttrTypes = map[string]attr.Type{
	"target_server":         types.StringType,
	"hostname":              types.StringType,
	"domain":                types.StringType,
	"basedn":                types.StringType,
	"smb_domain":            types.ObjectType{AttrTypes: ipaSMBDomainAttrTypes},
	"validate_certificates": types.BoolType,
}

// IPAConfigModel maps to the nested "configuration_ipa" attribute.
// target_server/hostname/domain/basedn are Required (non-nullable on the
// wire); smb_domain/validate_certificates are Optional+Computed.
type IPAConfigModel struct {
	TargetServer         types.String `tfsdk:"target_server"`
	Hostname             types.String `tfsdk:"hostname"`
	Domain               types.String `tfsdk:"domain"`
	BaseDN               types.String `tfsdk:"basedn"`
	SMBDomain            types.Object `tfsdk:"smb_domain"`
	ValidateCertificates types.Bool   `tfsdk:"validate_certificates"`
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
	Credential                   types.Object `tfsdk:"credential"`                    // CredentialModel; WriteOnly password/bindpw inside
	ConfigurationActiveDirectory types.Object `tfsdk:"configuration_activedirectory"` // ActiveDirectoryConfigModel
	ConfigurationLDAP            types.Object `tfsdk:"configuration_ldap"`            // LDAPConfigModel
	ConfigurationIPA             types.Object `tfsdk:"configuration_ipa"`             // IPAConfigModel
}

// DirectoryServicesDataSourceModel is the read-only model for the
// truenas_directoryservices datasource. It omits "credential" entirely
// (there is no non-sensitive substitute worth exposing: credential_type and
// username carry little value without ever being writable here, and the
// password/bindpw must never be readable) and adds "status"/"status_msg"
// from directoryservices.status.
type DirectoryServicesDataSourceModel struct {
	ID                           types.String `tfsdk:"id"`
	ServiceType                  types.String `tfsdk:"service_type"`
	Enable                       types.Bool   `tfsdk:"enable"`
	EnableAccountCache           types.Bool   `tfsdk:"enable_account_cache"`
	EnableDNSUpdates             types.Bool   `tfsdk:"enable_dns_updates"`
	Timeout                      types.Int64  `tfsdk:"timeout"`
	KerberosRealm                types.String `tfsdk:"kerberos_realm"`
	ConfigurationActiveDirectory types.Object `tfsdk:"configuration_activedirectory"`
	ConfigurationLDAP            types.Object `tfsdk:"configuration_ldap"`
	ConfigurationIPA             types.Object `tfsdk:"configuration_ipa"`
	Status                       types.String `tfsdk:"status"`
	StatusMsg                    types.String `tfsdk:"status_msg"`
}

// directoryServicesIdmapRangeAPI is the JSON wire shape shared by
// "idmap.builtin" (always) and used as the basis for "idmap.idmap_domain"
// (below): a nullable short domain "name" plus a UID/GID range.
type directoryServicesIdmapRangeAPI struct {
	Name      *string `json:"name"`
	RangeLow  int64   `json:"range_low"`
	RangeHigh int64   `json:"range_high"`
}

// directoryServicesIdmapDomainAPI is the JSON wire shape of
// "idmap.idmap_domain": a discriminated union on "idmap_backend" (probed:
// AD, LDAP, RFC2307, RID). Only the fields needed for the AD and RID
// variants are decoded — see idmapDomainAttrTypes's doc comment for why
// LDAP/RFC2307 (which need a secret "ldap_user_dn_password") aren't
// modeled. Decoding is safe regardless of which backend a live box actually
// has: unrecognized/absent fields simply stay at their Go zero value, and
// IdmapBackend is preserved verbatim either way (see
// adIdmapDomainToModel's doc comment for the read-back behavior on an
// unsupported backend).
type directoryServicesIdmapDomainAPI struct {
	IdmapBackend     string  `json:"idmap_backend"`
	Name             *string `json:"name"`
	RangeLow         int64   `json:"range_low"`
	RangeHigh        int64   `json:"range_high"`
	SchemaMode       *string `json:"schema_mode"`        // AD only
	UnixPrimaryGroup bool    `json:"unix_primary_group"` // AD only
	UnixNSSInfo      bool    `json:"unix_nss_info"`      // AD only
	SSSDCompat       bool    `json:"sssd_compat"`        // RID only
}

// directoryServicesADIdmapAPI is the JSON wire shape of
// "configuration.idmap" on the ActiveDirectoryConfig variant.
type directoryServicesADIdmapAPI struct {
	Builtin     *directoryServicesIdmapRangeAPI  `json:"builtin"`
	IdmapDomain *directoryServicesIdmapDomainAPI `json:"idmap_domain"`
}

// directoryServicesIPASMBDomainAPI is the JSON wire shape of
// "configuration.smb_domain" on the IPAConfig variant (IPA_SMBDomain in the
// probed schema). idmap_backend is always the constant "SSS" and is not
// decoded here (see ipaSMBDomainAttrTypes's doc comment).
type directoryServicesIPASMBDomainAPI struct {
	Name       *string `json:"name"`
	RangeLow   int64   `json:"range_low"`
	RangeHigh  int64   `json:"range_high"`
	DomainName *string `json:"domain_name"`
	DomainSID  *string `json:"domain_sid"`
}

// directoryServicesLDAPSearchBasesAPI is the JSON wire shape of
// "configuration.search_bases" on the LDAPConfig variant.
type directoryServicesLDAPSearchBasesAPI struct {
	BaseUser     *string `json:"base_user"`
	BaseGroup    *string `json:"base_group"`
	BaseNetgroup *string `json:"base_netgroup"`
}

// directoryServicesLDAPAttrMapPasswdAPI is the JSON wire shape of
// "configuration.attribute_maps.passwd" on the LDAPConfig variant.
type directoryServicesLDAPAttrMapPasswdAPI struct {
	UserObjectClass   *string `json:"user_object_class"`
	UserName          *string `json:"user_name"`
	UserUID           *string `json:"user_uid"`
	UserGID           *string `json:"user_gid"`
	UserGecos         *string `json:"user_gecos"`
	UserHomeDirectory *string `json:"user_home_directory"`
	UserShell         *string `json:"user_shell"`
}

// directoryServicesLDAPAttrMapShadowAPI is the JSON wire shape of
// "configuration.attribute_maps.shadow" on the LDAPConfig variant.
type directoryServicesLDAPAttrMapShadowAPI struct {
	ShadowLastChange *string `json:"shadow_last_change"`
	ShadowMin        *string `json:"shadow_min"`
	ShadowMax        *string `json:"shadow_max"`
	ShadowWarning    *string `json:"shadow_warning"`
	ShadowInactive   *string `json:"shadow_inactive"`
	ShadowExpire     *string `json:"shadow_expire"`
}

// directoryServicesLDAPAttrMapGroupAPI is the JSON wire shape of
// "configuration.attribute_maps.group" on the LDAPConfig variant.
type directoryServicesLDAPAttrMapGroupAPI struct {
	GroupObjectClass *string `json:"group_object_class"`
	GroupGID         *string `json:"group_gid"`
	GroupMember      *string `json:"group_member"`
}

// directoryServicesLDAPAttrMapNetgroupAPI is the JSON wire shape of
// "configuration.attribute_maps.netgroup" on the LDAPConfig variant.
type directoryServicesLDAPAttrMapNetgroupAPI struct {
	NetgroupObjectClass *string `json:"netgroup_object_class"`
	NetgroupMember      *string `json:"netgroup_member"`
	NetgroupTriple      *string `json:"netgroup_triple"`
}

// directoryServicesLDAPAttributeMapsAPI is the JSON wire shape of
// "configuration.attribute_maps" on the LDAPConfig variant.
type directoryServicesLDAPAttributeMapsAPI struct {
	Passwd   *directoryServicesLDAPAttrMapPasswdAPI   `json:"passwd"`
	Shadow   *directoryServicesLDAPAttrMapShadowAPI   `json:"shadow"`
	Group    *directoryServicesLDAPAttrMapGroupAPI    `json:"group"`
	Netgroup *directoryServicesLDAPAttrMapNetgroupAPI `json:"netgroup"`
}

// directoryServicesConfigAPI is the JSON wire format of the nested
// "configuration" object — the union of all three probed discriminated
// variants (ActiveDirectoryConfig | IPAConfig | LDAPConfig). A single flat
// struct is used rather than one per variant: none of the three variants'
// field names collide with a different Go type for the same wire name
// (hostname/domain are non-nullable strings on both AD and IPA; basedn/
// validate_certificates are shared the same way between IPA and LDAP), and
// the nested object does NOT echo back its own "service_type" discriminator
// key (confirmed live — decodes to "" from a real response), so which
// fields are meaningful is decided entirely by the top-level
// directoryServicesAPI.ServiceType, never by this struct alone. Per-variant
// required/nullable-ness (hostname/domain required on AD/IPA; site/
// computer_account_ou nullable; use_default_domain/enable_trusted_domains
// non-nullable booleans; etc.) is documented per field below.
//
// Idmap is decoded into a typed struct (directoryServicesADIdmapAPI) now
// that it is explicitly modeled in Terraform state — see updatePayload's
// doc comment for why this replaced the old verbatim json.RawMessage
// round-trip. TrustedDomains remains opaque json.RawMessage: it is still
// NOT surfaced in ActiveDirectoryConfigModel (out of scope for this
// version) but MUST continue to be round-tripped verbatim into any
// outgoing AD "configuration" the provider sends while already joined, for
// the same reason idmap originally needed it (see updatePayload).
type directoryServicesConfigAPI struct {
	ServiceType string `json:"service_type"` // always "" on a real response; see doc comment above

	// Shared by ActiveDirectoryConfig and IPAConfig.
	Hostname string `json:"hostname"`
	Domain   string `json:"domain"`

	// Shared by IPAConfig and LDAPConfig.
	BaseDN               string `json:"basedn"`
	ValidateCertificates bool   `json:"validate_certificates"`

	// ActiveDirectoryConfig only.
	Site                 *string                      `json:"site"`
	ComputerAccountOU    *string                      `json:"computer_account_ou"`
	UseDefaultDomain     bool                         `json:"use_default_domain"`
	EnableTrustedDomains bool                         `json:"enable_trusted_domains"`
	Idmap                *directoryServicesADIdmapAPI `json:"idmap"`
	TrustedDomains       json.RawMessage              `json:"trusted_domains"`

	// IPAConfig only.
	TargetServer string                            `json:"target_server"`
	SMBDomain    *directoryServicesIPASMBDomainAPI `json:"smb_domain"`

	// LDAPConfig only.
	ServerURLs          []string                               `json:"server_urls"`
	StartTLS            bool                                   `json:"starttls"`
	Schema              string                                 `json:"schema"`
	SearchBases         *directoryServicesLDAPSearchBasesAPI   `json:"search_bases"`
	AttributeMaps       *directoryServicesLDAPAttributeMapsAPI `json:"attribute_maps"`
	AuxiliaryParameters *string                                `json:"auxiliary_parameters"`
}

// directoryServicesCredentialAPI is the JSON wire format of the nested
// "credential" object, decoded ONLY for its non-secret credential_type/
// principal fields (the KERBEROS_PRINCIPAL shape). Confirmed live: TrueNAS
// itself swaps a raw KERBEROS_USER admin credential supplied for a join
// into this machine-account-keytab-backed KERBEROS_PRINCIPAL form
// immediately after the join succeeds, and requires it (not the original
// admin credential) on every subsequent update that keeps directory
// services enabled — see updatePayload's credentialPayload helper. The same
// Kerberos ticket-based swap applies to IPA joins (both AD and IPA
// authenticate via Kerberos); LDAP joins use LDAP_PLAIN/LDAP_ANONYMOUS/
// LDAP_MTLS credentials directly with no such swap.
//
// This struct deliberately has NO field for "username"/"password"/"binddn"/
// "bindpw"/"client_certificate": even though a live response CAN include
// some of them (observed after an update whose job itself failed, an
// anomalous non-swapped state — see directoryServicesAPI's doc comment),
// decoding them here would create a path for real secret/identifying
// material to flow through provider Go values, which this resource must
// never do regardless of whether it ends up in Terraform state. The mapper
// (responseToModel/responseToDataSourceModel) never touches "credential" at
// all — see their doc comments.
type directoryServicesCredentialAPI struct {
	CredentialType string  `json:"credential_type"`
	Principal      *string `json:"principal"`
}

// directoryServicesAPI mirrors the JSON object returned by
// directoryservices.config and (as its job result) directoryservices.update.
// Probed against live TrueNAS 25.10 and 26.0 boxes (`core.get_methods`
// for directoryservices.config/update — see task-1-report.md for the full
// verbatim diff): "id" and "enable" are always present as non-nullable
// int/bool; "service_type", "kerberos_realm", "credential", and
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
// network_config's stringOrEmpty helper; see setThreeWayString for the
// corresponding write-side convention (empty string clears the value on
// TrueNAS).
func stringPtrOrEmpty(v *string) types.String {
	if v == nil {
		return types.StringValue("")
	}
	return types.StringValue(*v)
}

// setThreeWayString applies the null/omitted, unknown/omitted,
// explicit-empty/nil, value/value convention (mirrors kerberos_realm's
// nameserver1-3 and this package's own site/computer_account_ou) to a
// nullable string field being written into an outgoing payload map: when v
// is null or unknown the key is omitted entirely (server keeps its current
// value/default); an explicit empty string clears the value (sent as JSON
// null); any other value is sent as-is.
func setThreeWayString(p map[string]any, key string, v types.String) {
	if v.IsNull() || v.IsUnknown() {
		return
	}
	if s := v.ValueString(); s != "" {
		p[key] = s
	} else {
		p[key] = nil
	}
}

// setOptionalBool sets p[key] to v's value when v is known and non-null,
// and otherwise omits the key entirely (server-side default applies).
func setOptionalBool(p map[string]any, key string, v types.Bool) {
	if !v.IsNull() && !v.IsUnknown() {
		p[key] = v.ValueBool()
	}
}

// setOptionalInt64 sets p[key] to v's value when v is known and non-null,
// and otherwise omits the key entirely (server-side default applies).
func setOptionalInt64(p map[string]any, key string, v types.Int64) {
	if !v.IsNull() && !v.IsUnknown() {
		p[key] = v.ValueInt64()
	}
}

// setOptionalString sets p[key] to v's value when v is known and non-null,
// and otherwise omits the key entirely. Unlike setThreeWayString, an empty
// string is sent as an empty string, not translated to nil: used for
// Required (never-null) string fields that are merely conditionally
// included in a sub-object (e.g. idmap fields, only sent when their parent
// block is set at all).
func setOptionalString(p map[string]any, key string, v types.String) {
	if !v.IsNull() && !v.IsUnknown() {
		p[key] = v.ValueString()
	}
}

// existingKerberosPrincipal extracts a reusable KERBEROS_PRINCIPAL from a
// previously-fetched directoryServicesAPI, or nil if none is available
// (never joined, or the stored credential is some other shape — e.g. a
// raw, non-swapped KERBEROS_USER, which is never reused; see
// directoryServicesCredentialAPI's doc comment). Applies equally to
// ActiveDirectory and IPA joins (both swap to a machine-account
// KERBEROS_PRINCIPAL after success); LDAP joins never populate this shape.
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

// needsServiceTypeReset reports whether existing's persisted service_type
// differs from plan's about-to-be-applied one (both non-empty/known) —
// i.e. this update is switching the singleton from one directory service
// type to another (e.g. a previous ACTIVEDIRECTORY/IPA join, now
// re-pointing at LDAP). See DirectoryServicesResource.resetStaleServiceType
// in resource.go for why the caller must clear stale state via a
// preliminary "nuke" update before sending the real one whenever this
// returns true: TrueNAS's own directoryservices.update has an internal
// directoryservices.reset() call meant to handle exactly this situation,
// but it does not reliably clear kerberos_realm/credential on a live box
// (confirmed live on TrueNAS 25.10 — see resetStaleServiceType's doc
// comment), so this provider must not rely on it.
func needsServiceTypeReset(plan *DirectoryServicesModel, existing *directoryServicesAPI) bool {
	if existing == nil || existing.ServiceType == nil || *existing.ServiceType == "" {
		return false
	}
	if plan.ServiceType.IsNull() || plan.ServiceType.IsUnknown() {
		return false
	}
	return *existing.ServiceType != plan.ServiceType.ValueString()
}

// adIdmapToModel builds a types.Object (idmapAttrTypes) from a probed
// directoryServicesADIdmapAPI, or types.ObjectNull(idmapAttrTypes) if api is
// nil (no idmap configuration returned — shouldn't normally happen once
// joined, since TrueNAS always assigns idmap defaults on first join, but
// handled defensively).
func adIdmapToModel(ctx context.Context, api *directoryServicesADIdmapAPI) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if api == nil {
		return types.ObjectNull(idmapAttrTypes), diags
	}

	builtinObj := types.ObjectNull(idmapBuiltinAttrTypes)
	if api.Builtin != nil {
		var d diag.Diagnostics
		builtinObj, d = types.ObjectValueFrom(ctx, idmapBuiltinAttrTypes, IdmapBuiltinModel{
			Name:      stringPtrOrEmptyNullable(api.Builtin.Name),
			RangeLow:  types.Int64Value(api.Builtin.RangeLow),
			RangeHigh: types.Int64Value(api.Builtin.RangeHigh),
		})
		diags.Append(d...)
	}

	domainObj := types.ObjectNull(idmapDomainAttrTypes)
	if api.IdmapDomain != nil {
		dm := api.IdmapDomain
		var d diag.Diagnostics
		domainObj, d = types.ObjectValueFrom(ctx, idmapDomainAttrTypes, IdmapDomainModel{
			IdmapBackend:     types.StringValue(dm.IdmapBackend),
			Name:             stringPtrOrEmptyNullable(dm.Name),
			RangeLow:         types.Int64Value(dm.RangeLow),
			RangeHigh:        types.Int64Value(dm.RangeHigh),
			SchemaMode:       stringPtrOrEmptyNullable(dm.SchemaMode),
			UnixPrimaryGroup: types.BoolValue(dm.UnixPrimaryGroup),
			UnixNSSInfo:      types.BoolValue(dm.UnixNSSInfo),
			SSSDCompat:       types.BoolValue(dm.SSSDCompat),
		})
		diags.Append(d...)
	}

	obj, d := types.ObjectValueFrom(ctx, idmapAttrTypes, IdmapModel{
		Builtin:     builtinObj,
		IdmapDomain: domainObj,
	})
	diags.Append(d...)
	return obj, diags
}

// stringPtrOrEmptyNullable maps a nullable wire string to a nullable
// Terraform string: nil stays null (unlike stringPtrOrEmpty's nil->"",
// used for the top-level three-way clearable fields). idmap's "name" is
// genuinely optional/absent rather than "clearable to empty" — there is no
// clearing convention for it in updatePayload (it's conditionally included,
// not three-way), so null-in/null-out is the simpler, accurate mapping.
func stringPtrOrEmptyNullable(v *string) types.String {
	if v == nil {
		return types.StringNull()
	}
	return types.StringValue(*v)
}

// ldapConfigToModel builds a types.Object (ldapConfigAttrTypes) from a
// probed directoryServicesConfigAPI (only valid when the top-level
// ServiceType is "LDAP" — see responseToModel).
func ldapConfigToModel(ctx context.Context, api *directoryServicesConfigAPI) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics

	serverURLs, d := types.ListValueFrom(ctx, types.StringType, api.ServerURLs)
	diags.Append(d...)

	searchBases := types.ObjectNull(ldapSearchBasesAttrTypes)
	if api.SearchBases != nil {
		searchBases, d = types.ObjectValueFrom(ctx, ldapSearchBasesAttrTypes, LDAPSearchBasesModel{
			BaseUser:     stringPtrOrEmptyNullable(api.SearchBases.BaseUser),
			BaseGroup:    stringPtrOrEmptyNullable(api.SearchBases.BaseGroup),
			BaseNetgroup: stringPtrOrEmptyNullable(api.SearchBases.BaseNetgroup),
		})
		diags.Append(d...)
	}

	attrMaps := types.ObjectNull(ldapAttributeMapsAttrTypes)
	if api.AttributeMaps != nil {
		am := api.AttributeMaps

		passwd := types.ObjectNull(ldapAttrMapPasswdAttrTypes)
		if am.Passwd != nil {
			passwd, d = types.ObjectValueFrom(ctx, ldapAttrMapPasswdAttrTypes, LDAPAttrMapPasswdModel{
				UserObjectClass:   stringPtrOrEmptyNullable(am.Passwd.UserObjectClass),
				UserName:          stringPtrOrEmptyNullable(am.Passwd.UserName),
				UserUID:           stringPtrOrEmptyNullable(am.Passwd.UserUID),
				UserGID:           stringPtrOrEmptyNullable(am.Passwd.UserGID),
				UserGecos:         stringPtrOrEmptyNullable(am.Passwd.UserGecos),
				UserHomeDirectory: stringPtrOrEmptyNullable(am.Passwd.UserHomeDirectory),
				UserShell:         stringPtrOrEmptyNullable(am.Passwd.UserShell),
			})
			diags.Append(d...)
		}

		shadow := types.ObjectNull(ldapAttrMapShadowAttrTypes)
		if am.Shadow != nil {
			shadow, d = types.ObjectValueFrom(ctx, ldapAttrMapShadowAttrTypes, LDAPAttrMapShadowModel{
				ShadowLastChange: stringPtrOrEmptyNullable(am.Shadow.ShadowLastChange),
				ShadowMin:        stringPtrOrEmptyNullable(am.Shadow.ShadowMin),
				ShadowMax:        stringPtrOrEmptyNullable(am.Shadow.ShadowMax),
				ShadowWarning:    stringPtrOrEmptyNullable(am.Shadow.ShadowWarning),
				ShadowInactive:   stringPtrOrEmptyNullable(am.Shadow.ShadowInactive),
				ShadowExpire:     stringPtrOrEmptyNullable(am.Shadow.ShadowExpire),
			})
			diags.Append(d...)
		}

		group := types.ObjectNull(ldapAttrMapGroupAttrTypes)
		if am.Group != nil {
			group, d = types.ObjectValueFrom(ctx, ldapAttrMapGroupAttrTypes, LDAPAttrMapGroupModel{
				GroupObjectClass: stringPtrOrEmptyNullable(am.Group.GroupObjectClass),
				GroupGID:         stringPtrOrEmptyNullable(am.Group.GroupGID),
				GroupMember:      stringPtrOrEmptyNullable(am.Group.GroupMember),
			})
			diags.Append(d...)
		}

		netgroup := types.ObjectNull(ldapAttrMapNetgroupAttrTypes)
		if am.Netgroup != nil {
			netgroup, d = types.ObjectValueFrom(ctx, ldapAttrMapNetgroupAttrTypes, LDAPAttrMapNetgroupModel{
				NetgroupObjectClass: stringPtrOrEmptyNullable(am.Netgroup.NetgroupObjectClass),
				NetgroupMember:      stringPtrOrEmptyNullable(am.Netgroup.NetgroupMember),
				NetgroupTriple:      stringPtrOrEmptyNullable(am.Netgroup.NetgroupTriple),
			})
			diags.Append(d...)
		}

		attrMaps, d = types.ObjectValueFrom(ctx, ldapAttributeMapsAttrTypes, LDAPAttributeMapsModel{
			Passwd:   passwd,
			Shadow:   shadow,
			Group:    group,
			Netgroup: netgroup,
		})
		diags.Append(d...)
	}

	obj, d := types.ObjectValueFrom(ctx, ldapConfigAttrTypes, LDAPConfigModel{
		ServerURLs:           serverURLs,
		BaseDN:               types.StringValue(api.BaseDN),
		StartTLS:             types.BoolValue(api.StartTLS),
		ValidateCertificates: types.BoolValue(api.ValidateCertificates),
		Schema:               types.StringValue(api.Schema),
		AuxiliaryParameters:  stringPtrOrEmptyNullable(api.AuxiliaryParameters),
		SearchBases:          searchBases,
		AttributeMaps:        attrMaps,
	})
	diags.Append(d...)
	return obj, diags
}

// ipaConfigToModel builds a types.Object (ipaConfigAttrTypes) from a probed
// directoryServicesConfigAPI (only valid when the top-level ServiceType is
// "IPA" — see responseToModel).
func ipaConfigToModel(ctx context.Context, api *directoryServicesConfigAPI) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics

	smbDomain := types.ObjectNull(ipaSMBDomainAttrTypes)
	if api.SMBDomain != nil {
		var d diag.Diagnostics
		smbDomain, d = types.ObjectValueFrom(ctx, ipaSMBDomainAttrTypes, IPASMBDomainModel{
			Name:       stringPtrOrEmptyNullable(api.SMBDomain.Name),
			RangeLow:   types.Int64Value(api.SMBDomain.RangeLow),
			RangeHigh:  types.Int64Value(api.SMBDomain.RangeHigh),
			DomainName: stringPtrOrEmptyNullable(api.SMBDomain.DomainName),
			DomainSID:  stringPtrOrEmptyNullable(api.SMBDomain.DomainSID),
		})
		diags.Append(d...)
	}

	obj, d := types.ObjectValueFrom(ctx, ipaConfigAttrTypes, IPAConfigModel{
		TargetServer:         types.StringValue(api.TargetServer),
		Hostname:             types.StringValue(api.Hostname),
		Domain:               types.StringValue(api.Domain),
		BaseDN:               types.StringValue(api.BaseDN),
		SMBDomain:            smbDomain,
		ValidateCertificates: types.BoolValue(api.ValidateCertificates),
	})
	diags.Append(d...)
	return obj, diags
}

// configurationBlocks builds the three configuration_* types.Object values
// (ActiveDirectory/LDAP/IPA) from an API response, filling only the one
// matching api.ServiceType and leaving the other two null. Shared by
// responseToModel and responseToDataSourceModel so the per-service-type
// dispatch logic exists in exactly one place.
func configurationBlocks(ctx context.Context, api *directoryServicesAPI) (ad, ldap, ipa types.Object, diags diag.Diagnostics) {
	ad = types.ObjectNull(adConfigAttrTypes)
	ldap = types.ObjectNull(ldapConfigAttrTypes)
	ipa = types.ObjectNull(ipaConfigAttrTypes)

	if api.ServiceType == nil || api.Configuration == nil {
		return ad, ldap, ipa, diags
	}

	switch *api.ServiceType {
	case "ACTIVEDIRECTORY":
		idmapObj, d := adIdmapToModel(ctx, api.Configuration.Idmap)
		diags.Append(d...)
		adModel := ActiveDirectoryConfigModel{
			Hostname:             types.StringValue(api.Configuration.Hostname),
			Domain:               types.StringValue(api.Configuration.Domain),
			Site:                 stringPtrOrEmpty(api.Configuration.Site),
			ComputerAccountOU:    stringPtrOrEmpty(api.Configuration.ComputerAccountOU),
			UseDefaultDomain:     types.BoolValue(api.Configuration.UseDefaultDomain),
			EnableTrustedDomains: types.BoolValue(api.Configuration.EnableTrustedDomains),
			Idmap:                idmapObj,
		}
		obj, d := types.ObjectValueFrom(ctx, adConfigAttrTypes, adModel)
		diags.Append(d...)
		ad = obj
	case "LDAP":
		obj, d := ldapConfigToModel(ctx, api.Configuration)
		diags.Append(d...)
		ldap = obj
	case "IPA":
		obj, d := ipaConfigToModel(ctx, api.Configuration)
		diags.Append(d...)
		ipa = obj
	}

	return ad, ldap, ipa, diags
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

	ad, ldap, ipa, d := configurationBlocks(ctx, api)
	diags.Append(d...)
	m.ConfigurationActiveDirectory = ad
	m.ConfigurationLDAP = ldap
	m.ConfigurationIPA = ipa

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

	ad, ldap, ipa, d := configurationBlocks(ctx, api)
	diags.Append(d...)
	m.ConfigurationActiveDirectory = ad
	m.ConfigurationLDAP = ldap
	m.ConfigurationIPA = ipa

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

// credentialPayload builds the "credential" payload member matching
// cred.CredentialType. Callers must run validateCredential first (or
// otherwise guarantee the required fields for the type are present) —
// this function does not itself validate, it only shapes the payload from
// whatever is in cred.
func credentialPayload(cred CredentialModel) map[string]any {
	switch cred.CredentialType.ValueString() {
	case "KERBEROS_USER":
		return map[string]any{
			"credential_type": "KERBEROS_USER",
			"username":        cred.Username.ValueString(),
			"password":        cred.Password.ValueString(),
		}
	case "KERBEROS_PRINCIPAL":
		return map[string]any{
			"credential_type": "KERBEROS_PRINCIPAL",
			"principal":       cred.Principal.ValueString(),
		}
	case "LDAP_PLAIN":
		return map[string]any{
			"credential_type": "LDAP_PLAIN",
			"binddn":          cred.BindDN.ValueString(),
			"bindpw":          cred.BindPW.ValueString(),
		}
	case "LDAP_MTLS":
		return map[string]any{
			"credential_type":    "LDAP_MTLS",
			"client_certificate": cred.ClientCertificate.ValueString(),
		}
	case "LDAP_ANONYMOUS":
		return map[string]any{"credential_type": "LDAP_ANONYMOUS"}
	default:
		return map[string]any{"credential_type": cred.CredentialType.ValueString()}
	}
}

// validateCredential returns an error diagnostic if cred is missing a
// field required for its own CredentialType — LDAP_PLAIN needs binddn+
// bindpw; KERBEROS_USER needs username+password; KERBEROS_PRINCIPAL needs
// principal; LDAP_MTLS needs client_certificate; LDAP_ANONYMOUS needs
// nothing. Mirrors the clean-diagnostic-over-EINVAL style of updatePayload's
// other preflight checks.
func validateCredential(cred CredentialModel) diag.Diagnostics {
	var diags diag.Diagnostics
	missing := func(field, credType string) {
		diags.AddError(
			"Incomplete directory services credential",
			fmt.Sprintf("%q is required in \"credential\" when credential_type is %q.", field, credType),
		)
	}

	switch cred.CredentialType.ValueString() {
	case "KERBEROS_USER":
		if cred.Username.IsNull() || cred.Username.IsUnknown() || cred.Username.ValueString() == "" {
			missing("username", "KERBEROS_USER")
		}
		if cred.Password.IsNull() || cred.Password.IsUnknown() || cred.Password.ValueString() == "" {
			missing("password", "KERBEROS_USER")
		}
	case "KERBEROS_PRINCIPAL":
		if cred.Principal.IsNull() || cred.Principal.IsUnknown() || cred.Principal.ValueString() == "" {
			missing("principal", "KERBEROS_PRINCIPAL")
		}
	case "LDAP_PLAIN":
		if cred.BindDN.IsNull() || cred.BindDN.IsUnknown() || cred.BindDN.ValueString() == "" {
			missing("binddn", "LDAP_PLAIN")
		}
		if cred.BindPW.IsNull() || cred.BindPW.IsUnknown() || cred.BindPW.ValueString() == "" {
			missing("bindpw", "LDAP_PLAIN")
		}
	case "LDAP_MTLS":
		if cred.ClientCertificate.IsNull() || cred.ClientCertificate.IsUnknown() || cred.ClientCertificate.ValueString() == "" {
			missing("client_certificate", "LDAP_MTLS")
		}
	case "LDAP_ANONYMOUS":
		// No fields required.
	}
	return diags
}

// validateIdmapDomainBackendFields returns a preflight error diagnostic for
// each idmap_domain field the wire's discriminated union does not accept
// under dm's own idmap_backend (AD-only schema_mode/unix_primary_group/
// unix_nss_info set while the backend is anything other than "AD"; RID-only
// sssd_compat set while the backend is "AD"). This runs BEFORE any API call,
// so a bad combination is rejected at plan/apply-preflight time instead of
// being silently dropped by buildADConfigPayload's own backend gating,
// joining successfully, and only then surfacing as a
// "Provider produced inconsistent result after apply" error once TrueNAS
// read-back decodes the dropped field back to its Go zero value (false).
//
// Only a genuinely truthy (bool) or non-empty (string) value is treated as
// "the user actually wants this set": every one of these fields is
// Optional+Computed with UseStateForUnknown, and adIdmapToModel decodes them
// unconditionally (types.BoolValue/stringPtrOrEmptyNullable), so a value
// merely carried forward from a prior read-back under a DIFFERENT backend —
// e.g. unix_primary_group=false surviving in state from before a box was
// re-pointed at RID — is a known, non-null Bool that is indistinguishable at
// this layer from one the user just wrote in HCL. false/"" is exactly the
// value backend-appropriate omission already produces, so it is always safe
// to let through; only true/non-empty can only originate from the user
// actually configuring a backend-inapplicable field.
func validateIdmapDomainBackendFields(dm IdmapDomainModel) diag.Diagnostics {
	var diags diag.Diagnostics
	backend := dm.IdmapBackend.ValueString()

	invalid := func(field, applicableBackend string) {
		diags.AddError(
			"Invalid idmap_domain configuration",
			fmt.Sprintf("%q in \"idmap.idmap_domain\" is only applicable when idmap_backend is %q; "+
				"it cannot be set when idmap_backend is %q.", field, applicableBackend, backend),
		)
	}

	if backend != "AD" {
		if !dm.SchemaMode.IsNull() && !dm.SchemaMode.IsUnknown() && dm.SchemaMode.ValueString() != "" {
			invalid("schema_mode", "AD")
		}
		if !dm.UnixPrimaryGroup.IsNull() && !dm.UnixPrimaryGroup.IsUnknown() && dm.UnixPrimaryGroup.ValueBool() {
			invalid("unix_primary_group", "AD")
		}
		if !dm.UnixNSSInfo.IsNull() && !dm.UnixNSSInfo.IsUnknown() && dm.UnixNSSInfo.ValueBool() {
			invalid("unix_nss_info", "AD")
		}
	}
	if backend != "RID" {
		if !dm.SSSDCompat.IsNull() && !dm.SSSDCompat.IsUnknown() && dm.SSSDCompat.ValueBool() {
			invalid("sssd_compat", "RID")
		}
	}

	return diags
}

// buildADConfigPayload builds the "configuration" payload for service_type
// ACTIVEDIRECTORY. idmap is built from ad.Idmap when set (see idmapPayload);
// when ad.Idmap is null/unknown, "idmap" is omitted entirely from the
// payload (TrueNAS applies its own defaults on a fresh join, or — since
// idmap is Optional+Computed with UseStateForUnknown — the prior state's
// idmap object carries forward as a KNOWN plan value on any later update,
// so in practice this branch is only taken on a fresh join). trusted_domains
// continues to be round-tripped verbatim from existing (out of scope for
// this version — see directoryServicesConfigAPI's doc comment).
func buildADConfigPayload(ctx context.Context, ad ActiveDirectoryConfigModel, existing *directoryServicesAPI) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	p := map[string]any{
		"service_type": "ACTIVEDIRECTORY",
		"hostname":     ad.Hostname.ValueString(),
		"domain":       ad.Domain.ValueString(),
	}
	// site/computer_account_ou are ALWAYS present in the outgoing payload
	// (unlike the newer omit-when-unset fields below) — regression coverage
	// for the original v1 behavior: ad.Site/ComputerAccountOU.ValueString()
	// returns "" for null/unknown values just as much as for an explicit ""
	// clear, so both cases send JSON null; only a genuinely non-empty value
	// is sent as-is.
	if v := ad.Site.ValueString(); v != "" {
		p["site"] = v
	} else {
		p["site"] = nil
	}
	if v := ad.ComputerAccountOU.ValueString(); v != "" {
		p["computer_account_ou"] = v
	} else {
		p["computer_account_ou"] = nil
	}
	setOptionalBool(p, "use_default_domain", ad.UseDefaultDomain)
	setOptionalBool(p, "enable_trusted_domains", ad.EnableTrustedDomains)

	if !ad.Idmap.IsNull() && !ad.Idmap.IsUnknown() {
		var idm IdmapModel
		diags.Append(ad.Idmap.As(ctx, &idm, basetypes.ObjectAsOptions{})...)
		idmapPayload := map[string]any{}

		if !idm.Builtin.IsNull() && !idm.Builtin.IsUnknown() {
			var b IdmapBuiltinModel
			diags.Append(idm.Builtin.As(ctx, &b, basetypes.ObjectAsOptions{})...)
			builtinPayload := map[string]any{}
			setThreeWayString(builtinPayload, "name", b.Name)
			setOptionalInt64(builtinPayload, "range_low", b.RangeLow)
			setOptionalInt64(builtinPayload, "range_high", b.RangeHigh)
			idmapPayload["builtin"] = builtinPayload
		}

		if !idm.IdmapDomain.IsNull() && !idm.IdmapDomain.IsUnknown() {
			var dm IdmapDomainModel
			diags.Append(idm.IdmapDomain.As(ctx, &dm, basetypes.ObjectAsOptions{})...)

			if dm.IdmapBackend.ValueString() == "AD" &&
				(dm.SchemaMode.IsNull() || dm.SchemaMode.IsUnknown() || dm.SchemaMode.ValueString() == "") {
				diags.AddError(
					"Incomplete idmap_domain configuration",
					"\"schema_mode\" is required in \"idmap.idmap_domain\" when idmap_backend is \"AD\".",
				)
			}
			diags.Append(validateIdmapDomainBackendFields(dm)...)
			if diags.HasError() {
				return p, diags
			}

			domainPayload := map[string]any{
				"idmap_backend": dm.IdmapBackend.ValueString(),
			}
			setThreeWayString(domainPayload, "name", dm.Name)
			setOptionalInt64(domainPayload, "range_low", dm.RangeLow)
			setOptionalInt64(domainPayload, "range_high", dm.RangeHigh)
			// schema_mode/unix_primary_group/unix_nss_info and sssd_compat
			// are mutually exclusive, backend-specific fields on the wire's
			// discriminated union (AD vs. RID — see idmapDomainAttrTypes's
			// doc comment). Confirmed live (TrueNAS 25.10, this plan's Task 4
			// AD idmap acceptance run): sending unix_primary_group/
			// unix_nss_info alongside idmap_backend="RID" is rejected with
			// "[EINVAL] directoryservices_update.configuration.
			// ACTIVEDIRECTORY.idmap.idmap_domain.RID.unix_nss_info: Extra
			// inputs are not permitted" (and the same for
			// unix_primary_group) even though both fields are non-null,
			// known Bool values in the model — read back from a PRIOR RID
			// join where the API simply didn't include them (decoding to
			// the Go zero value, false, since directoryServicesIdmapDomainAPI
			// has no pointer/omitempty distinction for them). Gating on the
			// selected backend, not merely on null-ness, is required.
			//
			// The switch is exhaustive over the two backends this provider
			// actually models (AD, RID — enforced for user-supplied config by
			// the idmap_backend schema validator's OneOf("AD", "RID")) and
			// falls through to a no-op default for any other value. That
			// default is reachable despite the validator: idmap_domain is
			// Optional+Computed with UseStateForUnknown, so a value read back
			// from a box joined to LDAP/RFC2307 out-of-band (idmap_backend
			// values this provider doesn't otherwise expose — see
			// idmapDomainAttrTypes's doc comment) carries forward into the
			// plan verbatim without ever passing through the validator. This
			// package keeps idmap_domain as a typed struct rather than raw
			// JSON (unlike trusted_domains), so there is no verbatim wire
			// payload left to pass through for such a backend; sending our
			// own guess at AD- or RID-specific fields for it previously sent
			// sssd_compat unconditionally for "any non-AD backend" and was
			// rejected live by LDAP/RFC2307 boxes with "[EINVAL] ... Extra
			// inputs are not permitted". Omitting all backend-specific keys
			// and sending only the fields common to every variant (
			// idmap_backend/name/range_low/range_high, set unconditionally
			// above) is the safe choice: it changes nothing about that
			// backend's own idmap fields rather than fabricating values for
			// a shape this provider cannot verify.
			switch dm.IdmapBackend.ValueString() {
			case "AD":
				setOptionalString(domainPayload, "schema_mode", dm.SchemaMode)
				setOptionalBool(domainPayload, "unix_primary_group", dm.UnixPrimaryGroup)
				setOptionalBool(domainPayload, "unix_nss_info", dm.UnixNSSInfo)
			case "RID":
				setOptionalBool(domainPayload, "sssd_compat", dm.SSSDCompat)
			}
			idmapPayload["idmap_domain"] = domainPayload
		}

		p["idmap"] = idmapPayload
	}

	if existing != nil && existing.ServiceType != nil && *existing.ServiceType == "ACTIVEDIRECTORY" &&
		existing.Configuration != nil && len(existing.Configuration.TrustedDomains) > 0 {
		var trustedDomains any
		if err := json.Unmarshal(existing.Configuration.TrustedDomains, &trustedDomains); err == nil {
			p["trusted_domains"] = trustedDomains
		}
	}

	return p, diags
}

// buildLDAPConfigPayload builds the "configuration" payload for
// service_type LDAP. search_bases/attribute_maps are omitted entirely from
// the payload when their block is left null/unknown (server defaults
// apply); when set, only their individually-set leaf fields are included
// (each following the three-way clearable-string convention).
func buildLDAPConfigPayload(ctx context.Context, l LDAPConfigModel) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	var serverURLs []string
	diags.Append(l.ServerURLs.ElementsAs(ctx, &serverURLs, false)...)

	p := map[string]any{
		"service_type": "LDAP",
		"server_urls":  serverURLs,
		"basedn":       l.BaseDN.ValueString(),
	}
	setOptionalBool(p, "starttls", l.StartTLS)
	setOptionalBool(p, "validate_certificates", l.ValidateCertificates)
	setOptionalString(p, "schema", l.Schema)
	setThreeWayString(p, "auxiliary_parameters", l.AuxiliaryParameters)

	if !l.SearchBases.IsNull() && !l.SearchBases.IsUnknown() {
		var sb LDAPSearchBasesModel
		diags.Append(l.SearchBases.As(ctx, &sb, basetypes.ObjectAsOptions{})...)
		sbPayload := map[string]any{}
		setThreeWayString(sbPayload, "base_user", sb.BaseUser)
		setThreeWayString(sbPayload, "base_group", sb.BaseGroup)
		setThreeWayString(sbPayload, "base_netgroup", sb.BaseNetgroup)
		p["search_bases"] = sbPayload
	}

	if !l.AttributeMaps.IsNull() && !l.AttributeMaps.IsUnknown() {
		var am LDAPAttributeMapsModel
		diags.Append(l.AttributeMaps.As(ctx, &am, basetypes.ObjectAsOptions{})...)
		amPayload := map[string]any{}

		if !am.Passwd.IsNull() && !am.Passwd.IsUnknown() {
			var passwd LDAPAttrMapPasswdModel
			diags.Append(am.Passwd.As(ctx, &passwd, basetypes.ObjectAsOptions{})...)
			passwdPayload := map[string]any{}
			setThreeWayString(passwdPayload, "user_object_class", passwd.UserObjectClass)
			setThreeWayString(passwdPayload, "user_name", passwd.UserName)
			setThreeWayString(passwdPayload, "user_uid", passwd.UserUID)
			setThreeWayString(passwdPayload, "user_gid", passwd.UserGID)
			setThreeWayString(passwdPayload, "user_gecos", passwd.UserGecos)
			setThreeWayString(passwdPayload, "user_home_directory", passwd.UserHomeDirectory)
			setThreeWayString(passwdPayload, "user_shell", passwd.UserShell)
			amPayload["passwd"] = passwdPayload
		}

		if !am.Shadow.IsNull() && !am.Shadow.IsUnknown() {
			var shadow LDAPAttrMapShadowModel
			diags.Append(am.Shadow.As(ctx, &shadow, basetypes.ObjectAsOptions{})...)
			shadowPayload := map[string]any{}
			setThreeWayString(shadowPayload, "shadow_last_change", shadow.ShadowLastChange)
			setThreeWayString(shadowPayload, "shadow_min", shadow.ShadowMin)
			setThreeWayString(shadowPayload, "shadow_max", shadow.ShadowMax)
			setThreeWayString(shadowPayload, "shadow_warning", shadow.ShadowWarning)
			setThreeWayString(shadowPayload, "shadow_inactive", shadow.ShadowInactive)
			setThreeWayString(shadowPayload, "shadow_expire", shadow.ShadowExpire)
			amPayload["shadow"] = shadowPayload
		}

		if !am.Group.IsNull() && !am.Group.IsUnknown() {
			var group LDAPAttrMapGroupModel
			diags.Append(am.Group.As(ctx, &group, basetypes.ObjectAsOptions{})...)
			groupPayload := map[string]any{}
			setThreeWayString(groupPayload, "group_object_class", group.GroupObjectClass)
			setThreeWayString(groupPayload, "group_gid", group.GroupGID)
			setThreeWayString(groupPayload, "group_member", group.GroupMember)
			amPayload["group"] = groupPayload
		}

		if !am.Netgroup.IsNull() && !am.Netgroup.IsUnknown() {
			var netgroup LDAPAttrMapNetgroupModel
			diags.Append(am.Netgroup.As(ctx, &netgroup, basetypes.ObjectAsOptions{})...)
			netgroupPayload := map[string]any{}
			setThreeWayString(netgroupPayload, "netgroup_object_class", netgroup.NetgroupObjectClass)
			setThreeWayString(netgroupPayload, "netgroup_member", netgroup.NetgroupMember)
			setThreeWayString(netgroupPayload, "netgroup_triple", netgroup.NetgroupTriple)
			amPayload["netgroup"] = netgroupPayload
		}

		p["attribute_maps"] = amPayload
	}

	return p, diags
}

// buildIPAConfigPayload builds the "configuration" payload for
// service_type IPA. smb_domain is omitted entirely from the payload when
// left null/unknown (normally the case — TrueNAS detects it during the
// join); idmap_backend within it is always the constant "SSS" and is
// injected here rather than exposed as a user-settable field.
func buildIPAConfigPayload(ctx context.Context, ipa IPAConfigModel) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	p := map[string]any{
		"service_type":  "IPA",
		"target_server": ipa.TargetServer.ValueString(),
		"hostname":      ipa.Hostname.ValueString(),
		"domain":        ipa.Domain.ValueString(),
		"basedn":        ipa.BaseDN.ValueString(),
	}
	setOptionalBool(p, "validate_certificates", ipa.ValidateCertificates)

	if !ipa.SMBDomain.IsNull() && !ipa.SMBDomain.IsUnknown() {
		var smb IPASMBDomainModel
		diags.Append(ipa.SMBDomain.As(ctx, &smb, basetypes.ObjectAsOptions{})...)
		smbPayload := map[string]any{"idmap_backend": "SSS"}
		setThreeWayString(smbPayload, "name", smb.Name)
		setOptionalInt64(smbPayload, "range_low", smb.RangeLow)
		setOptionalInt64(smbPayload, "range_high", smb.RangeHigh)
		setThreeWayString(smbPayload, "domain_name", smb.DomainName)
		setThreeWayString(smbPayload, "domain_sid", smb.DomainSID)
		p["smb_domain"] = smbPayload
	}

	return p, diags
}

// updatePayload builds the directoryservices.update argument from a plan
// that has already had its write-only "credential" fields (password/bindpw)
// sourced from req.Config (see resource.go Create/Update). Its shape is
// dictated entirely by m.Enable, per live testing against a TrueNAS 25.10 box
// that repeatedly contradicted the JSON schema's own "_required_": false on
// every top-level field:
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
//     changing — including "trusted_domains" (AD only), which this
//     provider's schema does not expose — or the job fails with
//     "[EINVAL] directoryservices.update.configuration: Permitted changes
//     while directory services are enabled are limited to account
//     caching, DNS updates, and timeouts." AD's "idmap" no longer needs
//     the same verbatim-JSON round-trip treatment now that it is
//     explicitly modeled (Optional+Computed, UseStateForUnknown): once
//     read back after a join, its value carries forward as a KNOWN plan
//     value on every subsequent update even when the HCL omits the block,
//     and buildADConfigPayload reconstructs the exact same object TrueNAS
//     persisted from that known value. "credential" has the same
//     "must match" behavior but with a twist: TrueNAS swaps a raw
//     KERBEROS_USER (or, for IPA, the equivalent Kerberos-authenticated)
//     admin credential used for the initial join into a KERBEROS_PRINCIPAL
//     backed by the new machine account's own keytab, and then REJECTS
//     resending the raw credential on a later call — confirmed live, the
//     disabling call specifically fails with "[EINVAL]
//     directoryservices.update.credential.credential_type: Kerberos user
//     credentials may not be stored for disabled directory services..."
//     if the last update re-stored a raw KERBEROS_USER. So once joined,
//     every subsequent AD/IPA call (including eventually disabling) must
//     resend the swapped KERBEROS_PRINCIPAL, not m.Credential — see
//     existingKerberosPrincipal. LDAP joins have no such swap: their
//     credential (LDAP_PLAIN/LDAP_ANONYMOUS/LDAP_MTLS) is resent as
//     supplied on every update.
//
// existing is the currently-persisted directoryServicesAPI (nil, or with a
// nil Configuration, on a fresh join where none exists yet) — the FULL
// top-level object, not just its nested Configuration: the nested
// "configuration" object returned by directoryservices.config/update does
// NOT echo back its own "service_type" discriminator key (confirmed live —
// directoryServicesConfigAPI.ServiceType decodes to "" from a real
// response), so the top-level ServiceType is the only reliable signal for
// "was this an Active Directory configuration" (used by
// buildADConfigPayload's trusted_domains round-trip). kerberos_realm uses
// the same three-way guard as network_config's nameserver1-3: omitted when
// null/unknown, sent as JSON nil when the model holds an explicit ""
// (clearing the realm), sent as its string value otherwise.
//
// Preflight (in order, all diagnostics collected before returning): service
// type must be set; exactly one configuration_* block must be
// non-null/non-unknown, and it must be the one matching service_type;
// credential must be present (or reusable from an existing
// KERBEROS_PRINCIPAL) and internally consistent for its own
// credential_type (validateCredential).
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

	adSet := !m.ConfigurationActiveDirectory.IsNull() && !m.ConfigurationActiveDirectory.IsUnknown()
	ldapSet := !m.ConfigurationLDAP.IsNull() && !m.ConfigurationLDAP.IsUnknown()
	ipaSet := !m.ConfigurationIPA.IsNull() && !m.ConfigurationIPA.IsUnknown()
	configCount := 0
	for _, set := range []bool{adSet, ldapSet, ipaSet} {
		if set {
			configCount++
		}
	}
	if configCount != 1 {
		diags.AddError(
			"Invalid directory services configuration",
			fmt.Sprintf("Exactly one of \"configuration_activedirectory\", \"configuration_ldap\", or "+
				"\"configuration_ipa\" must be set whenever \"enable\" is true; found %d set.", configCount),
		)
	} else if !m.ServiceType.IsNull() && !m.ServiceType.IsUnknown() {
		serviceType := m.ServiceType.ValueString()
		mismatched := (serviceType == "ACTIVEDIRECTORY" && !adSet) ||
			(serviceType == "LDAP" && !ldapSet) ||
			(serviceType == "IPA" && !ipaSet)
		if mismatched {
			diags.AddError(
				"Invalid directory services configuration",
				fmt.Sprintf("\"service_type\" is %q, but the matching configuration_* block is not set.", serviceType),
			)
		}
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
		// Already joined (AD or IPA): resend the machine account's own
		// principal (no secret material) rather than the original admin
		// credential.
		p["credential"] = map[string]any{
			"credential_type": "KERBEROS_PRINCIPAL",
			"principal":       *existingPrincipal,
		}
	} else {
		var cred CredentialModel
		diags.Append(m.Credential.As(ctx, &cred, basetypes.ObjectAsOptions{})...)
		diags.Append(validateCredential(cred)...)
		if diags.HasError() {
			return p, diags
		}
		p["credential"] = credentialPayload(cred)
	}

	switch {
	case adSet:
		var ad ActiveDirectoryConfigModel
		diags.Append(m.ConfigurationActiveDirectory.As(ctx, &ad, basetypes.ObjectAsOptions{})...)
		configPayload, d := buildADConfigPayload(ctx, ad, existing)
		diags.Append(d...)
		p["configuration"] = configPayload
	case ldapSet:
		var l LDAPConfigModel
		diags.Append(m.ConfigurationLDAP.As(ctx, &l, basetypes.ObjectAsOptions{})...)
		configPayload, d := buildLDAPConfigPayload(ctx, l)
		diags.Append(d...)
		p["configuration"] = configPayload
	case ipaSet:
		var ipa IPAConfigModel
		diags.Append(m.ConfigurationIPA.As(ctx, &ipa, basetypes.ObjectAsOptions{})...)
		configPayload, d := buildIPAConfigPayload(ctx, ipa)
		diags.Append(d...)
		p["configuration"] = configPayload
	}

	return p, diags
}

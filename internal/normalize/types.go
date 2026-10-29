// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package normalize

// This file declares the exported string types. Each one is an alias of the
// shared stringType/stringValue instantiated with a kind, so they all behave
// the same way and differ only in how two spellings are compared.

// IPAddressType is the attribute type for an IP address; IPv6 compressed, exploded and upper-case spellings are equal (B-076 gateway).
// Use it as CustomType on a schema.StringAttribute; the model field is IPAddress.
type IPAddressType = stringType[ipKind]

// IPAddress is the value of a IPAddressType attribute. It embeds basetypes.StringValue
// (ValueString, IsNull, IsUnknown, ...).
type IPAddress = stringValue[ipKind]

// NewIPAddressValue returns a known IPAddress.
func NewIPAddressValue(s string) IPAddress { return newValue[ipKind](s) }

// NewIPAddressNull returns a null IPAddress.
func NewIPAddressNull() IPAddress { return newNull[ipKind]() }

// NewIPAddressUnknown returns an unknown IPAddress.
func NewIPAddressUnknown() IPAddress { return newUnknown[ipKind]() }

// NewIPAddressPointerValue returns a IPAddress that is null when s is nil.
func NewIPAddressPointerValue(s *string) IPAddress { return newPointer[ipKind](s) }

// CIDRType is the attribute type for an address or network; a bare IP equals its /32 or /128 form, a dotted IPv4 netmask equals its prefix length, IPv6 spelling is ignored, host bits matter (B-075, B-076, B-079, B-124).
// Use it as CustomType on a schema.StringAttribute; the model field is CIDR.
type CIDRType = stringType[cidrKind]

// CIDR is the value of a CIDRType attribute. It embeds basetypes.StringValue
// (ValueString, IsNull, IsUnknown, ...).
type CIDR = stringValue[cidrKind]

// NewCIDRValue returns a known CIDR.
func NewCIDRValue(s string) CIDR { return newValue[cidrKind](s) }

// NewCIDRNull returns a null CIDR.
func NewCIDRNull() CIDR { return newNull[cidrKind]() }

// NewCIDRUnknown returns an unknown CIDR.
func NewCIDRUnknown() CIDR { return newUnknown[cidrKind]() }

// NewCIDRPointerValue returns a CIDR that is null when s is nil.
func NewCIDRPointerValue(s *string) CIDR { return newPointer[cidrKind](s) }

// NetworkType is the attribute type for an address or network like CIDR, but host bits are ignored: 172.17.0.0/12 equals 172.16.0.0/12 (NFS networks #13, docker address_pools B-078).
// Use it as CustomType on a schema.StringAttribute; the model field is Network.
type NetworkType = stringType[networkKind]

// Network is the value of a NetworkType attribute. It embeds basetypes.StringValue
// (ValueString, IsNull, IsUnknown, ...).
type Network = stringValue[networkKind]

// NewNetworkValue returns a known Network.
func NewNetworkValue(s string) Network { return newValue[networkKind](s) }

// NewNetworkNull returns a null Network.
func NewNetworkNull() Network { return newNull[networkKind]() }

// NewNetworkUnknown returns an unknown Network.
func NewNetworkUnknown() Network { return newUnknown[networkKind]() }

// NewNetworkPointerValue returns a Network that is null when s is nil.
func NewNetworkPointerValue(s *string) Network { return newPointer[networkKind](s) }

// EmailType is the attribute type for an e-mail address whose domain part compares case-insensitively (B-077, B-127).
// Use it as CustomType on a schema.StringAttribute; the model field is Email.
type EmailType = stringType[emailKind]

// Email is the value of a EmailType attribute. It embeds basetypes.StringValue
// (ValueString, IsNull, IsUnknown, ...).
type Email = stringValue[emailKind]

// NewEmailValue returns a known Email.
func NewEmailValue(s string) Email { return newValue[emailKind](s) }

// NewEmailNull returns a null Email.
func NewEmailNull() Email { return newNull[emailKind]() }

// NewEmailUnknown returns an unknown Email.
func NewEmailUnknown() Email { return newUnknown[emailKind]() }

// NewEmailPointerValue returns a Email that is null when s is nil.
func NewEmailPointerValue(s *string) Email { return newPointer[emailKind](s) }

// CaseInsensitiveType is the attribute type for a string compared case-insensitively: enums, SIDs, MAC addresses, ZFS sizes (B-002, B-074, B-082, B-083, B-126).
// Use it as CustomType on a schema.StringAttribute; the model field is CaseInsensitive.
type CaseInsensitiveType = stringType[caseInsensitiveKind]

// CaseInsensitive is the value of a CaseInsensitiveType attribute. It embeds basetypes.StringValue
// (ValueString, IsNull, IsUnknown, ...).
type CaseInsensitive = stringValue[caseInsensitiveKind]

// NewCaseInsensitiveValue returns a known CaseInsensitive.
func NewCaseInsensitiveValue(s string) CaseInsensitive { return newValue[caseInsensitiveKind](s) }

// NewCaseInsensitiveNull returns a null CaseInsensitive.
func NewCaseInsensitiveNull() CaseInsensitive { return newNull[caseInsensitiveKind]() }

// NewCaseInsensitiveUnknown returns an unknown CaseInsensitive.
func NewCaseInsensitiveUnknown() CaseInsensitive { return newUnknown[caseInsensitiveKind]() }

// NewCaseInsensitivePointerValue returns a CaseInsensitive that is null when s is nil.
func NewCaseInsensitivePointerValue(s *string) CaseInsensitive {
	return newPointer[caseInsensitiveKind](s)
}

// TimeOfDayType is the attribute type for an "HH:MM" time; "8:00" equals "08:00" (B-088).
// Use it as CustomType on a schema.StringAttribute; the model field is TimeOfDay.
type TimeOfDayType = stringType[timeOfDayKind]

// TimeOfDay is the value of a TimeOfDayType attribute. It embeds basetypes.StringValue
// (ValueString, IsNull, IsUnknown, ...).
type TimeOfDay = stringValue[timeOfDayKind]

// NewTimeOfDayValue returns a known TimeOfDay.
func NewTimeOfDayValue(s string) TimeOfDay { return newValue[timeOfDayKind](s) }

// NewTimeOfDayNull returns a null TimeOfDay.
func NewTimeOfDayNull() TimeOfDay { return newNull[timeOfDayKind]() }

// NewTimeOfDayUnknown returns an unknown TimeOfDay.
func NewTimeOfDayUnknown() TimeOfDay { return newUnknown[timeOfDayKind]() }

// NewTimeOfDayPointerValue returns a TimeOfDay that is null when s is nil.
func NewTimeOfDayPointerValue(s *string) TimeOfDay { return newPointer[timeOfDayKind](s) }

// RFC3339Type is the attribute type for an RFC3339 timestamp compared as an instant to the second, whatever the offset (B-059).
// Use it as CustomType on a schema.StringAttribute; the model field is RFC3339.
type RFC3339Type = stringType[rfc3339Kind]

// RFC3339 is the value of a RFC3339Type attribute. It embeds basetypes.StringValue
// (ValueString, IsNull, IsUnknown, ...).
type RFC3339 = stringValue[rfc3339Kind]

// NewRFC3339Value returns a known RFC3339.
func NewRFC3339Value(s string) RFC3339 { return newValue[rfc3339Kind](s) }

// NewRFC3339Null returns a null RFC3339.
func NewRFC3339Null() RFC3339 { return newNull[rfc3339Kind]() }

// NewRFC3339Unknown returns an unknown RFC3339.
func NewRFC3339Unknown() RFC3339 { return newUnknown[rfc3339Kind]() }

// NewRFC3339PointerValue returns a RFC3339 that is null when s is nil.
func NewRFC3339PointerValue(s *string) RFC3339 { return newPointer[rfc3339Kind](s) }

// URLType is the attribute type for a URL compared after HttpUrl normalization: trailing slash, scheme/host case, default port (B-080, B-085, B-122).
// Use it as CustomType on a schema.StringAttribute; the model field is URL.
type URLType = stringType[urlKind]

// URL is the value of a URLType attribute. It embeds basetypes.StringValue
// (ValueString, IsNull, IsUnknown, ...).
type URL = stringValue[urlKind]

// NewURLValue returns a known URL.
func NewURLValue(s string) URL { return newValue[urlKind](s) }

// NewURLNull returns a null URL.
func NewURLNull() URL { return newNull[urlKind]() }

// NewURLUnknown returns an unknown URL.
func NewURLUnknown() URL { return newUnknown[urlKind]() }

// NewURLPointerValue returns a URL that is null when s is nil.
func NewURLPointerValue(s *string) URL { return newPointer[urlKind](s) }

// TrimmedType is the attribute type for a text blob compared without leading/trailing whitespace or CRLF differences: SSH public keys, PEM (B-053, B-054).
// Use it as CustomType on a schema.StringAttribute; the model field is Trimmed.
type TrimmedType = stringType[trimmedKind]

// Trimmed is the value of a TrimmedType attribute. It embeds basetypes.StringValue
// (ValueString, IsNull, IsUnknown, ...).
type Trimmed = stringValue[trimmedKind]

// NewTrimmedValue returns a known Trimmed.
func NewTrimmedValue(s string) Trimmed { return newValue[trimmedKind](s) }

// NewTrimmedNull returns a null Trimmed.
func NewTrimmedNull() Trimmed { return newNull[trimmedKind]() }

// NewTrimmedUnknown returns an unknown Trimmed.
func NewTrimmedUnknown() Trimmed { return newUnknown[trimmedKind]() }

// NewTrimmedPointerValue returns a Trimmed that is null when s is nil.
func NewTrimmedPointerValue(s *string) Trimmed { return newPointer[trimmedKind](s) }

// PrivateKeyType is the attribute type for a PEM private key compared by key material, so PKCS#1/SEC1 equals the server's PKCS#8 (B-013).
// Use it as CustomType on a schema.StringAttribute; the model field is PrivateKey.
type PrivateKeyType = stringType[privateKeyKind]

// PrivateKey is the value of a PrivateKeyType attribute. It embeds basetypes.StringValue
// (ValueString, IsNull, IsUnknown, ...).
type PrivateKey = stringValue[privateKeyKind]

// NewPrivateKeyValue returns a known PrivateKey.
func NewPrivateKeyValue(s string) PrivateKey { return newValue[privateKeyKind](s) }

// NewPrivateKeyNull returns a null PrivateKey.
func NewPrivateKeyNull() PrivateKey { return newNull[privateKeyKind]() }

// NewPrivateKeyUnknown returns an unknown PrivateKey.
func NewPrivateKeyUnknown() PrivateKey { return newUnknown[privateKeyKind]() }

// NewPrivateKeyPointerValue returns a PrivateKey that is null when s is nil.
func NewPrivateKeyPointerValue(s *string) PrivateKey { return newPointer[privateKeyKind](s) }

// SANType is the attribute type for a certificate subjectAltName entry; "10.0.0.1" and "IP:10.0.0.1" equal "IP Address:10.0.0.1" (B-060).
// Use it as CustomType on a schema.StringAttribute; the model field is SAN.
type SANType = stringType[sanKind]

// SAN is the value of a SANType attribute. It embeds basetypes.StringValue
// (ValueString, IsNull, IsUnknown, ...).
type SAN = stringValue[sanKind]

// NewSANValue returns a known SAN.
func NewSANValue(s string) SAN { return newValue[sanKind](s) }

// NewSANNull returns a null SAN.
func NewSANNull() SAN { return newNull[sanKind]() }

// NewSANUnknown returns an unknown SAN.
func NewSANUnknown() SAN { return newUnknown[sanKind]() }

// NewSANPointerValue returns a SAN that is null when s is nil.
func NewSANPointerValue(s *string) SAN { return newPointer[sanKind](s) }

// OctalModeType is the attribute type for an octal file mode; "755" equals "0755" (B-091).
// Use it as CustomType on a schema.StringAttribute; the model field is OctalMode.
type OctalModeType = stringType[octalModeKind]

// OctalMode is the value of a OctalModeType attribute. It embeds basetypes.StringValue
// (ValueString, IsNull, IsUnknown, ...).
type OctalMode = stringValue[octalModeKind]

// NewOctalModeValue returns a known OctalMode.
func NewOctalModeValue(s string) OctalMode { return newValue[octalModeKind](s) }

// NewOctalModeNull returns a null OctalMode.
func NewOctalModeNull() OctalMode { return newNull[octalModeKind]() }

// NewOctalModeUnknown returns an unknown OctalMode.
func NewOctalModeUnknown() OctalMode { return newUnknown[octalModeKind]() }

// NewOctalModePointerValue returns a OctalMode that is null when s is nil.
func NewOctalModePointerValue(s *string) OctalMode { return newPointer[octalModeKind](s) }

// JSONType is the attribute type for a JSON document compared by value: key order, whitespace and number spelling ignored (B-061, B-095).
// Use it as CustomType on a schema.StringAttribute; the model field is JSON.
type JSONType = stringType[jsonKind]

// JSON is the value of a JSONType attribute. It embeds basetypes.StringValue
// (ValueString, IsNull, IsUnknown, ...).
type JSON = stringValue[jsonKind]

// NewJSONValue returns a known JSON.
func NewJSONValue(s string) JSON { return newValue[jsonKind](s) }

// NewJSONNull returns a null JSON.
func NewJSONNull() JSON { return newNull[jsonKind]() }

// NewJSONUnknown returns an unknown JSON.
func NewJSONUnknown() JSON { return newUnknown[jsonKind]() }

// NewJSONPointerValue returns a JSON that is null when s is nil.
func NewJSONPointerValue(s *string) JSON { return newPointer[jsonKind](s) }

// JSONSubsetType is the attribute type for a JSON document that equals the server document when every configured value is present and equal in it; keys the server adds, also inside list items, are ignored (B-092, B-094, B-120).
// Use it as CustomType on a schema.StringAttribute; the model field is JSONSubset.
type JSONSubsetType = stringType[jsonSubsetKind]

// JSONSubset is the value of a JSONSubsetType attribute. It embeds basetypes.StringValue
// (ValueString, IsNull, IsUnknown, ...).
type JSONSubset = stringValue[jsonSubsetKind]

// NewJSONSubsetValue returns a known JSONSubset.
func NewJSONSubsetValue(s string) JSONSubset { return newValue[jsonSubsetKind](s) }

// NewJSONSubsetNull returns a null JSONSubset.
func NewJSONSubsetNull() JSONSubset { return newNull[jsonSubsetKind]() }

// NewJSONSubsetUnknown returns an unknown JSONSubset.
func NewJSONSubsetUnknown() JSONSubset { return newUnknown[jsonSubsetKind]() }

// NewJSONSubsetPointerValue returns a JSONSubset that is null when s is nil.
func NewJSONSubsetPointerValue(s *string) JSONSubset { return newPointer[jsonSubsetKind](s) }

// YAMLType is the attribute type for a YAML document compared by value under YAML 1.1 (PyYAML) rules: yes/off are booleans, 0755 is octal, 1:30 is base 60 (B-097).
// Use it as CustomType on a schema.StringAttribute; the model field is YAML.
type YAMLType = stringType[yamlKind]

// YAML is the value of a YAMLType attribute. It embeds basetypes.StringValue
// (ValueString, IsNull, IsUnknown, ...).
type YAML = stringValue[yamlKind]

// NewYAMLValue returns a known YAML.
func NewYAMLValue(s string) YAML { return newValue[yamlKind](s) }

// NewYAMLNull returns a null YAML.
func NewYAMLNull() YAML { return newNull[yamlKind]() }

// NewYAMLUnknown returns an unknown YAML.
func NewYAMLUnknown() YAML { return newUnknown[yamlKind]() }

// NewYAMLPointerValue returns a YAML that is null when s is nil.
func NewYAMLPointerValue(s *string) YAML { return newPointer[yamlKind](s) }

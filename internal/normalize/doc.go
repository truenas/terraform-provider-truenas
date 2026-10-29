// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

// Package normalize holds custom attribute types for values that TrueNAS
// stores in a different spelling than the one configured. Comparing them
// verbatim gives "Provider produced inconsistent result after apply" or a
// diff on every plan (BUGS C2/C3). Each type implements the framework's
// semantic equality: when the value a resource reads back is equivalent to
// the prior (configured) value, the framework keeps the configured spelling
// in state. A resource therefore sets the server string as-is in Read,
// Create and Update; it does not need to compare or preserve anything.
//
// String types (schema.StringAttribute CustomType / model field):
//
//	IPAddressType       / IPAddress       IPv6 spelling (B-076 gateway)
//	CIDRType            / CIDR            bare IP == /32, netmask == prefix, IPv6 spelling (B-075, B-076, B-079, B-124)
//	NetworkType         / Network         CIDR with host bits ignored (#13 NFS networks, B-078 docker address_pools)
//	EmailType           / Email           e-mail domain case (B-077, B-127)
//	CaseInsensitiveType / CaseInsensitive enums, SIDs, MACs, sizes (B-002, B-074, B-082, B-083, B-126)
//	TimeOfDayType       / TimeOfDay       "8:00" == "08:00" (B-088)
//	RFC3339Type         / RFC3339         same instant, any offset (B-059)
//	URLType             / URL             trailing slash, scheme/host case, default port (B-080, B-085, B-122)
//	TrimmedType         / Trimmed         surrounding whitespace, CRLF (B-053, B-054)
//	PrivateKeyType      / PrivateKey      PKCS#1/SEC1 == PKCS#8 (B-013)
//	SANType             / SAN             "10.0.0.1" == "IP Address:10.0.0.1" (B-060)
//	OctalModeType       / OctalMode       "755" == "0755" (B-091)
//	JSONType            / JSON            JSON by value (B-061, B-095, B-005 idmap, B-118)
//	JSONSubsetType      / JSONSubset      JSON ignoring server-added keys (B-092, B-094, B-119, B-120)
//	YAMLType            / YAML            YAML 1.1 / PyYAML scalars (B-097)
//
// List type: UnorderedListOf(elemType) / UnorderedList compares lists in any
// order, with element semantic equality (group users, replication
// periodic_snapshot_tasks, portal listen IPs).
//
// The Equal*/Parse*/Decode* functions expose the same comparisons for code
// that cannot use a type, such as a JSON blob projection or a data source
// filter.
//
// Semantic equality only runs where the framework compares a new value with a
// known prior value: after Create, Update and Read. It does not run at plan
// time, so when state holds the server spelling (after `terraform import`, or
// after a config edit that only changes the spelling) the plan would show an
// update, or a replacement for a RequiresReplace attribute. Add the plan
// modifier KeepStateIfEquivalent(<type>) (KeepStateIfEquivalentList for
// lists), placed before any RequiresReplace modifier, to plan no change in
// that case.
package normalize

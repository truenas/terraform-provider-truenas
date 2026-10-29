// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package directoryservices

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestNeedsDisableForChange: an enabled service needs the disable/re-enable
// toggle for a kerberos_realm or configuration_* change, but not for the
// top-level fields TrueNAS lets change in place, nor when it is (or is
// becoming) disabled. (B-049)
func TestNeedsDisableForChange(t *testing.T) {
	enabled := &directoryServicesAPI{Enable: true}
	prior := baseModel()
	prior.Enable = types.BoolValue(true)
	prior.KerberosRealm = types.StringValue("")
	prior.Timeout = types.Int64Value(10)

	same := *prior
	same.Timeout = types.Int64Value(20)
	if needsDisableForChange(&same, prior, enabled) {
		t.Error("a timeout-only change must not toggle")
	}

	realm := *prior
	realm.KerberosRealm = types.StringValue("EXAMPLE.COM")
	if !needsDisableForChange(&realm, prior, enabled) {
		t.Error("a kerberos_realm change must toggle")
	}
	if needsDisableForChange(&realm, prior, &directoryServicesAPI{Enable: false}) {
		t.Error("a disabled service must not toggle")
	}
	disabling := realm
	disabling.Enable = types.BoolValue(false)
	if needsDisableForChange(&disabling, prior, enabled) {
		t.Error("a plan that disables must not toggle")
	}

	unknown := *prior
	unknown.ConfigurationLDAP = types.ObjectUnknown(ldapConfigAttrTypes)
	if !needsDisableForChange(&unknown, prior, enabled) {
		t.Error("an unknown planned configuration must toggle")
	}
}

// TestImportedLDAPCredential: import reads back the non-secret part of an LDAP
// credential, and leaves AD/IPA (swapped KERBEROS_PRINCIPAL) and an empty
// configuration null. (B-050)
func TestImportedLDAPCredential(t *testing.T) {
	got, d := importedLDAPCredential(json.RawMessage(`{"credential":{"credential_type":"LDAP_PLAIN","binddn":"cn=admin,dc=x","bindpw":"********"}}`))
	if d.HasError() {
		t.Fatal(d)
	}
	attrs := got.Attributes()
	if attrs["credential_type"].(types.String).ValueString() != "LDAP_PLAIN" ||
		attrs["binddn"].(types.String).ValueString() != "cn=admin,dc=x" ||
		!attrs["bindpw"].IsNull() {
		t.Errorf("LDAP_PLAIN import = %v", got)
	}

	got, _ = importedLDAPCredential(json.RawMessage(`{"credential":{"credential_type":"LDAP_MTLS","client_certificate":"c1"}}`))
	if got.Attributes()["client_certificate"].(types.String).ValueString() != "c1" || !got.Attributes()["binddn"].IsNull() {
		t.Errorf("LDAP_MTLS import = %v", got)
	}

	for _, raw := range []string{
		`{"credential":{"credential_type":"KERBEROS_PRINCIPAL","principal":"HOST$@X"}}`,
		`{"credential":null}`,
	} {
		got, d := importedLDAPCredential(json.RawMessage(raw))
		if d.HasError() || !got.IsNull() {
			t.Errorf("%s: got %v (%v), want null", raw, got, d)
		}
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ipmi_lan

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// --- isDHCP ------------------------------------------------------------

func TestIsDHCP(t *testing.T) {
	cases := map[string]bool{
		"static": false,
		"Static": false,
		"dhcp":   true,
		"DHCP":   true,
		"":       false,
	}
	for in, want := range cases {
		if got := isDHCP(in); got != want {
			t.Errorf("isDHCP(%q) = %v, want %v", in, got, want)
		}
	}
}

// --- parseChannel --------------------------------------------------------

func TestParseChannel(t *testing.T) {
	got, err := parseChannel("1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 1 {
		t.Errorf("parseChannel(\"1\") = %d, want 1", got)
	}
	if _, err := parseChannel("not-a-number"); err == nil {
		t.Error("expected an error for a non-numeric import ID")
	}
}

// --- responseToModel -------------------------------------------------------

// apiShape mirrors the live TrueNAS 25.10.4 Enterprise HA probe of channel 1
// (static IP, no VLAN): see model.go's ipmiLanAPI doc comment.
func apiShape() *ipmiLanAPI {
	return &ipmiLanAPI{
		Channel:                 1,
		ID:                      1,
		IPAddressSource:         "static",
		IPAddress:               "10.220.2.97",
		MACAddress:              "3c:ec:ef:da:e4:1d",
		SubnetMask:              "255.255.240.0",
		DefaultGatewayIPAddress: "10.220.0.1",
		VlanID:                  nil,
		VlanIDEnable:            false,
		VlanPriority:            0,
	}
}

func TestResponseToModel(t *testing.T) {
	m := &IPMILanModel{}
	responseToModel(apiShape(), m)

	if m.ID.ValueInt64() != 1 {
		t.Errorf("ID = %d, want 1", m.ID.ValueInt64())
	}
	if m.Channel.ValueInt64() != 1 {
		t.Errorf("Channel = %d, want 1", m.Channel.ValueInt64())
	}
	if m.DHCP.ValueBool() {
		t.Error("DHCP should be false for ip_address_source = \"static\"")
	}
	if m.IPAddress.ValueString() != "10.220.2.97" {
		t.Errorf("IPAddress = %q, want 10.220.2.97", m.IPAddress.ValueString())
	}
	if m.Netmask.ValueString() != "255.255.240.0" {
		t.Errorf("Netmask = %q, want 255.255.240.0", m.Netmask.ValueString())
	}
	if m.Gateway.ValueString() != "10.220.0.1" {
		t.Errorf("Gateway = %q, want 10.220.0.1", m.Gateway.ValueString())
	}
	if !m.Vlan.IsNull() {
		t.Errorf("Vlan should be null when vlan_id is nil, got %v", m.Vlan)
	}
	if m.VlanPriority.ValueInt64() != 0 {
		t.Errorf("VlanPriority = %d, want 0", m.VlanPriority.ValueInt64())
	}
	if m.MACAddress.ValueString() != "3c:ec:ef:da:e4:1d" {
		t.Errorf("MACAddress = %q, want 3c:ec:ef:da:e4:1d", m.MACAddress.ValueString())
	}
}

func TestResponseToModel_VlanSet(t *testing.T) {
	api := apiShape()
	vlan := int64(42)
	api.VlanID = &vlan
	api.VlanIDEnable = true

	m := &IPMILanModel{}
	responseToModel(api, m)
	if m.Vlan.IsNull() {
		t.Fatal("Vlan should not be null when vlan_id is set")
	}
	if m.Vlan.ValueInt64() != 42 {
		t.Errorf("Vlan = %d, want 42", m.Vlan.ValueInt64())
	}
}

func TestResponseToModel_DHCPSource(t *testing.T) {
	api := apiShape()
	api.IPAddressSource = "DHCP"

	m := &IPMILanModel{}
	responseToModel(api, m)
	if !m.DHCP.ValueBool() {
		t.Error("DHCP should be true for ip_address_source = \"DHCP\"")
	}
}

// responseToModel deliberately does not touch Password/ApplyRemote; callers
// must preserve whatever the model already had. Verify that a pre-set
// Password value survives a responseToModel call untouched.
func TestResponseToModel_PasswordUntouched(t *testing.T) {
	m := &IPMILanModel{Password: types.StringValue("do-not-clobber")}
	responseToModel(apiShape(), m)
	if m.Password.ValueString() != "do-not-clobber" {
		t.Errorf("Password = %q, want unchanged \"do-not-clobber\"", m.Password.ValueString())
	}
}

// --- responseToDataSourceModel ---------------------------------------------

func TestResponseToDataSourceModel(t *testing.T) {
	m := &IPMILanDataSourceModel{}
	diags := responseToDataSourceModel(apiShape(), m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.Channel.ValueInt64() != 1 {
		t.Errorf("Channel = %d, want 1", m.Channel.ValueInt64())
	}
	if m.DHCP.ValueBool() {
		t.Error("DHCP should be false")
	}
	if !m.Vlan.IsNull() {
		t.Error("Vlan should be null")
	}
}

// --- updatePayload ---------------------------------------------------------

func TestUpdatePayload_DHCPTrueOmitsStaticFields(t *testing.T) {
	m := &IPMILanModel{DHCP: types.BoolValue(true)}
	p, err := m.updatePayload()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p["dhcp"] != true {
		t.Errorf(`p["dhcp"] = %#v, want true`, p["dhcp"])
	}
	for _, k := range []string{"ipaddress", "netmask", "gateway"} {
		if _, ok := p[k]; ok {
			t.Errorf("payload should not include %q when dhcp = true (rejected by the API's DHCP-variant schema)", k)
		}
	}
}

func TestUpdatePayload_DHCPFalseRequiresStaticFields(t *testing.T) {
	m := &IPMILanModel{DHCP: types.BoolValue(false)}
	_, err := m.updatePayload()
	if err != errMissingStaticFields {
		t.Errorf("updatePayload() error = %v, want errMissingStaticFields", err)
	}
}

func TestUpdatePayload_DHCPFalseWithStaticFields(t *testing.T) {
	m := &IPMILanModel{
		DHCP:      types.BoolValue(false),
		IPAddress: types.StringValue("192.168.1.150"),
		Netmask:   types.StringValue("255.255.255.0"),
		Gateway:   types.StringValue("192.168.1.1"),
	}
	p, err := m.updatePayload()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p["dhcp"] != false {
		t.Errorf(`p["dhcp"] = %#v, want false`, p["dhcp"])
	}
	if p["ipaddress"] != "192.168.1.150" {
		t.Errorf(`p["ipaddress"] = %#v, want 192.168.1.150`, p["ipaddress"])
	}
	if p["netmask"] != "255.255.255.0" {
		t.Errorf(`p["netmask"] = %#v, want 255.255.255.0`, p["netmask"])
	}
	if p["gateway"] != "192.168.1.1" {
		t.Errorf(`p["gateway"] = %#v, want 192.168.1.1`, p["gateway"])
	}
}

func TestUpdatePayload_VlanOnlySentWhenConfigured(t *testing.T) {
	m := &IPMILanModel{DHCP: types.BoolValue(true)}
	p, err := m.updatePayload()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p["vlan"]; ok {
		t.Error("payload should not include \"vlan\" when unconfigured")
	}

	m.Vlan = types.Int64Value(100)
	p, err = m.updatePayload()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p["vlan"] != int64(100) {
		t.Errorf(`p["vlan"] = %#v, want int64(100)`, p["vlan"])
	}
}

// --- updatePayload: "vlan" is never sent as an explicit null ------------

// TestUpdatePayload_VlanNeverSentAsExplicitNull covers every shape of
// "vlan unconfigured": updatePayload takes no state into account at all
// (an earlier version did — a "stateVlanSet" parameter — but that was
// reverted; see updatePayload's doc comment for the full reasoning and the
// live-confirmed crash it caused). "vlan" must always stay OMITTED, never
// sent as an explicit null, regardless of what a caller might imagine the
// prior state was — resource.go's Update is responsible for refusing an
// apply outright via vlanCannotBeCleared before ever reaching
// updatePayload, rather than updatePayload silently clearing (or not
// clearing) a tag on its own guess.
func TestUpdatePayload_VlanNeverSentAsExplicitNull(t *testing.T) {
	m := &IPMILanModel{DHCP: types.BoolValue(true)}
	p, err := m.updatePayload()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p["vlan"]; ok {
		t.Errorf(`p["vlan"] = %#v, want key absent`, p["vlan"])
	}
}

// --- vlanCannotBeCleared -------------------------------------------------

func TestVlanCannotBeCleared_StateNullConfigNull(t *testing.T) {
	// "vlan" was never set and still isn't: nothing to clear, no error.
	if vlanCannotBeCleared(types.Int64Null(), types.Int64Null()) {
		t.Error("want false: state.Vlan null, nothing to clear")
	}
}

func TestVlanCannotBeCleared_StateSetConfigMatches(t *testing.T) {
	// The ordinary case: config explicitly (re)affirms the same non-null
	// value the prior state already has.
	if vlanCannotBeCleared(types.Int64Value(100), types.Int64Value(100)) {
		t.Error("want false: config.Vlan is non-null")
	}
}

func TestVlanCannotBeCleared_StateSetConfigNull(t *testing.T) {
	// The exact transition resource.go's Update must refuse rather than
	// guess at: prior state has a non-null "vlan", new config's "vlan" is
	// null — indistinguishable between "the user just asked to clear it"
	// and "vlan was simply never mentioned in this config" (see
	// updatePayload's doc comment for why neither config nor Terraform's
	// own plan can resolve that ambiguity).
	if !vlanCannotBeCleared(types.Int64Value(100), types.Int64Null()) {
		t.Error("want true: state.Vlan non-null, config.Vlan null")
	}
}

func TestVlanCannotBeCleared_StateNullConfigSet(t *testing.T) {
	// Setting a fresh vlan where none existed before is always fine.
	if vlanCannotBeCleared(types.Int64Null(), types.Int64Value(100)) {
		t.Error("want false: state.Vlan already null")
	}
}

func TestUpdatePayload_PasswordOnlySentWhenConfiguredAndNonEmpty(t *testing.T) {
	m := &IPMILanModel{DHCP: types.BoolValue(true)}
	p, _ := m.updatePayload()
	if _, ok := p["password"]; ok {
		t.Error("payload should not include \"password\" when unconfigured")
	}

	m.Password = types.StringValue("")
	p, _ = m.updatePayload()
	if _, ok := p["password"]; ok {
		t.Error("payload should not include \"password\" when configured but empty")
	}

	m.Password = types.StringValue("SuperSecret123!")
	p, _ = m.updatePayload()
	if p["password"] != "SuperSecret123!" {
		t.Errorf(`p["password"] = %#v, want SuperSecret123!`, p["password"])
	}
}

func TestUpdatePayload_ApplyRemoteOnlySentWhenConfigured(t *testing.T) {
	m := &IPMILanModel{DHCP: types.BoolValue(true)}
	p, _ := m.updatePayload()
	if _, ok := p["apply_remote"]; ok {
		t.Error("payload should not include \"apply_remote\" when unconfigured")
	}

	m.ApplyRemote = types.BoolValue(false)
	p, _ = m.updatePayload()
	if p["apply_remote"] != false {
		t.Errorf(`p["apply_remote"] = %#v, want false`, p["apply_remote"])
	}
}

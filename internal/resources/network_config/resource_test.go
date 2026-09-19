// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package network_config

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// TestNetworkConfigSchema_IDIsComputed verifies that "id" is a Computed-only
// StringAttribute with UseStateForUnknown, since it's a fixed singleton
// value never supplied by the user.
func TestNetworkConfigSchema_IDIsComputed(t *testing.T) {
	s := resourceSchema()

	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	idStr, ok := idAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.StringAttribute", idAttr)
	}
	if !idStr.IsComputed() {
		t.Error("'id' should be Computed")
	}
	if idStr.IsRequired() || idStr.IsOptional() {
		t.Error("'id' should be Computed-only (not Required/Optional)")
	}
	if len(idStr.PlanModifiers) == 0 {
		t.Error("'id' should have plan modifiers (UseStateForUnknown)")
	}
}

// TestNetworkConfigSchema_WritableFieldsOptionalComputed verifies that every
// writable field is Optional+Computed with a UseStateForUnknown plan
// modifier.
func TestNetworkConfigSchema_WritableFieldsOptionalComputed(t *testing.T) {
	s := resourceSchema()

	stringFields := []string{
		"hostname", "domain", "httpproxy",
		"ipv4gateway", "ipv6gateway",
		"nameserver1", "nameserver2", "nameserver3",
	}
	for _, name := range stringFields {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		strAttr, ok := attr.(schema.StringAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.StringAttribute", name, attr)
		}
		if !strAttr.IsOptional() || !strAttr.IsComputed() {
			t.Errorf("%q should be Optional+Computed", name)
		}
		if len(strAttr.PlanModifiers) == 0 {
			t.Errorf("%q should have plan modifiers (UseStateForUnknown)", name)
		}
	}

	listFields := []string{"domains", "hosts"}
	for _, name := range listFields {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		listAttr, ok := attr.(schema.ListAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.ListAttribute", name, attr)
		}
		if !listAttr.IsOptional() || !listAttr.IsComputed() {
			t.Errorf("%q should be Optional+Computed", name)
		}
		if len(listAttr.PlanModifiers) == 0 {
			t.Errorf("%q should have plan modifiers (UseStateForUnknown)", name)
		}
	}

	saAttr, ok := s.Attributes["service_announcement"]
	if !ok {
		t.Fatal("schema missing 'service_announcement' attribute")
	}
	saNested, ok := saAttr.(schema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("'service_announcement' attribute is %T, want schema.SingleNestedAttribute", saAttr)
	}
	if !saNested.IsOptional() || !saNested.IsComputed() {
		t.Error("'service_announcement' should be Optional+Computed")
	}
	if len(saNested.PlanModifiers) == 0 {
		t.Error("'service_announcement' should have plan modifiers (UseStateForUnknown)")
	}
	for _, name := range []string{"mdns", "netbios", "wsd"} {
		nested, ok := saNested.Attributes[name]
		if !ok {
			t.Fatalf("'service_announcement' schema missing %q attribute", name)
		}
		boolAttr, ok := nested.(schema.BoolAttribute)
		if !ok {
			t.Fatalf("'service_announcement.%s' attribute is %T, want schema.BoolAttribute", name, nested)
		}
		if !boolAttr.IsRequired() {
			t.Errorf("'service_announcement.%s' should be Required", name)
		}
	}
}

// TestResponseToModel_NullableStringFieldsMapToEmpty verifies that a nil
// ipv4gateway/ipv6gateway/nameserver1-3 on the wire maps to "" (not a
// Terraform null), and that a non-nil value (including an explicit "" from
// a live system) maps through unchanged.
func TestResponseToModel_NullableStringFieldsMapToEmpty(t *testing.T) {
	api := &networkConfigAPI{
		Hostname:    "logrus",
		Domain:      "example.com",
		Domains:     []string{},
		Hosts:       []string{},
		HTTPProxy:   "",
		IPv4Gateway: nil,
		IPv6Gateway: nil,
		Nameserver1: nil,
		Nameserver2: nil,
		Nameserver3: nil,
	}

	m := &NetworkConfigModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	for name, v := range map[string]types.String{
		"ipv4gateway": m.IPv4Gateway,
		"ipv6gateway": m.IPv6Gateway,
		"nameserver1": m.Nameserver1,
		"nameserver2": m.Nameserver2,
		"nameserver3": m.Nameserver3,
	} {
		if v.IsNull() {
			t.Errorf("%s should map to empty string, not null", name)
		}
		if v.ValueString() != "" {
			t.Errorf("%s = %q, want \"\"", name, v.ValueString())
		}
	}

	// A live box has been observed to return "" (not null) directly on the
	// wire for an unset nameserver3; this must map identically to the nil
	// case above.
	empty := ""
	api.Nameserver3 = &empty
	m2 := &NetworkConfigModel{}
	diags = responseToModel(context.Background(), api, m2)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m2.Nameserver3.IsNull() || m2.Nameserver3.ValueString() != "" {
		t.Errorf("Nameserver3 = %v, want empty non-null string", m2.Nameserver3)
	}

	gw := "192.168.1.254"
	ns := "8.8.8.8"
	api.IPv4Gateway = &gw
	api.Nameserver1 = &ns
	m3 := &NetworkConfigModel{}
	diags = responseToModel(context.Background(), api, m3)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m3.IPv4Gateway.ValueString() != gw {
		t.Errorf("IPv4Gateway = %q, want %q", m3.IPv4Gateway.ValueString(), gw)
	}
	if m3.Nameserver1.ValueString() != ns {
		t.Errorf("Nameserver1 = %q, want %q", m3.Nameserver1.ValueString(), ns)
	}
}

// TestResponseToModel_ListsNilMapToEmpty verifies that nil domains/hosts
// from the API map to empty (non-null) Terraform lists.
func TestResponseToModel_ListsNilMapToEmpty(t *testing.T) {
	api := &networkConfigAPI{
		Domains: nil,
		Hosts:   nil,
	}

	m := &NetworkConfigModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	for name, l := range map[string]types.List{
		"domains": m.Domains,
		"hosts":   m.Hosts,
	} {
		if l.IsNull() {
			t.Errorf("%s should be an empty list, not null", name)
		}
		if len(l.Elements()) != 0 {
			t.Errorf("%s should have 0 elements, got %d", name, len(l.Elements()))
		}
	}
}

// TestResponseToModel_ServiceAnnouncementMapping verifies that a present
// service_announcement object on the wire maps to a known (non-null)
// Terraform object with the correct field values, and that a nil
// service_announcement maps to an ObjectNull.
func TestResponseToModel_ServiceAnnouncementMapping(t *testing.T) {
	api := &networkConfigAPI{
		ServiceAnnouncement: &serviceAnnouncementAPI{
			Mdns:    true,
			Netbios: true,
			Wsd:     false,
		},
	}

	m := &NetworkConfigModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.ServiceAnnouncement.IsNull() {
		t.Fatal("ServiceAnnouncement should not be null when API returns a value")
	}
	var sa ServiceAnnouncementModel
	diags = m.ServiceAnnouncement.As(context.Background(), &sa, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("unexpected error decoding ServiceAnnouncement: %v", diags)
	}
	if !sa.Mdns.ValueBool() || !sa.Netbios.ValueBool() || sa.Wsd.ValueBool() {
		t.Errorf("ServiceAnnouncement = %+v, want {mdns:true netbios:true wsd:false}", sa)
	}

	api.ServiceAnnouncement = nil
	m2 := &NetworkConfigModel{}
	diags = responseToModel(context.Background(), api, m2)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !m2.ServiceAnnouncement.IsNull() {
		t.Error("ServiceAnnouncement should be null when API returns nil")
	}
}

// TestResponseToModel_MapsPlainFields verifies that non-nullable,
// non-pointer fields are copied through unchanged.
func TestResponseToModel_MapsPlainFields(t *testing.T) {
	api := &networkConfigAPI{
		ID:        1,
		Hostname:  "logrus",
		Domain:    "example.com",
		Domains:   []string{"example.com"},
		Hosts:     []string{"192.168.1.1 foo"},
		HTTPProxy: "http://proxy.example.com:3128",
	}

	m := &NetworkConfigModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueString() != networkConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), networkConfigResourceID)
	}
	if m.Hostname.ValueString() != api.Hostname {
		t.Errorf("Hostname = %q, want %q", m.Hostname.ValueString(), api.Hostname)
	}
	if m.Domain.ValueString() != api.Domain {
		t.Errorf("Domain = %q, want %q", m.Domain.ValueString(), api.Domain)
	}
	if m.HTTPProxy.ValueString() != api.HTTPProxy {
		t.Errorf("HTTPProxy = %q, want %q", m.HTTPProxy.ValueString(), api.HTTPProxy)
	}
}

// TestUpdatePayload_OnlyKnownFieldsSent verifies that updatePayload omits
// any field whose model value is null or unknown.
func TestUpdatePayload_OnlyKnownFieldsSent(t *testing.T) {
	m := &NetworkConfigModel{
		Hostname:            types.StringNull(),
		Domain:              types.StringValue("example.com"),
		Domains:             types.ListNull(types.StringType),
		Hosts:               types.ListUnknown(types.StringType),
		HTTPProxy:           types.StringUnknown(),
		IPv4Gateway:         types.StringNull(),
		IPv6Gateway:         types.StringUnknown(),
		Nameserver1:         types.StringNull(),
		Nameserver2:         types.StringNull(),
		Nameserver3:         types.StringNull(),
		ServiceAnnouncement: types.ObjectNull(serviceAnnouncementAttrTypes),
	}

	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	for _, key := range []string{
		"hostname", "domains", "hosts", "httpproxy",
		"ipv4gateway", "ipv6gateway", "nameserver1", "nameserver2", "nameserver3",
		"service_announcement",
	} {
		if _, ok := p[key]; ok {
			t.Errorf("expected %q to be omitted (null/unknown)", key)
		}
	}
	if v, ok := p["domain"]; !ok || v != "example.com" {
		t.Errorf("expected 'domain' = example.com, got %v (present=%v)", v, ok)
	}
	if len(p) != 1 {
		t.Errorf("payload has %d keys (%v), want 1", len(p), p)
	}
}

// TestUpdatePayload_AllKnownFieldsSent verifies that updatePayload includes
// every guarded field when its model value is known.
func TestUpdatePayload_AllKnownFieldsSent(t *testing.T) {
	ctx := context.Background()
	domains, _ := types.ListValueFrom(ctx, types.StringType, []string{"example.com"})
	hosts, _ := types.ListValueFrom(ctx, types.StringType, []string{"192.168.1.1 foo"})
	sa, _ := types.ObjectValueFrom(ctx, serviceAnnouncementAttrTypes, ServiceAnnouncementModel{
		Mdns:    types.BoolValue(true),
		Netbios: types.BoolValue(true),
		Wsd:     types.BoolValue(true),
	})

	m := &NetworkConfigModel{
		Hostname:            types.StringValue("logrus"),
		Domain:              types.StringValue("example.com"),
		Domains:             domains,
		Hosts:               hosts,
		HTTPProxy:           types.StringValue("http://proxy.example.com:3128"),
		IPv4Gateway:         types.StringValue("192.168.1.254"),
		IPv6Gateway:         types.StringValue("fe80::1"),
		Nameserver1:         types.StringValue("8.8.8.8"),
		Nameserver2:         types.StringValue("8.8.4.4"),
		Nameserver3:         types.StringValue("1.1.1.1"),
		ServiceAnnouncement: sa,
	}

	p, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	want := map[string]any{
		"hostname":    "logrus",
		"domain":      "example.com",
		"httpproxy":   "http://proxy.example.com:3128",
		"ipv4gateway": "192.168.1.254",
		"ipv6gateway": "fe80::1",
		"nameserver1": "8.8.8.8",
		"nameserver2": "8.8.4.4",
		"nameserver3": "1.1.1.1",
	}
	for k, v := range want {
		if p[k] != v {
			t.Errorf("payload[%q] = %v, want %v", k, p[k], v)
		}
	}
	if v, ok := p["domains"]; !ok {
		t.Error("expected 'domains' present")
	} else if s, ok := v.([]string); !ok || len(s) != 1 || s[0] != "example.com" {
		t.Errorf("payload[domains] = %v, want [example.com]", v)
	}
	if v, ok := p["hosts"]; !ok {
		t.Error("expected 'hosts' present")
	} else if s, ok := v.([]string); !ok || len(s) != 1 || s[0] != "192.168.1.1 foo" {
		t.Errorf("payload[hosts] = %v, want [192.168.1.1 foo]", v)
	}
	if v, ok := p["service_announcement"]; !ok {
		t.Error("expected 'service_announcement' present")
	} else if m, ok := v.(map[string]bool); !ok || !m["mdns"] || !m["netbios"] || !m["wsd"] {
		t.Errorf("payload[service_announcement] = %v, want all-true 3-key map", v)
	}
}

// TestUpdatePayload_GatewayNameserverThreeWay verifies the three-way
// behavior shared by ipv4gateway, ipv6gateway, and nameserver1-3: null and
// unknown omit the key entirely, an explicit "" sends JSON nil (clearing
// the value), and any other value sends that value.
func TestUpdatePayload_GatewayNameserverThreeWay(t *testing.T) {
	fields := []struct {
		key string
		set func(m *NetworkConfigModel, v types.String)
	}{
		{"ipv4gateway", func(m *NetworkConfigModel, v types.String) { m.IPv4Gateway = v }},
		{"ipv6gateway", func(m *NetworkConfigModel, v types.String) { m.IPv6Gateway = v }},
		{"nameserver1", func(m *NetworkConfigModel, v types.String) { m.Nameserver1 = v }},
		{"nameserver2", func(m *NetworkConfigModel, v types.String) { m.Nameserver2 = v }},
		{"nameserver3", func(m *NetworkConfigModel, v types.String) { m.Nameserver3 = v }},
	}

	base := func() *NetworkConfigModel {
		return &NetworkConfigModel{
			Hostname:            types.StringNull(),
			Domain:              types.StringNull(),
			Domains:             types.ListNull(types.StringType),
			Hosts:               types.ListNull(types.StringType),
			HTTPProxy:           types.StringNull(),
			IPv4Gateway:         types.StringNull(),
			IPv6Gateway:         types.StringNull(),
			Nameserver1:         types.StringNull(),
			Nameserver2:         types.StringNull(),
			Nameserver3:         types.StringNull(),
			ServiceAnnouncement: types.ObjectNull(serviceAnnouncementAttrTypes),
		}
	}

	for _, f := range fields {
		t.Run(f.key, func(t *testing.T) {
			// null -> omitted
			m := base()
			f.set(m, types.StringNull())
			p, diags := m.updatePayload(context.Background())
			if diags.HasError() {
				t.Fatalf("unexpected error: %v", diags)
			}
			if _, ok := p[f.key]; ok {
				t.Errorf("expected %q to be omitted when null", f.key)
			}

			// unknown -> omitted
			m = base()
			f.set(m, types.StringUnknown())
			p, diags = m.updatePayload(context.Background())
			if diags.HasError() {
				t.Fatalf("unexpected error: %v", diags)
			}
			if _, ok := p[f.key]; ok {
				t.Errorf("expected %q to be omitted when unknown", f.key)
			}

			// explicit "" -> "" (TrueNAS rejects null for these fields; an
			// empty value must be sent as "" to clear it).
			m = base()
			f.set(m, types.StringValue(""))
			p, diags = m.updatePayload(context.Background())
			if diags.HasError() {
				t.Fatalf("unexpected error: %v", diags)
			}
			v, ok := p[f.key]
			if !ok {
				t.Fatalf("expected %q to be present when explicitly \"\"", f.key)
			}
			if v != "" {
				t.Errorf("%s = %v, want \"\"", f.key, v)
			}

			// value -> value
			m = base()
			f.set(m, types.StringValue("1.2.3.4"))
			p, diags = m.updatePayload(context.Background())
			if diags.HasError() {
				t.Fatalf("unexpected error: %v", diags)
			}
			if v, ok := p[f.key]; !ok || v != "1.2.3.4" {
				t.Errorf("%s = %v (present=%v), want 1.2.3.4", f.key, v, ok)
			}
		})
	}
}

// TestUpdatePayload_DomainsHostsNilToEmpty verifies that a known-but-nil
// ElementsAs result for domains/hosts is normalized to an empty slice
// (rather than sending a Go nil, which would marshal to JSON null instead
// of an empty array).
func TestUpdatePayload_DomainsHostsNilToEmpty(t *testing.T) {
	ctx := context.Background()
	emptyDomains, _ := types.ListValueFrom(ctx, types.StringType, []string{})
	emptyHosts, _ := types.ListValueFrom(ctx, types.StringType, []string{})

	m := &NetworkConfigModel{
		Hostname:            types.StringNull(),
		Domain:              types.StringNull(),
		Domains:             emptyDomains,
		Hosts:               emptyHosts,
		HTTPProxy:           types.StringNull(),
		IPv4Gateway:         types.StringNull(),
		IPv6Gateway:         types.StringNull(),
		Nameserver1:         types.StringNull(),
		Nameserver2:         types.StringNull(),
		Nameserver3:         types.StringNull(),
		ServiceAnnouncement: types.ObjectNull(serviceAnnouncementAttrTypes),
	}

	p, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	for _, key := range []string{"domains", "hosts"} {
		v, ok := p[key]
		if !ok {
			t.Fatalf("expected %q present", key)
		}
		s, ok := v.([]string)
		if !ok {
			t.Fatalf("%s = %T, want []string", key, v)
		}
		if s == nil {
			t.Errorf("%s should be an empty non-nil slice", key)
		}
		if len(s) != 0 {
			t.Errorf("%s should have 0 elements, got %d", key, len(s))
		}
	}
}

// TestUpdatePayload_ServiceAnnouncementOnlyWhenKnown verifies that
// service_announcement is omitted entirely when null/unknown, and sent as
// a full 3-key map only when known.
func TestUpdatePayload_ServiceAnnouncementOnlyWhenKnown(t *testing.T) {
	base := func(sa types.Object) *NetworkConfigModel {
		return &NetworkConfigModel{
			Hostname:            types.StringNull(),
			Domain:              types.StringNull(),
			Domains:             types.ListNull(types.StringType),
			Hosts:               types.ListNull(types.StringType),
			HTTPProxy:           types.StringNull(),
			IPv4Gateway:         types.StringNull(),
			IPv6Gateway:         types.StringNull(),
			Nameserver1:         types.StringNull(),
			Nameserver2:         types.StringNull(),
			Nameserver3:         types.StringNull(),
			ServiceAnnouncement: sa,
		}
	}

	// null -> omitted
	p, diags := base(types.ObjectNull(serviceAnnouncementAttrTypes)).updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, ok := p["service_announcement"]; ok {
		t.Error("expected 'service_announcement' to be omitted when null")
	}

	// unknown -> omitted
	p, diags = base(types.ObjectUnknown(serviceAnnouncementAttrTypes)).updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, ok := p["service_announcement"]; ok {
		t.Error("expected 'service_announcement' to be omitted when unknown")
	}

	// known -> full 3-key map
	ctx := context.Background()
	sa, d := types.ObjectValueFrom(ctx, serviceAnnouncementAttrTypes, ServiceAnnouncementModel{
		Mdns:    types.BoolValue(true),
		Netbios: types.BoolValue(false),
		Wsd:     types.BoolValue(true),
	})
	if d.HasError() {
		t.Fatalf("unexpected error building object: %v", d)
	}
	p, diags = base(sa).updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	v, ok := p["service_announcement"]
	if !ok {
		t.Fatal("expected 'service_announcement' present when known")
	}
	saMap, ok := v.(map[string]bool)
	if !ok {
		t.Fatalf("service_announcement = %T, want map[string]bool", v)
	}
	if len(saMap) != 3 {
		t.Errorf("service_announcement has %d keys, want 3: %v", len(saMap), saMap)
	}
	if !saMap["mdns"] || saMap["netbios"] || !saMap["wsd"] {
		t.Errorf("service_announcement = %v, want {mdns:true netbios:false wsd:true}", saMap)
	}
}

// TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder
// returns exactly one warning (no errors) and does not require or touch a
// client — this is what makes "Delete makes no client calls" verifiable:
// NetworkConfigResource.Delete calls only this pure function.
func TestDeleteWarningDiagnostics(t *testing.T) {
	diags := deleteWarningDiagnostics()
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if len(diags) != 1 {
		t.Fatalf("expected exactly 1 diagnostic, got %d: %v", len(diags), diags)
	}
	summary := diags[0].Summary()
	detail := diags[0].Detail()
	if summary == "" || detail == "" {
		t.Error("expected non-empty summary and detail")
	}
	if detail != "Network configuration left in place; removed from Terraform state only" {
		t.Errorf("detail = %q, want the documented warning text", detail)
	}
}

// TestNetworkConfigDataSourceModel_MatchesSchema verifies that every tfsdk
// tag on NetworkConfigDataSourceModel has a corresponding attribute in the
// datasource schema, and vice versa. terraform-plugin-framework requires an
// exact field/attribute match: any mismatch causes every datasource Read to
// fail with "Struct defines fields not found in object: ...".
func TestNetworkConfigDataSourceModel_MatchesSchema(t *testing.T) {
	d := &NetworkConfigDataSource{}
	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	schemaAttrs := make([]string, 0, len(resp.Schema.Attributes))
	for name := range resp.Schema.Attributes {
		schemaAttrs = append(schemaAttrs, name)
	}
	sort.Strings(schemaAttrs)

	modelType := reflect.TypeOf(NetworkConfigDataSourceModel{})
	modelFields := make([]string, 0, modelType.NumField())
	for i := 0; i < modelType.NumField(); i++ {
		tag := modelType.Field(i).Tag.Get("tfsdk")
		if tag == "" {
			t.Fatalf("field %q has no tfsdk tag", modelType.Field(i).Name)
		}
		modelFields = append(modelFields, tag)
	}
	sort.Strings(modelFields)

	if !reflect.DeepEqual(schemaAttrs, modelFields) {
		t.Fatalf("NetworkConfigDataSourceModel tfsdk tags %v do not match datasource schema attributes %v", modelFields, schemaAttrs)
	}
}

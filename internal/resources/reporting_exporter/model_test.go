// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package reporting_exporter

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// buildAttributes constructs a known (non-null, non-unknown) attributes
// object value for use in tests.
func buildAttributes(t *testing.T, ip string, port int64, namespace, prefix string, updateEvery, bufferOnFailures int64, sendNames bool, matchingCharts string) types.Object {
	t.Helper()
	obj, diags := types.ObjectValueFrom(context.Background(), attributesAttrTypes, AttributesModel{
		DestinationIP:         types.StringValue(ip),
		DestinationPort:       types.Int64Value(port),
		Namespace:             types.StringValue(namespace),
		Prefix:                types.StringValue(prefix),
		UpdateEvery:           types.Int64Value(updateEvery),
		BufferOnFailures:      types.Int64Value(bufferOnFailures),
		SendNamesInsteadOfIDs: types.BoolValue(sendNames),
		MatchingCharts:        types.StringValue(matchingCharts),
	})
	if diags.HasError() {
		t.Fatalf("building attributes object: %v", diags)
	}
	return obj
}

// TestApiPayload_FullySet verifies apiPayload includes name, enabled, and a
// full attributes sub-map (with the fixed exporter_type constant injected),
// matching the probed reporting.exporters.create shape.
func TestApiPayload_FullySet(t *testing.T) {
	ctx := context.Background()
	m := &ReportingExporterModel{
		Name:       types.StringValue("tf-exporter"),
		Enabled:    types.BoolValue(false),
		Attributes: buildAttributes(t, "192.0.2.50", 2003, "tfacc", "scale", 1, 10, true, "*"),
	}

	p, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}

	if p["name"] != "tf-exporter" {
		t.Errorf("payload[name] = %v, want tf-exporter", p["name"])
	}
	if p["enabled"] != false {
		t.Errorf("payload[enabled] = %v, want false", p["enabled"])
	}
	attrs, ok := p["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("payload[attributes] is %T, want map[string]any", p["attributes"])
	}
	if attrs["exporter_type"] != graphiteExporterType {
		t.Errorf("attrs[exporter_type] = %v, want %v", attrs["exporter_type"], graphiteExporterType)
	}
	if attrs["destination_ip"] != "192.0.2.50" {
		t.Errorf("attrs[destination_ip] = %v, want 192.0.2.50", attrs["destination_ip"])
	}
	if attrs["destination_port"] != int64(2003) {
		t.Errorf("attrs[destination_port] = %v, want 2003", attrs["destination_port"])
	}
	if attrs["namespace"] != "tfacc" {
		t.Errorf("attrs[namespace] = %v, want tfacc", attrs["namespace"])
	}
	if attrs["prefix"] != "scale" {
		t.Errorf("attrs[prefix] = %v, want scale", attrs["prefix"])
	}
	if attrs["update_every"] != int64(1) {
		t.Errorf("attrs[update_every] = %v, want 1", attrs["update_every"])
	}
	if attrs["buffer_on_failures"] != int64(10) {
		t.Errorf("attrs[buffer_on_failures] = %v, want 10", attrs["buffer_on_failures"])
	}
	if attrs["send_names_instead_of_ids"] != true {
		t.Errorf("attrs[send_names_instead_of_ids] = %v, want true", attrs["send_names_instead_of_ids"])
	}
	if attrs["matching_charts"] != "*" {
		t.Errorf("attrs[matching_charts] = %v, want *", attrs["matching_charts"])
	}
}

// TestAttributesPayload_UnsetOptionalsOmitted verifies that the
// Optional+Computed GRAPHITE fields are omitted from the attributes payload
// when null/unknown, leaving only exporter_type + the three Required fields
// — so the TrueNAS-side default takes effect for the rest.
func TestAttributesPayload_UnsetOptionalsOmitted(t *testing.T) {
	ctx := context.Background()
	obj, diags := types.ObjectValueFrom(ctx, attributesAttrTypes, AttributesModel{
		DestinationIP:         types.StringValue("192.0.2.50"),
		DestinationPort:       types.Int64Value(2003),
		Namespace:             types.StringValue("tfacc"),
		Prefix:                types.StringNull(),
		UpdateEvery:           types.Int64Unknown(),
		BufferOnFailures:      types.Int64Null(),
		SendNamesInsteadOfIDs: types.BoolUnknown(),
		MatchingCharts:        types.StringNull(),
	})
	if diags.HasError() {
		t.Fatalf("building attributes object: %v", diags)
	}

	p, diags := attributesPayload(ctx, obj)
	if diags.HasError() {
		t.Fatalf("attributesPayload returned diagnostics errors: %v", diags)
	}

	if len(p) != 4 {
		t.Fatalf("payload has %d keys (%v), want 4 (exporter_type, destination_ip, destination_port, namespace)", len(p), p)
	}
	if p["exporter_type"] != graphiteExporterType {
		t.Errorf("payload[exporter_type] = %v, want %v", p["exporter_type"], graphiteExporterType)
	}
}

// TestResponseToModel_ProbedShape verifies responseToModel against the
// exact shape observed from a live reporting.exporters.create/get_instance
// call, including the server-filled GRAPHITE defaults.
func TestResponseToModel_ProbedShape(t *testing.T) {
	ctx := context.Background()
	api := &reportingExporterAPI{
		ID:      1,
		Name:    "tf-probe-exporter",
		Enabled: false,
		Attributes: attributesAPI{
			ExporterType:          "GRAPHITE",
			DestinationIP:         "192.0.2.50",
			DestinationPort:       2003,
			Prefix:                "scale",
			Namespace:             "tfprobe",
			UpdateEvery:           1,
			BufferOnFailures:      10,
			SendNamesInsteadOfIDs: true,
			MatchingCharts:        "*",
		},
	}

	m := &ReportingExporterModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueInt64() != 1 {
		t.Errorf("ID = %v, want 1", m.ID)
	}
	if m.Name.ValueString() != "tf-probe-exporter" {
		t.Errorf("Name = %q, want tf-probe-exporter", m.Name.ValueString())
	}
	if m.Enabled.ValueBool() {
		t.Error("Enabled = true, want false")
	}

	var attrs AttributesModel
	diags = m.Attributes.As(ctx, &attrs, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("reading back attributes: %v", diags)
	}
	if attrs.DestinationIP.ValueString() != "192.0.2.50" {
		t.Errorf("attributes.destination_ip = %q, want 192.0.2.50", attrs.DestinationIP.ValueString())
	}
	if attrs.DestinationPort.ValueInt64() != 2003 {
		t.Errorf("attributes.destination_port = %v, want 2003", attrs.DestinationPort.ValueInt64())
	}
	if attrs.Prefix.ValueString() != "scale" {
		t.Errorf("attributes.prefix = %q, want scale", attrs.Prefix.ValueString())
	}
}

// TestResponseToDataSourceModel_ProbedShape mirrors
// TestResponseToModel_ProbedShape for the datasource model.
func TestResponseToDataSourceModel_ProbedShape(t *testing.T) {
	ctx := context.Background()
	api := &reportingExporterAPI{
		ID:      1,
		Name:    "tf-probe-exporter",
		Enabled: true,
		Attributes: attributesAPI{
			ExporterType:          "GRAPHITE",
			DestinationIP:         "192.0.2.50",
			DestinationPort:       2003,
			Prefix:                "scale",
			Namespace:             "tfprobe",
			UpdateEvery:           1,
			BufferOnFailures:      10,
			SendNamesInsteadOfIDs: true,
			MatchingCharts:        "*",
		},
	}

	m := &ReportingExporterDataSourceModel{}
	diags := responseToDataSourceModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !m.Enabled.ValueBool() {
		t.Error("Enabled = false, want true")
	}
}

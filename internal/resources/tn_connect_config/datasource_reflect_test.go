// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package tn_connect_config

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// TestTnConnectConfigModel_MatchesSchema verifies that every tfsdk tag on
// TnConnectConfigModel has a corresponding attribute in the resource schema,
// and vice versa. terraform-plugin-framework requires an exact field/
// attribute match: any mismatch causes every Create/Read/Update to fail
// with "Struct defines fields not found in object: ...".
func TestTnConnectConfigModel_MatchesSchema(t *testing.T) {
	r := &TnConnectConfigResource{}
	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)

	schemaAttrs := make([]string, 0, len(resp.Schema.Attributes))
	for name := range resp.Schema.Attributes {
		schemaAttrs = append(schemaAttrs, name)
	}
	sort.Strings(schemaAttrs)

	modelFields := tfsdkTags(t, TnConnectConfigModel{})
	sort.Strings(modelFields)

	if !reflect.DeepEqual(schemaAttrs, modelFields) {
		t.Fatalf("TnConnectConfigModel tfsdk tags %v do not match resource schema attributes %v", modelFields, schemaAttrs)
	}
}

// TestTnConnectConfigDataSourceModel_MatchesSchema is the datasource
// counterpart of TestTnConnectConfigModel_MatchesSchema.
func TestTnConnectConfigDataSourceModel_MatchesSchema(t *testing.T) {
	d := &TnConnectConfigDataSource{}
	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	schemaAttrs := make([]string, 0, len(resp.Schema.Attributes))
	for name := range resp.Schema.Attributes {
		schemaAttrs = append(schemaAttrs, name)
	}
	sort.Strings(schemaAttrs)

	modelFields := tfsdkTags(t, TnConnectConfigDataSourceModel{})
	sort.Strings(modelFields)

	if !reflect.DeepEqual(schemaAttrs, modelFields) {
		t.Fatalf("TnConnectConfigDataSourceModel tfsdk tags %v do not match datasource schema attributes %v", modelFields, schemaAttrs)
	}
}

func tfsdkTags(t *testing.T, v any) []string {
	t.Helper()
	modelType := reflect.TypeOf(v)
	fields := make([]string, 0, modelType.NumField())
	for i := 0; i < modelType.NumField(); i++ {
		tag := modelType.Field(i).Tag.Get("tfsdk")
		if tag == "" {
			t.Fatalf("field %q has no tfsdk tag", modelType.Field(i).Name)
		}
		fields = append(fields, tag)
	}
	return fields
}

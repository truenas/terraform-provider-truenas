// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_portal

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// TestISCSIPortalDataSourceModel_MatchesSchema verifies that every tfsdk tag
// on ISCSIPortalDataSourceModel has a corresponding attribute in the
// datasource schema, and vice versa. The terraform-plugin-framework requires
// an exact field/attribute match: any mismatch causes every datasource Read
// to fail with "Struct defines fields not found in object: ...".
func TestISCSIPortalDataSourceModel_MatchesSchema(t *testing.T) {
	d := &ISCSIPortalDataSource{}
	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	schemaAttrs := make([]string, 0, len(resp.Schema.Attributes))
	for name := range resp.Schema.Attributes {
		schemaAttrs = append(schemaAttrs, name)
	}
	sort.Strings(schemaAttrs)

	modelType := reflect.TypeOf(ISCSIPortalDataSourceModel{})
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
		t.Fatalf("ISCSIPortalDataSourceModel tfsdk tags %v do not match datasource schema attributes %v", modelFields, schemaAttrs)
	}
}

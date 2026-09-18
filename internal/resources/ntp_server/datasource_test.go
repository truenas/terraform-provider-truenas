// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ntp_server

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// TestNTPServerDataSourceModel_MatchesSchema verifies that every tfsdk tag on
// NTPServerDataSourceModel has a corresponding attribute in the datasource
// schema, and vice versa. The terraform-plugin-framework requires an exact
// field/attribute match: any mismatch causes every datasource Read to fail
// with "Struct defines fields not found in object: ...". This is the test
// that would have caught NTPServerModel (with its write-only "force" field)
// being reused as the datasource state model.
func TestNTPServerDataSourceModel_MatchesSchema(t *testing.T) {
	d := &NTPServerDataSource{}
	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	schemaAttrs := make([]string, 0, len(resp.Schema.Attributes))
	for name := range resp.Schema.Attributes {
		schemaAttrs = append(schemaAttrs, name)
	}
	sort.Strings(schemaAttrs)

	modelType := reflect.TypeOf(NTPServerDataSourceModel{})
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
		t.Fatalf("NTPServerDataSourceModel tfsdk tags %v do not match datasource schema attributes %v", modelFields, schemaAttrs)
	}
}

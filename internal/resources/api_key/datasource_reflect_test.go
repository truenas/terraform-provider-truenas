// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package api_key

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// TestAPIKeyDataSourceModel_MatchesSchema verifies that every tfsdk tag on
// APIKeyDataSourceModel has a corresponding attribute in the datasource
// schema, and vice versa. terraform-plugin-framework requires an exact
// field/attribute match: any mismatch causes every datasource Read to fail
// with "Struct defines fields not found in object: ...".
func TestAPIKeyDataSourceModel_MatchesSchema(t *testing.T) {
	d := &APIKeyDataSource{}
	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	schemaAttrs := make([]string, 0, len(resp.Schema.Attributes))
	for name := range resp.Schema.Attributes {
		schemaAttrs = append(schemaAttrs, name)
	}
	sort.Strings(schemaAttrs)

	modelType := reflect.TypeOf(APIKeyDataSourceModel{})
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
		t.Fatalf("APIKeyDataSourceModel tfsdk tags %v do not match datasource schema attributes %v", modelFields, schemaAttrs)
	}
}

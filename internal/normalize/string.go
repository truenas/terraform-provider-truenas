// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package normalize

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/attr/xattr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// kind is the per-type behavior plugged into the shared string type. Each
// exported string type (CIDR, URL, ...) is an alias of stringType/stringValue
// instantiated with one kind.
type kind interface {
	// name is the exported type name, used in String() and diagnostics.
	name() string
	// equal reports whether the prior value (config, plan or state) and the
	// value the server returned name the same thing. It is called only with
	// known, non-null values. It must return false, not guess, when either
	// side does not parse.
	equal(prior, proposed string) bool
	// validate returns an error when a configured value cannot be valid for
	// this kind. Empty strings are never passed (see stringValue.ValidateAttribute).
	validate(s string) error
}

// stringType is the attr.Type of a normalized string. Use it through the
// exported aliases (CIDRType, URLType, ...).
type stringType[K kind] struct {
	basetypes.StringType
}

var (
	_ basetypes.StringTypable = stringType[caseInsensitiveKind]{}
)

func (t stringType[K]) Equal(o attr.Type) bool {
	_, ok := o.(stringType[K])
	return ok
}

func (t stringType[K]) String() string {
	var k K
	return "normalize." + k.name() + "Type"
}

func (t stringType[K]) ValueFromString(_ context.Context, in basetypes.StringValue) (basetypes.StringValuable, diag.Diagnostics) {
	return stringValue[K]{StringValue: in}, nil
}

func (t stringType[K]) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	av, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}
	sv, ok := av.(basetypes.StringValue)
	if !ok {
		return nil, fmt.Errorf("unexpected value type %T", av)
	}
	return stringValue[K]{StringValue: sv}, nil
}

func (t stringType[K]) ValueType(context.Context) attr.Value { return stringValue[K]{} }

// stringValue is a string whose equality with the server's spelling is
// decided by its kind. Terraform keeps the prior (configured) spelling in
// state whenever the server value is semantically equal.
type stringValue[K kind] struct {
	basetypes.StringValue
}

var (
	_ basetypes.StringValuableWithSemanticEquals = stringValue[caseInsensitiveKind]{}
	_ xattr.ValidateableAttribute                = stringValue[caseInsensitiveKind]{}
)

func (v stringValue[K]) Type(context.Context) attr.Type { return stringType[K]{} }

func (v stringValue[K]) Equal(o attr.Value) bool {
	other, ok := o.(stringValue[K])
	return ok && v.StringValue.Equal(other.StringValue)
}

// StringSemanticEquals is called by the framework on the new value (from the
// provider's Read/Create/Update) with the prior value (plan or state) as the
// argument. Returning true keeps the prior spelling.
func (v stringValue[K]) StringSemanticEquals(_ context.Context, prior basetypes.StringValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	p, ok := prior.(stringValue[K])
	if !ok {
		return false, diags
	}
	if v.IsNull() || v.IsUnknown() || p.IsNull() || p.IsUnknown() {
		return false, diags
	}
	a, b := p.ValueString(), v.ValueString()
	if a == b {
		return true, diags
	}
	var k K
	return k.equal(a, b), diags
}

// ValidateAttribute rejects configured values that cannot be valid for the
// kind. Null, unknown and "" are not checked: "" is the clear/unset sentinel on
// many TrueNAS fields, and a resource that rejects it says so itself.
func (v stringValue[K]) ValidateAttribute(_ context.Context, req xattr.ValidateAttributeRequest, resp *xattr.ValidateAttributeResponse) {
	if v.IsNull() || v.IsUnknown() || v.ValueString() == "" {
		return
	}
	var k K
	if err := k.validate(v.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid "+k.name()+" value", err.Error())
	}
}

func newValue[K kind](s string) stringValue[K] {
	return stringValue[K]{StringValue: basetypes.NewStringValue(s)}
}

func newNull[K kind]() stringValue[K] {
	return stringValue[K]{StringValue: basetypes.NewStringNull()}
}

func newUnknown[K kind]() stringValue[K] {
	return stringValue[K]{StringValue: basetypes.NewStringUnknown()}
}

func newPointer[K kind](s *string) stringValue[K] {
	return stringValue[K]{StringValue: basetypes.NewStringPointerValue(s)}
}

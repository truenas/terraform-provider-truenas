// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package normalize

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// UnorderedListType is a list type whose values are equal to the server's
// list when they hold the same elements in any order (duplicates counted).
// Elements are compared with their own semantic equality when the element
// type has one (for example a list of CIDR), so both the order and the
// spelling of each element may differ. Build it with UnorderedListOf.
type UnorderedListType struct {
	basetypes.ListType
}

var _ basetypes.ListTypable = UnorderedListType{}

// UnorderedListOf returns the UnorderedListType for the element type. Use the
// same element type for the attribute's ElementType.
func UnorderedListOf(elemType attr.Type) UnorderedListType {
	return UnorderedListType{ListType: basetypes.ListType{ElemType: elemType}}
}

func (t UnorderedListType) Equal(o attr.Type) bool {
	other, ok := o.(UnorderedListType)
	return ok && t.ListType.Equal(other.ListType)
}

func (t UnorderedListType) String() string {
	return fmt.Sprintf("normalize.UnorderedListType[%s]", t.ElementType())
}

func (t UnorderedListType) ValueFromList(_ context.Context, in basetypes.ListValue) (basetypes.ListValuable, diag.Diagnostics) {
	return UnorderedList{ListValue: in}, nil
}

func (t UnorderedListType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	av, err := t.ListType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}
	lv, ok := av.(basetypes.ListValue)
	if !ok {
		return nil, fmt.Errorf("unexpected value type %T", av)
	}
	return UnorderedList{ListValue: lv}, nil
}

func (t UnorderedListType) ValueType(context.Context) attr.Value {
	return UnorderedList{ListValue: basetypes.NewListNull(t.ElementType())}
}

// UnorderedList is the value of an UnorderedListType attribute. It embeds
// basetypes.ListValue, so Elements, ElementsAs, IsNull and IsUnknown work as
// on types.List.
type UnorderedList struct {
	basetypes.ListValue
}

var _ basetypes.ListValuableWithSemanticEquals = UnorderedList{}

func (v UnorderedList) Type(ctx context.Context) attr.Type {
	return UnorderedListOf(v.ElementType(ctx))
}

func (v UnorderedList) Equal(o attr.Value) bool {
	other, ok := o.(UnorderedList)
	return ok && v.ListValue.Equal(other.ListValue)
}

// ListSemanticEquals is called on the new value with the prior value as the
// argument; true keeps the prior order and spelling.
func (v UnorderedList) ListSemanticEquals(ctx context.Context, priorValuable basetypes.ListValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	prior, d := priorValuable.ToListValue(ctx)
	diags.Append(d...)
	if diags.HasError() {
		return false, diags
	}
	if v.IsNull() || v.IsUnknown() || prior.IsNull() || prior.IsUnknown() {
		return false, diags
	}
	got, want := v.Elements(), prior.Elements()
	if len(got) != len(want) {
		return false, diags
	}
	used := make([]bool, len(got))
	for _, w := range want {
		found := false
		for i, g := range got {
			if used[i] {
				continue
			}
			eq, d := elementEqual(ctx, w, g)
			diags.Append(d...)
			if eq {
				used[i] = true
				found = true
				break
			}
		}
		if !found {
			return false, diags
		}
	}
	return true, diags
}

// elementEqual compares one prior and one new element, using the element's
// semantic equality when it has one.
func elementEqual(ctx context.Context, prior, got attr.Value) (bool, diag.Diagnostics) {
	if prior.IsUnknown() || got.IsUnknown() {
		return false, nil
	}
	if prior.Equal(got) {
		return true, nil
	}
	if prior.IsNull() || got.IsNull() {
		return false, nil
	}
	switch g := got.(type) {
	case basetypes.StringValuableWithSemanticEquals:
		if p, ok := prior.(basetypes.StringValuable); ok {
			return g.StringSemanticEquals(ctx, p)
		}
	case basetypes.Int64ValuableWithSemanticEquals:
		if p, ok := prior.(basetypes.Int64Valuable); ok {
			return g.Int64SemanticEquals(ctx, p)
		}
	}
	return false, nil
}

// NewUnorderedListValue builds a known UnorderedList from elements of elemType.
func NewUnorderedListValue(elemType attr.Type, elems []attr.Value) (UnorderedList, diag.Diagnostics) {
	lv, diags := basetypes.NewListValue(elemType, elems)
	return UnorderedList{ListValue: lv}, diags
}

// NewUnorderedListValueFrom builds a known UnorderedList from a Go slice, for
// example []string or []int64, the way types.ListValueFrom does.
func NewUnorderedListValueFrom(ctx context.Context, elemType attr.Type, elems any) (UnorderedList, diag.Diagnostics) {
	lv, diags := basetypes.NewListValueFrom(ctx, elemType, elems)
	return UnorderedList{ListValue: lv}, diags
}

// NewUnorderedListNull returns a null UnorderedList.
func NewUnorderedListNull(elemType attr.Type) UnorderedList {
	return UnorderedList{ListValue: basetypes.NewListNull(elemType)}
}

// NewUnorderedListUnknown returns an unknown UnorderedList.
func NewUnorderedListUnknown(elemType attr.Type) UnorderedList {
	return UnorderedList{ListValue: basetypes.NewListUnknown(elemType)}
}

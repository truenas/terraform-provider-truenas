// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"context"
	"math/big"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// dsValue builds a truenas_dataset object value with every attribute null
// except volsize (nil = null, -1 = unknown).
func dsValue(t *testing.T, volsize *int64) tftypes.Value {
	t.Helper()
	typ := resourceSchema().Type().TerraformType(context.Background()).(tftypes.Object)
	vals := map[string]tftypes.Value{}
	for name, at := range typ.AttributeTypes {
		vals[name] = tftypes.NewValue(at, nil)
	}
	if volsize != nil {
		if *volsize < 0 {
			vals["volsize"] = tftypes.NewValue(tftypes.Number, tftypes.UnknownValue)
		} else {
			vals["volsize"] = tftypes.NewValue(tftypes.Number, new(big.Float).SetInt64(*volsize))
		}
	}
	return tftypes.NewValue(typ, vals)
}

func i64(v int64) *int64 { return &v }

// An unconfigured refreservation goes unknown only when the configured volsize
// changes; an unconfigured volsize (a FILESYSTEM, unknown in the plan before
// its own UseStateForUnknown runs) leaves it alone. (B-038)
func TestRefreservationFollowsVolsize(t *testing.T) {
	s := resourceSchema()
	cases := []struct {
		name          string
		cfgVol, state *int64
		wantUnknown   bool
	}{
		{"volsize grows", i64(2 << 30), i64(1 << 30), true},
		{"volsize unchanged", i64(1 << 30), i64(1 << 30), false},
		{"volsize not configured", nil, i64(0), false},
		{"volsize unknown", i64(-1), i64(1 << 30), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := planmodifier.Int64Request{
				Config:      tfsdk.Config{Schema: s, Raw: dsValue(t, c.cfgVol)},
				Plan:        tfsdk.Plan{Schema: s, Raw: dsValue(t, i64(-1))},
				State:       tfsdk.State{Schema: s, Raw: dsValue(t, c.state)},
				ConfigValue: types.Int64Null(),
				StateValue:  types.Int64Value(1 << 30),
				PlanValue:   types.Int64Value(1 << 30),
			}
			resp := &planmodifier.Int64Response{PlanValue: req.PlanValue}
			RefreservationFollowsVolsize().PlanModifyInt64(context.Background(), req, resp)
			if resp.PlanValue.IsUnknown() != c.wantUnknown {
				t.Errorf("plan = %v, want unknown=%v", resp.PlanValue, c.wantUnknown)
			}
		})
	}
}

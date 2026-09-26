// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestKeepLocalSpecialSmallBlockSize(t *testing.T) {
	null, unknown := types.StringNull(), types.StringUnknown()
	s := types.StringValue
	cases := []struct {
		name        string
		state, plan types.String
		wantErr     bool
	}{
		{"size to INHERIT", s("16384"), s("INHERIT"), true},
		{"size to lower-case inherit", s("0"), s("inherit"), true},
		{"size to another size", s("16384"), s("32768"), false},
		{"INHERIT to size", s("INHERIT"), s("16384"), false},
		{"INHERIT stays INHERIT", s("INHERIT"), s("inherit"), false},
		{"create with INHERIT", null, s("INHERIT"), false},
		{"unknown state", unknown, s("INHERIT"), false},
		{"unknown plan", s("16384"), unknown, false},
		{"destroy", s("16384"), null, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := planmodifier.StringRequest{
				Path:       path.Root("special_small_block_size"),
				StateValue: c.state,
				PlanValue:  c.plan,
			}
			resp := &planmodifier.StringResponse{PlanValue: c.plan}
			keepLocalSpecialSmallBlockSize{}.PlanModifyString(context.Background(), req, resp)
			if got := resp.Diagnostics.HasError(); got != c.wantErr {
				t.Fatalf("error = %v, want %v: %v", got, c.wantErr, resp.Diagnostics)
			}
		})
	}
}

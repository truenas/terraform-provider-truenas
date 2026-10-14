// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TrueNAS 25.10.7 reports explicit replicated ZFS properties as RECEIVED.
// Reading them as inherited causes false drift, including ACL replacement.
func TestResponseToModel_ReceivedProperties(t *testing.T) {
	for _, source := range []string{"LOCAL", "RECEIVED", "INHERITED", "DEFAULT"} {
		t.Run(source, func(t *testing.T) {
			var api apiResponse
			api.Name = "tank/restored"
			api.Compression.Parsed, api.Compression.Source = "lz4", source
			api.AClType.Parsed, api.AClType.Source = "posix", source
			api.ATimeP = sourced("OFF", true, `false`, source)
			api.RecordSizeP = sourced("128K", true, `131072`, source)
			api.SSBSP = sourced("0", true, `0`, source)
			model := DatasetModel{Compression: types.StringValue("LZ4"), AClType: types.StringValue("posix")}
			if diags := (&DatasetResource{}).responseToModel(&api, &model); diags.HasError() {
				t.Fatal(diags)
			}
			explicit := source == "LOCAL" || source == "RECEIVED"
			want := map[string]types.String{
				"compression": types.StringValue("inherit"),
				"acltype":     types.StringValue("inherit"),
				"atime":       types.StringValue("INHERIT"),
				"recordsize":  types.StringNull(),
			}
			wantSize := types.Int64Null()
			if explicit {
				want["compression"] = types.StringValue("LZ4")
				want["acltype"] = types.StringValue("posix")
				want["atime"] = types.StringValue("OFF")
				want["recordsize"] = types.StringValue("128K")
				wantSize = types.Int64Value(0)
			}
			for name, got := range map[string]types.String{
				"compression": model.Compression, "acltype": model.AClType,
				"atime": model.ATime, "recordsize": model.RecordSize,
			} {
				if !got.Equal(want[name]) {
					t.Errorf("%s = %v, want %v", name, got, want[name])
				}
			}
			if !model.SpecialSmallBlockSize.Equal(wantSize) {
				t.Errorf("special_small_block_size = %v, want %v", model.SpecialSmallBlockSize, wantSize)
			}
		})
	}
}

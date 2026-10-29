// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/truenas/terraform-provider-truenas/internal/planmod"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// fakeDeleter answers the pre-check queries from children/snaps (one entry per
// call; the last entry repeats) and records pool.dataset.delete calls.
type fakeDeleter struct {
	children [][]string // names; type is always FILESYSTEM
	snaps    [][]string
	deletes  []map[string]any
	delErr   error
	calls    int
}

func pick(v [][]string, i int) []string {
	if len(v) == 0 {
		return nil
	}
	if i >= len(v) {
		i = len(v) - 1
	}
	return v[i]
}

func (f *fakeDeleter) Call(_ context.Context, method string, _ ...any) (json.RawMessage, error) {
	switch method {
	case "pool.dataset.query":
		var out []map[string]string
		for _, n := range pick(f.children, f.calls) {
			out = append(out, map[string]string{"name": n, "type": "FILESYSTEM"})
		}
		b, _ := json.Marshal(out)
		return b, nil
	case "pool.snapshot.query":
		var out []map[string]string
		for _, n := range pick(f.snaps, f.calls) {
			out = append(out, map[string]string{"name": n})
		}
		f.calls++
		b, _ := json.Marshal(out)
		return b, nil
	}
	return nil, fmt.Errorf("unexpected method %s", method)
}

func (f *fakeDeleter) CallJob(_ context.Context, method string, params ...any) (json.RawMessage, error) {
	if method != "pool.dataset.delete" {
		return nil, fmt.Errorf("unexpected job %s", method)
	}
	f.deletes = append(f.deletes, params[1].(map[string]any))
	if f.delErr != nil {
		return nil, f.delErr
	}
	return json.RawMessage("true"), nil
}

func shrinkSettle(t *testing.T) {
	t.Helper()
	oldT, oldP := deleteSettleTimeout, deleteSettlePoll
	deleteSettleTimeout, deleteSettlePoll = 30*time.Millisecond, 5*time.Millisecond
	t.Cleanup(func() { deleteSettleTimeout, deleteSettlePoll = oldT, oldP })
}

func TestDeleteDataset_emptyIsNonRecursive(t *testing.T) {
	f := &fakeDeleter{}
	if d := DeleteDataset(context.Background(), f, "tank/a", "dataset", false); d.HasError() {
		t.Fatalf("unexpected error: %v", d)
	}
	if len(f.deletes) != 1 || f.deletes[0]["recursive"] != false {
		t.Fatalf("want one recursive=false delete, got %v", f.deletes)
	}
}

func TestDeleteDataset_forceIsRecursiveWithoutCheck(t *testing.T) {
	f := &fakeDeleter{children: [][]string{{"tank/a/b"}}}
	if d := DeleteDataset(context.Background(), f, "tank/a", "dataset", true); d.HasError() {
		t.Fatalf("unexpected error: %v", d)
	}
	if f.calls != 0 || len(f.deletes) != 1 || f.deletes[0]["recursive"] != true {
		t.Fatalf("want one recursive=true delete and no pre-check, got calls=%d deletes=%v", f.calls, f.deletes)
	}
}

func TestDeleteDataset_blockedNamesThemAndDeletesNothing(t *testing.T) {
	shrinkSettle(t)
	var kids []string
	for i := 0; i < 12; i++ {
		kids = append(kids, fmt.Sprintf("tank/a/c%02d", i))
	}
	f := &fakeDeleter{children: [][]string{kids}, snaps: [][]string{{"tank/a@s1"}}}
	d := DeleteDataset(context.Background(), f, "tank/a", "dataset", false)
	if !d.HasError() {
		t.Fatal("want an error")
	}
	if len(f.deletes) != 0 {
		t.Fatalf("nothing may be deleted, got %v", f.deletes)
	}
	msg := d[0].Detail()
	for _, want := range []string{"12 child datasets/zvols and 1 snapshot", "tank/a/c00 (FILESYSTEM)", "tank/a/c09", "... and 3 more", "force_destroy = true", "depends_on"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message lacks %q:\n%s", want, msg)
		}
	}
	if strings.Contains(msg, "tank/a/c10") {
		t.Errorf("message lists more than %d entries:\n%s", maxListedBlockers, msg)
	}
}

func TestDeleteDataset_childVanishesDuringSettle(t *testing.T) {
	shrinkSettle(t)
	deleteSettleTimeout = time.Second
	f := &fakeDeleter{children: [][]string{{"tank/a/b"}, nil}}
	if d := DeleteDataset(context.Background(), f, "tank/a", "dataset", false); d.HasError() {
		t.Fatalf("unexpected error: %v", d)
	}
	if len(f.deletes) != 1 || f.deletes[0]["recursive"] != false {
		t.Fatalf("want one recursive=false delete, got %v", f.deletes)
	}
}

func TestDeleteDataset_serverRefusalWithoutBlockersReportsServerError(t *testing.T) {
	shrinkSettle(t)
	f := &fakeDeleter{delErr: errors.New("dataset is busy")}
	d := DeleteDataset(context.Background(), f, "tank/a", "zvol", false)
	if !d.HasError() || !strings.Contains(d[0].Detail(), "dataset is busy") || d[0].Summary() != "Delete zvol failed" {
		t.Fatalf("want the server error, got %v", d)
	}
}

func TestReplaceIfChangedFromKnownFold(t *testing.T) {
	sch := resourceSchema()
	obj := sch.Type().TerraformType(context.Background())
	nonNull := func() tftypes.Value {
		vals := map[string]tftypes.Value{}
		for k, ty := range obj.(tftypes.Object).AttributeTypes {
			vals[k] = tftypes.NewValue(ty, nil)
		}
		return tftypes.NewValue(obj, vals)
	}
	cases := []struct {
		name        string
		state, plan types.String
		want        bool
	}{
		{"null state (import) records in place", types.StringNull(), types.StringValue("SMB"), false},
		{"case only", types.StringValue("filesystem"), types.StringValue("FILESYSTEM"), false},
		{"real change", types.StringValue("posix"), types.StringValue("nfsv4"), true},
		{"removed", types.StringValue("SMB"), types.StringNull(), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := planmodifier.StringRequest{
				Path:       path.Root("share_type"),
				State:      tfsdk.State{Schema: sch, Raw: nonNull()},
				Plan:       tfsdk.Plan{Schema: sch, Raw: nonNull()},
				StateValue: c.state,
				PlanValue:  c.plan,
			}
			resp := &planmodifier.StringResponse{PlanValue: c.plan}
			planmod.ReplaceFromKnownStringFold().PlanModifyString(context.Background(), req, resp)
			if resp.RequiresReplace != c.want {
				t.Fatalf("RequiresReplace = %v, want %v", resp.RequiresReplace, c.want)
			}
		})
	}
}

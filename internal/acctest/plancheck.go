// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// LogPlanDiff is a plan check that never fails; it logs, per resource change,
// the attributes whose planned value differs from the prior state. Pair it
// with plancheck.ExpectEmptyPlan() so a failing probe says *what* would be
// rewritten, not just that something would.
func LogPlanDiff(t *testing.T) plancheck.PlanCheck { return logPlanDiff{t} }

type logPlanDiff struct{ t *testing.T }

func (c logPlanDiff) CheckPlan(_ context.Context, req plancheck.CheckPlanRequest, _ *plancheck.CheckPlanResponse) {
	for _, rc := range req.Plan.ResourceChanges {
		if rc.Change == nil || len(rc.Change.Actions) == 0 || rc.Change.Actions[0] == "no-op" {
			continue
		}
		before, _ := rc.Change.Before.(map[string]any)
		after, _ := rc.Change.After.(map[string]any)
		unknown, _ := rc.Change.AfterUnknown.(map[string]any)
		var diffs []string
		keys := map[string]bool{}
		for k := range before {
			keys[k] = true
		}
		for k := range after {
			keys[k] = true
		}
		for k := range keys {
			b, _ := json.Marshal(before[k])
			a, _ := json.Marshal(after[k])
			if u, ok := unknown[k]; ok && u == true {
				diffs = append(diffs, fmt.Sprintf("%s: %s -> (known after apply)", k, b))
				continue
			}
			if string(a) != string(b) {
				diffs = append(diffs, fmt.Sprintf("%s: %s -> %s", k, b, a))
			}
		}
		sort.Strings(diffs)
		c.t.Logf("PLAN DIFF %s %v:\n    %s", rc.Address, rc.Change.Actions, strings.Join(diffs, "\n    "))
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// ImportReapplyNoop returns the TestSteps that guard the import lifecycle
// against spurious diffs: forget the resource without destroying it, import it
// back into the working state, then plan the *identical* config.
//
// Import reads state back from the API; attributes the API does not return
// (write-only or create-only inputs) come back null in the imported state. The
// next plan compares the user's config — which sets those inputs — against that
// null state. If such an input is wrongly marked RequiresReplace, the plan
// destroys and recreates a resource the user only imported (GH #29: dataset
// encryption inputs; GH #30: vm_device attributes; BUGS cluster C1).
//
// The steps, appended after the caller's create step:
//  1. re-apply config (no-op) and capture the resource ID;
//  2. the same config with the resource block swapped for a
//     `removed { lifecycle { destroy = false } }` block, so the object leaves
//     state but stays on the server (ImportStatePersist into a state that
//     still holds the resource fails with "Resource already managed");
//  3. `terraform import` with ImportStatePersist, so the imported state
//     replaces the created state;
//  4. plan the identical config: the resource must not be replaced, and only
//     the attributes named in allowedImportDiff may differ (they are recorded
//     in place by the apply);
//  5. plan again: must be empty.
//
// resourceName is the address (e.g. "truenas_dataset.test"). config is the
// exact HCL already applied in the preceding create step (provider block +
// resource blocks); nothing else in it may reference resourceName, since step 2
// removes that block. allowedImportDiff lists the create-only inputs the API
// genuinely cannot return (they read back null after import and are recorded in
// place on the next apply). Pass none when every attribute round-trips, and
// prefer fixing the read over listing an attribute.
func ImportReapplyNoop(resourceName, config string, allowedImportDiff ...string) []resource.TestStep {
	var id string
	return []resource.TestStep{
		{
			Config: config,
			Check: func(s *terraform.State) error {
				rs, ok := s.RootModule().Resources[resourceName]
				if !ok {
					return fmt.Errorf("%s not in state", resourceName)
				}
				id = rs.Primary.ID
				return nil
			},
		},
		{
			Config: removedConfig(config, resourceName),
		},
		{
			Config:             config,
			ResourceName:       resourceName,
			ImportState:        true,
			ImportStatePersist: true,
			ImportStateIdFunc:  func(*terraform.State) (string, error) { return id, nil },
		},
		{
			Config: config,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: importPlanChecks(resourceName, allowedImportDiff),
			},
		},
		{
			Config: config,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
			},
		},
	}
}

func importPlanChecks(resourceName string, allowed []string) []plancheck.PlanCheck {
	if len(allowed) == 0 {
		return []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}
	}
	return []plancheck.PlanCheck{ExpectNoReplace(resourceName), ExpectOnlyAttrChanges(resourceName, allowed...)}
}

// removedConfig returns config with the resource block for addr replaced by a
// removed block that keeps the object on the server. It panics on HCL it
// cannot parse or a missing block: both are test-authoring errors.
func removedConfig(config, addr string) string {
	typ, name, ok := strings.Cut(addr, ".")
	if !ok {
		panic(fmt.Sprintf("ImportReapplyNoop: resource address %q is not <type>.<name>", addr))
	}
	f, diags := hclwrite.ParseConfig([]byte(config), "config.tf", hcl.InitialPos)
	if diags.HasErrors() {
		panic(fmt.Sprintf("ImportReapplyNoop: parse config: %s", diags.Error()))
	}
	body := f.Body()
	blk := body.FirstMatchingBlock("resource", []string{typ, name})
	if blk == nil {
		panic(fmt.Sprintf("ImportReapplyNoop: no resource block %s in config", addr))
	}
	body.RemoveBlock(blk)
	return string(f.Bytes()) + fmt.Sprintf("\nremoved {\n  from = %s\n  lifecycle {\n    destroy = false\n  }\n}\n", addr)
}

// ExpectNoReplace fails when the plan destroys and recreates addr.
func ExpectNoReplace(addr string) plancheck.PlanCheck { return noReplaceCheck{addr: addr} }

type noReplaceCheck struct{ addr string }

func (c noReplaceCheck) CheckPlan(_ context.Context, req plancheck.CheckPlanRequest, resp *plancheck.CheckPlanResponse) {
	for _, rc := range req.Plan.ResourceChanges {
		if rc.Address == c.addr && rc.Change != nil && rc.Change.Actions.Replace() {
			resp.Error = fmt.Errorf("%s: plan replaces the resource (%v), expected no replacement", c.addr, rc.Change.Actions)
			return
		}
	}
}

// ExpectOnlyAttrChanges fails when the planned change for addr sets any
// top-level attribute other than those listed to a different known value.
// Computed attributes that become unknown (re-read by the update) are not
// counted.
func ExpectOnlyAttrChanges(addr string, allowed ...string) plancheck.PlanCheck {
	ok := map[string]bool{}
	for _, a := range allowed {
		ok[a] = true
	}
	return onlyAttrChangesCheck{addr: addr, allowed: ok}
}

type onlyAttrChangesCheck struct {
	addr    string
	allowed map[string]bool
}

func (c onlyAttrChangesCheck) CheckPlan(_ context.Context, req plancheck.CheckPlanRequest, resp *plancheck.CheckPlanResponse) {
	for _, rc := range req.Plan.ResourceChanges {
		if rc.Address != c.addr || rc.Change == nil {
			continue
		}
		before, _ := rc.Change.Before.(map[string]any)
		after, _ := rc.Change.After.(map[string]any)
		unknown, _ := rc.Change.AfterUnknown.(map[string]any)
		keys := map[string]bool{}
		for k := range before {
			keys[k] = true
		}
		for k := range after {
			keys[k] = true
		}
		var bad []string
		for k := range keys {
			// A computed value the update re-reads (unknown after) is not a
			// change the user asked for; the empty plan after the apply proves
			// it settles back to the same value.
			if u, ok := unknown[k].(bool); ok && u {
				continue
			}
			if !reflect.DeepEqual(before[k], after[k]) && !c.allowed[k] {
				bad = append(bad, k)
			}
		}
		if len(bad) > 0 {
			sort.Strings(bad)
			resp.Error = fmt.Errorf("%s: plan changes %v after import; only %v may differ", c.addr, bad, keysOf(c.allowed))
		}
		return
	}
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

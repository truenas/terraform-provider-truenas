package acctest_test

import (
	"os"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestUnrelatedUpdate_Audit applies a resource, changes exactly one unrelated
// field, re-applies, and asserts every other server field (value and property
// source) is unchanged. It catches #61: a comment-only dataset update flipping
// quota/refquota/reservation source DEFAULT -> LOCAL.
func TestUnrelatedUpdate_Audit(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Set TF_ACC=1 (and TRUENAS_* env) to run the live unrelated-update audit")
	}
	wired := map[string]serverTruthResource{}
	for _, r := range serverTruthResources() {
		wired[r.typ] = r
	}
	singletons := map[string]bool{}
	for _, r := range singletonResources() {
		singletons[r.typ] = true
	}
	types := allProviderResourceTypes(t)
	sort.Strings(types)
	for _, typ := range types {
		typ := typ
		if singletons[typ] {
			t.Run(typ, func(t *testing.T) { t.Skip("config singleton: covered by TestSingletonUnrelatedUpdate_Audit") })
			continue
		}
		if lifecycleRatchet[typ].noUpdate {
			t.Run(typ, func(t *testing.T) { t.Skip("by design: no in-place update (lifecycleRatchet noUpdate)") })
			continue
		}
		entry, ok := wired[typ]
		if !ok || entry.perturb == nil {
			reason, ratcheted := unrelatedUpdateRatchet[typ]
			if !ratcheted {
				t.Errorf("%s has no perturb spec in serverTruthResources() and is not in unrelatedUpdateRatchet", typ)
				continue
			}
			t.Run(typ, func(t *testing.T) { t.Skip(reason) })
			continue
		}
		t.Run(typ, func(t *testing.T) {
			if entry.minMajor > 0 && !acctest.ServerVersionAtLeast(t, entry.minMajor, entry.minMinor) {
				t.Skipf("%s requires TrueNAS %d.%d+", typ, entry.minMajor, entry.minMinor)
			}
			name := acctest.RandName("tfuu")
			pt := entry.perturb
			changed := pt.changedServerPaths
			if len(changed) == 0 {
				changed = []string{apiPath(typ, pt.attr)}
			}
			var before map[string]any
			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { acctest.PreCheck(t) },
				ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config: acctest.ProviderConfig() + pt.configBefore(name),
						Check: func(s *terraform.State) error {
							before = entry.fetch(t, s)
							return nil
						},
					},
					{
						Config: acctest.ProviderConfig() + pt.configAfter(name),
						Check: func(s *terraform.State) error {
							after := entry.fetch(t, s)
							acctest.AssertUnchangedExcept(t, before, after, changed, entry.volatile)
							if typ == "truenas_dataset" {
								// #61: sources must not flip on a comment-only update.
								for _, k := range []string{"quota", "refquota", "reservation"} {
									p := k + ".source"
									want, ok := acctest.ServerPath(before, p)
									ws, isStr := want.(string)
									if !ok || !isStr {
										t.Errorf("dataset %s missing or non-string in server instance; #61 guard not comparing it", p)
										continue
									}
									acctest.AssertServerSource(t, after, p, ws)
								}
							}
							return nil
						},
					},
				},
			})
		})
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

// lifecyclePhases records which lifecycle phases a resource's acceptance test
// is (still) allowed to skip, and why. See TestLifecycleCoverage_Audit.
//
// Two kinds of entry:
//   - by design: the phase is structurally impossible (an immutable resource
//     has no in-place update; a resource with no ImportState has no import).
//     These are permanent — the reason says "by design".
//   - debt: the phase IS supported but the test doesn't exercise it yet.
//     These must shrink: add the step, delete the entry.
type lifecyclePhases struct {
	noUpdate, noImport bool
	reason             string
}

// lifecycleRatchet is the shrinking allowlist of resources whose acceptance
// test does not exercise UPDATE and/or IMPORT. A new resource must ship both
// steps (or an immutable/no-import resource gets a "by design" entry here with
// the justification) — never add a plain debt entry to dodge the check.
var lifecycleRatchet = map[string]lifecyclePhases{
	// --- Update impossible by design (permanent) ---
	"truenas_snapshot": {noUpdate: true,
		reason: "by design: no Update method; dataset+name are RequiresReplace, a snapshot is immutable"},
	"truenas_nvmet_host_subsys": {noUpdate: true,
		reason: "by design: pure join; host_id+subsys_id are the only attrs and both RequiresReplace"},

	// --- Update supported but untested (debt: add a second changed-config step) ---
	"truenas_cloud_backup":       {noUpdate: true, reason: "debt: cloud.backup.update implemented, test is single-config"},
	"truenas_enclosure_label":    {noUpdate: true, reason: "debt: label update implemented, test is single-config"},
	"truenas_ipmi_lan":           {noUpdate: true, reason: "debt: ipmi.lan update implemented, test is set-and-restore single-config"},
	"truenas_network_interface":  {noUpdate: true, reason: "debt: interface.update implemented, test is single-config"},
	"truenas_truecommand_config": {noUpdate: true, reason: "debt: truecommand.update implemented, test is set-and-restore single-config"},

	// --- Import supported but untested (debt: add an ImportState step) ---
	"truenas_directoryservices": {noImport: true, reason: "debt: ImportState implemented; test needs a joined-directory fixture"},
}

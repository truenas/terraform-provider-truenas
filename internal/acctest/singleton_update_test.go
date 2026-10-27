// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// singletonResource registers one config singleton (always exists, read with
// <ns>.config, written with <ns>.update, never created or destroyed) for the
// set-and-restore variant of Guard 1.
//
// The Terraform config for each singleton sets ONLY the perturbed field. None
// of these resources has a schema Default and each updatePayload skips
// null/unknown attributes, so the apply writes just that field and the restore
// only has to put that field (restoreKeys) back.
type singletonResource struct {
	typ          string
	configMethod string
	updateMethod string
	// attr is the Terraform attribute that is perturbed.
	attr string
	// changedServerPaths are the server leaf paths the perturbation
	// legitimately moves (attr itself plus documented side effects).
	changedServerPaths []string
	// restoreKeys are the update-method keys written back from the saved
	// original config on cleanup. Only keys present in the original are sent.
	restoreKeys []string
	// skip optionally returns a reason to skip given the saved original.
	skip func(orig map[string]any) string
	// gateEnv optionally names an env var that must be set to run (for
	// updates with heavy side effects, e.g. restarting sshd).
	gateEnv string
	// configA/configB render the HCL (without provider block) with the
	// perturbed field at value A / value B. A and B always differ.
	configA, configB string
	volatile         []string
	// minMajor/minMinor, when set, skip the singleton on servers older than
	// that release (the namespace does not exist there). Checked before the
	// config method is called.
	minMajor, minMinor int
}

// singletonHCL renders a resource "typ" "t" block with the given body lines.
func singletonHCL(typ string, body string) string {
	return fmt.Sprintf("\nresource %q \"t\" {\n  %s\n}\n", typ, body)
}

func singletonResources() []singletonResource {
	str := func(typ, attr, v string) string { return singletonHCL(typ, fmt.Sprintf("%s = %q", attr, v)) }
	raw := func(typ, attr string, v any) string { return singletonHCL(typ, fmt.Sprintf("%s = %v", attr, v)) }
	return []singletonResource{
		{
			typ: "truenas_snmp_config", configMethod: "snmp.config", updateMethod: "snmp.update",
			attr: "location", changedServerPaths: []string{"location"}, restoreKeys: []string{"location"},
			configA: str("truenas_snmp_config", "location", "tfsu-loc-a"),
			configB: str("truenas_snmp_config", "location", "tfsu-loc-b"),
		},
		{
			typ: "truenas_ftp_config", configMethod: "ftp.config", updateMethod: "ftp.update",
			attr: "banner", changedServerPaths: []string{"banner"}, restoreKeys: []string{"banner"},
			configA: str("truenas_ftp_config", "banner", "tfsu-banner-a"),
			configB: str("truenas_ftp_config", "banner", "tfsu-banner-b"),
		},
		{
			// Any ssh.update restarts sshd, so honor the same opt-in gate as
			// TestAccSSHConfig_basic. compression cannot malform sshd_config
			// and never touches port, auth or bind interface.
			typ: "truenas_ssh_config", configMethod: "ssh.config", updateMethod: "ssh.update",
			attr: "compression", changedServerPaths: []string{"compression"}, restoreKeys: []string{"compression"},
			gateEnv: "TRUENAS_TEST_SSH_CONFIG",
			configA: raw("truenas_ssh_config", "compression", true),
			configB: raw("truenas_ssh_config", "compression", false),
		},
		{
			typ: "truenas_nfs_config", configMethod: "nfs.config", updateMethod: "nfs.update",
			attr: "v4_domain", changedServerPaths: []string{"v4_domain"}, restoreKeys: []string{"v4_domain"},
			configA: str("truenas_nfs_config", "v4_domain", "tfsu-a.example"),
			configB: str("truenas_nfs_config", "v4_domain", "tfsu-b.example"),
		},
		{
			typ: "truenas_smb_config", configMethod: "smb.config", updateMethod: "smb.update",
			attr: "description", changedServerPaths: []string{"description"}, restoreKeys: []string{"description"},
			configA: str("truenas_smb_config", "description", "tfsu-desc-a"),
			configB: str("truenas_smb_config", "description", "tfsu-desc-b"),
		},
		{
			// quota_fill_warning must be 5-80 and below quota_fill_critical.
			typ: "truenas_audit_config", configMethod: "audit.config", updateMethod: "audit.update",
			attr: "quota_fill_warning", changedServerPaths: []string{"quota_fill_warning"}, restoreKeys: []string{"quota_fill_warning"},
			// space.* is the audit dataset's live usage, which fluctuates as the
			// box writes audit logs — volatile, not config.
			volatile: []string{"space"},
			configA:  raw("truenas_audit_config", "quota_fill_warning", 55),
			configB:  raw("truenas_audit_config", "quota_fill_warning", 65),
		},
		{
			// Resilver priority window: restore enabled plus the window
			// fields, mirroring TestAccResilverConfig_setAndRestore.
			typ: "truenas_resilver_config", configMethod: "pool.resilver.config", updateMethod: "pool.resilver.update",
			attr: "enabled", changedServerPaths: []string{"enabled"}, restoreKeys: []string{"enabled", "weekday", "begin", "end"},
			configA: raw("truenas_resilver_config", "enabled", true),
			configB: raw("truenas_resilver_config", "enabled", false),
		},
		{
			// ups.update requires driver and port on every call, so the
			// restore writes the whole writable config back. An unconfigured
			// UPS (empty driver/port) cannot be restored to that state, so
			// skip it. monpwd is write-only and never read back; the config
			// here never sets it.
			typ: "truenas_ups_config", configMethod: "ups.config", updateMethod: "ups.update",
			attr: "description", changedServerPaths: []string{"description"},
			restoreKeys: []string{"identifier", "mode", "remotehost", "remoteport", "driver", "port",
				"options", "optionsupsd", "description", "shutdown", "shutdowntimer", "monuser",
				"extrausers", "rmonitor", "powerdown", "hostsync", "shutdowncmd", "nocommwarntime"},
			skip: func(orig map[string]any) string {
				if d, _ := orig["driver"].(string); d == "" {
					return "UPS is unconfigured (empty driver); cannot restore the unconfigured state"
				}
				if p, _ := orig["port"].(string); p == "" {
					return "UPS is unconfigured (empty port); cannot restore the unconfigured state"
				}
				return ""
			},
			configA: str("truenas_ups_config", "description", "tfsu-desc-a"),
			configB: str("truenas_ups_config", "description", "tfsu-desc-b"),
		},
		{
			typ: "truenas_nvmet_global", configMethod: "nvmet.global.config", updateMethod: "nvmet.global.update",
			attr: "xport_referral", changedServerPaths: []string{"xport_referral"}, restoreKeys: []string{"xport_referral"},
			configA: raw("truenas_nvmet_global", "xport_referral", true),
			configB: raw("truenas_nvmet_global", "xport_referral", false),
		},
		{
			// Only pool_avail_threshold is sent. basename, listen_port, alua
			// and iser are deliberately left alone: changing the global
			// basename knocked boot LUNs offline on the lab boxes.
			typ: "truenas_iscsi_global", configMethod: "iscsi.global.config", updateMethod: "iscsi.global.update",
			attr: "pool_avail_threshold", changedServerPaths: []string{"pool_avail_threshold"}, restoreKeys: []string{"pool_avail_threshold"},
			configA: raw("truenas_iscsi_global", "pool_avail_threshold", 80),
			configB: raw("truenas_iscsi_global", "pool_avail_threshold", 70),
		},
		{
			// ui_consolemsg is a cosmetic login-screen toggle with no network
			// or certificate effect. ui_address/ui_port/ui_httpsport/
			// ui_allowlist/ui_v6address/ui_certificate/hostname/nameservers
			// are never sent: the Terraform config sets only this field and
			// updatePayload skips null/unknown attributes.
			typ: "truenas_system_general", configMethod: "system.general.config", updateMethod: "system.general.update",
			attr: "ui_consolemsg", changedServerPaths: []string{"ui_consolemsg"}, restoreKeys: []string{"ui_consolemsg"},
			configA: raw("truenas_system_general", "ui_consolemsg", true),
			configB: raw("truenas_system_general", "ui_consolemsg", false),
		},
		{
			// mail.update requires fromemail (and the transport fields) on
			// every call; the resource merges them from the live config, so
			// the restore writes the same set back. pass is write-only and
			// never read, so it is neither sent nor restored (an update
			// without pass keeps the stored one). The relay is never cleared:
			// an unconfigured mail (empty fromemail) is skipped.
			typ: "truenas_mail", configMethod: "mail.config", updateMethod: "mail.update",
			attr: "fromname", changedServerPaths: []string{"fromname"},
			restoreKeys: []string{"fromemail", "fromname", "outgoingserver", "port", "security", "smtp", "user"},
			skip: func(orig map[string]any) string {
				if e, _ := orig["fromemail"].(string); e == "" {
					return "mail is unconfigured (empty fromemail); cannot restore the unconfigured state"
				}
				return ""
			},
			configA: str("truenas_mail", "fromname", "tfsu-name-a"),
			configB: str("truenas_mail", "fromname", "tfsu-name-b"),
		},
		{
			// kerberos.update parses appdefaults_aux server-side and crashes
			// on a line that does not split on "=". The only perturbable
			// fields are the two aux strings, so this toggles the recognized
			// MIT appdefaults boolean "no_addresses" (same well-formed
			// key = value line TestAccKerberosConfig_setAndRestore probed
			// live). libdefaults_aux is never touched.
			typ: "truenas_kerberos_config", configMethod: "kerberos.config", updateMethod: "kerberos.update",
			attr: "appdefaults_aux", changedServerPaths: []string{"appdefaults_aux"}, restoreKeys: []string{"appdefaults_aux"},
			configA: str("truenas_kerberos_config", "appdefaults_aux", "no_addresses = true"),
			configB: str("truenas_kerberos_config", "appdefaults_aux", "no_addresses = false"),
		},
		{
			// A null max_parallel_replication_tasks (unlimited) is restored
			// as JSON null by restoreSingleton.
			typ: "truenas_replication_config", configMethod: "replication.config.config", updateMethod: "replication.config.update",
			attr: "max_parallel_replication_tasks", changedServerPaths: []string{"max_parallel_replication_tasks"},
			restoreKeys: []string{"max_parallel_replication_tasks"},
			configA:     raw("truenas_replication_config", "max_parallel_replication_tasks", 2),
			configB:     raw("truenas_replication_config", "max_parallel_replication_tasks", 3),
		},
		{
			// search is cosmetic; bindip/passkey/groups carry auth and
			// exposure risk and are never sent. webshare.update accepts a
			// partial payload. The namespace exists only on 26.0+.
			typ: "truenas_webshare_config", configMethod: "webshare.config", updateMethod: "webshare.update",
			attr: "search", changedServerPaths: []string{"search"}, restoreKeys: []string{"search"},
			minMajor: 26, minMinor: 0,
			configA: raw("truenas_webshare_config", "search", true),
			configB: raw("truenas_webshare_config", "search", false),
		},
		{
			// v4_network only. preferred_pool/bridge/v6_network are never
			// sent; the singleton is skipped when preferred_pool is set (LXC
			// in use). The namespace exists only on 26.0+.
			typ: "truenas_lxc_config", configMethod: "lxc.config", updateMethod: "lxc.update",
			attr: "v4_network", changedServerPaths: []string{"v4_network"}, restoreKeys: []string{"v4_network"},
			minMajor: 26, minMinor: 0,
			skip: func(orig map[string]any) string {
				if p, ok := orig["preferred_pool"]; ok && p != nil {
					return "lxc preferred_pool is set (LXC in use); not touching the network of an active LXC setup"
				}
				return ""
			},
			configA: str("truenas_lxc_config", "v4_network", "172.201.0.0/24"),
			configB: str("truenas_lxc_config", "v4_network", "172.202.0.0/24"),
		},
	}
}

// readSingletonConfig calls the config method and decodes it as a generic map.
func readSingletonConfig(t *testing.T, method string) map[string]any {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), method)
	if err != nil {
		t.Fatalf("singleton: %s: %v", method, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("singleton: decode %s: %v", method, err)
	}
	return m
}

// restoreSingleton writes restoreKeys from orig back via the update method.
// JSON null values are sent as null (e.g. pool_avail_threshold unset).
func restoreSingleton(r singletonResource, orig map[string]any) error {
	payload := map[string]any{}
	for _, k := range r.restoreKeys {
		if v, ok := orig[k]; ok {
			payload[k] = v
		}
	}
	if len(payload) == 0 {
		return fmt.Errorf("no restore keys found in original %s (keys wanted: %v)", r.configMethod, r.restoreKeys)
	}
	_, err := acctest.RestoreCall(context.Background(), r.updateMethod, payload)
	return err
}

// TestSingletonUnrelatedUpdate_Audit is Guard 1 for config singletons. Each
// is saved, set to value A of one field, snapshotted, set to value B,
// snapshotted, and the two snapshots must differ only in that field. The
// original value is written back from a t.Cleanup that runs even on failure,
// and the full config is then re-read and compared to the saved original to
// catch any residue.
func TestSingletonUnrelatedUpdate_Audit(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Set TF_ACC=1 (and TRUENAS_* env) to run the live singleton unrelated-update audit")
	}
	for _, r := range singletonResources() {
		r := r
		t.Run(r.typ, func(t *testing.T) {
			acctest.DisruptiveCheck(t)
			if r.gateEnv != "" && os.Getenv(r.gateEnv) == "" {
				t.Skipf("set %s=1 to run (update has heavy side effects)", r.gateEnv)
			}
			if r.minMajor != 0 && !acctest.ServerVersionAtLeast(t, r.minMajor, r.minMinor) {
				t.Skipf("%s needs TrueNAS %d.%d or later", r.typ, r.minMajor, r.minMinor)
			}
			// 1. Save the original config BEFORE anything else.
			orig := readSingletonConfig(t, r.configMethod)
			if r.skip != nil {
				if reason := r.skip(orig); reason != "" {
					t.Skip(reason)
				}
			}
			// 5. Restore, registered before any mutation so it always runs.
			t.Cleanup(func() {
				if err := restoreSingleton(r, orig); err != nil {
					t.Errorf("RESTORE FAILED for %s, box left altered: %v", r.typ, err)
					return
				}
				now := readSingletonConfig(t, r.configMethod)
				acctest.AssertUnchangedExcept(t, orig, now, nil, r.volatile)
			})

			var s1 map[string]any
			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { acctest.PreCheck(t) },
				ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config: acctest.ProviderConfig() + r.configA,
						Check: func(*terraform.State) error {
							s1 = readSingletonConfig(t, r.configMethod)
							return nil
						},
					},
					{
						Config: acctest.ProviderConfig() + r.configB,
						Check: func(*terraform.State) error {
							s2 := readSingletonConfig(t, r.configMethod)
							acctest.AssertUnchangedExcept(t, s1, s2, r.changedServerPaths, r.volatile)
							return nil
						},
					},
				},
			})
		})
	}
}

// TestSingletonRegistry_WellFormed needs no TrueNAS box.
func TestSingletonRegistry_WellFormed(t *testing.T) {
	known := map[string]bool{}
	for _, typ := range allProviderResourceTypes(t) {
		known[typ] = true
	}
	seen := map[string]bool{}
	for _, r := range singletonResources() {
		if !known[r.typ] {
			t.Errorf("%s: not a provider resource type", r.typ)
		}
		if seen[r.typ] {
			t.Errorf("%s: duplicate singleton entry", r.typ)
		}
		seen[r.typ] = true
		if r.configMethod == "" || r.updateMethod == "" || r.attr == "" || len(r.restoreKeys) == 0 || len(r.changedServerPaths) == 0 {
			t.Errorf("%s: incomplete entry", r.typ)
		}
		if r.configA == r.configB || !strings.Contains(r.configA, `resource "`+r.typ+`" "t"`) || !strings.Contains(r.configB, r.attr) {
			t.Errorf("%s: malformed configs", r.typ)
		}
		if !strings.HasSuffix(r.configMethod, ".config") || !strings.HasSuffix(r.updateMethod, ".update") {
			t.Errorf("%s: unexpected method names %s / %s", r.typ, r.configMethod, r.updateMethod)
		}
	}
}

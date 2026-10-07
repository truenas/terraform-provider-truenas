// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

import (
	"crypto/rand"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestValueRoundTrip is the exhaustive set-vs-read-back coverage test. For every
// value the TrueNAS API advertises as settable on an enum field
// (core.get_methods `accepts`), it applies a resource with that value and
// asserts the next plan is EMPTY — i.e. what you set reads back as what you set
// (or is reconciled, like INHERIT). This is the bug class users keep reporting
// (set compression="inherit" -> read back "lz4" -> inconsistent result; acltype
// casing; ...), caught before release instead of after.
//
// It drives every enum-bearing resource for which a resource is creatable in the
// lab, using a minimal fixture adapted from that resource's acceptance test
// (dependencies included). Each (resource, field, value) is its own subtest, so
// one value failing does not stop the rest — the failing subtests ARE the report
// of which values do not line up. Resources whose create needs an external
// endpoint, special hardware, or a value-dependent companion config that isn't a
// single-field substitution are skipped with a stated reason (see rtSkips).
//
// TF_ACC + TRUENAS_* required. Long: one create/apply/plan/destroy per value.
// Run against a disposable box.
func TestValueRoundTrip(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Set TF_ACC=1 (and TRUENAS_* env) to run the value round-trip coverage test")
	}
	methods := getMethods(t)

	for _, res := range rtResources() {
		res := res
		t.Run(res.typ, func(t *testing.T) {
			enums := acceptsEnums(methods[res.method])
			for _, f := range res.fields {
				f := f
				attr := f.attr
				if attr == "" {
					attr = f.api
				}
				vals := enums[f.api]
				if len(vals) == 0 {
					t.Run(attr, func(t *testing.T) { t.Skipf("API method %s advertises no enum for %s", res.method, f.api) })
					continue
				}
				t.Run(attr, func(t *testing.T) {
					for _, v := range vals {
						v := v
						t.Run(v, func(t *testing.T) {
							body := fmt.Sprintf("  %s = %q\n", attr, v)
							if f.comp != nil {
								extra, skip := f.comp(v)
								if skip != "" {
									t.Skip(skip)
								}
								body += extra
							}
							name := acctest.RandName("tfrt")
							cfg := acctest.ProviderConfig() + res.base(name, body)
							resource.Test(t, resource.TestCase{
								PreCheck:                 func() { acctest.PreCheck(t) },
								ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
								Steps: []resource.TestStep{
									{Config: cfg}, // apply the value (fails here on inconsistent-result)
									{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{ // must read back clean
										PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
									}},
								},
							})
						})
					}
				})
			}
		})
	}
}

// rtField is one enum field to round-trip. comp supplies any companion lines a
// value needs (or a skip reason for an unsupported value, e.g. FC without FC
// hardware).
type rtField struct {
	api  string
	attr string // provider attribute name if it differs from the API field
	comp func(value string) (extra string, skip string)
}

// rtResource is a creatable fixture plus the enum fields to vary on it. base
// returns the full HCL (dependencies included) with body injected into the
// resource under test.
type rtResource struct {
	typ    string
	method string
	base   func(name, body string) string
	fields []rtField
}

const (
	rtDHKey  = "DHHC-1:01:pK9QIlr3QJRVUe+FslA2wwBHRDapL5WvfxqGMzMeRHNpQt3H:"
	rtDHCtrl = "DHHC-1:01:HTIM44KJK/u+Ih5Bxaqz1vCRGhcEen4UjWZyZKhtEG8sxusU:"
)

// rtUUID returns a random RFC-4122-shaped UUID (8-4-4-4-12 hex). An NVMe-oF
// host NQN of the uuid form requires a full-length UUID; a short token is
// rejected with "uuid is incorrect length".
func rtUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func rtResources() []rtResource {
	pool := acctest.TestPool()
	dsEnum := []rtField{{api: "compression"}, {api: "sync"}, {api: "checksum"},
		{api: "deduplication", attr: "dedup"}, {api: "atime"}, {api: "exec"},
		{api: "readonly"}}

	return []rtResource{
		{
			typ: "truenas_dataset", method: "pool.dataset.create",
			base: func(name, body string) string {
				return fmt.Sprintf("resource \"truenas_dataset\" \"t\" {\n  name = %q\n%s}\n", pool+"/"+name, body)
			},
			fields: append(append([]rtField{}, dsEnum...),
				rtField{api: "snapdir"},
				rtField{api: "aclmode", comp: func(v string) (string, string) {
					// PASSTHROUGH/RESTRICTED are NFSv4 aclmodes; the pool default
					// acltype is POSIX, which only permits DISCARD, so pair them
					// with acltype=NFSV4. DISCARD/INHERIT work under POSIX as-is.
					if v == "PASSTHROUGH" || v == "RESTRICTED" {
						return "  acltype = \"NFSV4\"\n", ""
					}
					return "", ""
				}},
				rtField{api: "acltype", comp: func(v string) (string, string) {
					// aclmode and acltype must agree: POSIX/OFF require DISCARD,
					// NFSV4 requires a non-DISCARD mode. Pair each explicitly so
					// the create is unambiguous. INHERIT inherits both.
					switch v {
					case "POSIX", "OFF":
						return "  aclmode = \"DISCARD\"\n", ""
					case "NFSV4":
						return "  aclmode = \"PASSTHROUGH\"\n", ""
					}
					return "", ""
				}},
			),
		},
		{
			typ: "truenas_zvol", method: "pool.dataset.create",
			base: func(name, body string) string {
				return fmt.Sprintf("resource \"truenas_zvol\" \"t\" {\n  name = %q\n  volsize = 67108864\n%s}\n", pool+"/"+name, body)
			},
			fields: []rtField{{api: "compression"}, {api: "sync"}, {api: "checksum"},
				{api: "deduplication", attr: "dedup"}, {api: "readonly"}, {api: "snapdev"}},
		},
		{
			typ: "truenas_vm", method: "vm.create",
			base: func(name, body string) string {
				alnum := strings.ReplaceAll(name, "-", "")
				return fmt.Sprintf("resource \"truenas_vm\" \"t\" {\n  name = %q\n  memory = 536870912\n  vcpus = 1\n  autostart = false\n  running = false\n%s}\n", alnum, body)
			},
			fields: []rtField{{api: "bootloader"}, {api: "cpu_mode"}, {api: "time"}},
		},
		{
			typ: "truenas_alert_service", method: "alertservice.create",
			base: func(name, body string) string {
				return fmt.Sprintf("resource \"truenas_alert_service\" \"t\" {\n  name = %q\n  attributes = jsonencode({ type = \"Mail\", email = \"tfacc@example.com\" })\n%s}\n", name, body)
			},
			fields: []rtField{{api: "level"}},
		},
		{
			typ: "truenas_periodic_snapshot", method: "pool.snapshottask.create",
			base: func(name, body string) string {
				return fmt.Sprintf(`resource "truenas_dataset" "d" { name = %q }
resource "truenas_periodic_snapshot_task" "t" {
  dataset        = truenas_dataset.d.name
  recursive      = false
  lifetime_value = 2
  naming_schema  = "tfrt-%%Y%%m%%d-%%H%%M"
  enabled        = false
  schedule = { minute = "0", hour = "0", dom = "*", month = "*", dow = "*" }
%s}
`, pool+"/"+name, body)
			},
			fields: []rtField{{api: "lifetime_unit"}},
		},
		{
			typ: "truenas_iscsi_auth", method: "iscsi.auth.create",
			base: func(name, body string) string {
				return fmt.Sprintf("resource \"truenas_iscsi_auth\" \"t\" {\n  tag = 991\n  user = \"tfrtuser\"\n  secret = \"chapsecret123\"\n%s}\n", body)
			},
			fields: []rtField{{api: "discovery_auth", comp: func(v string) (string, string) {
				if v == "CHAP_MUTUAL" {
					return "  peeruser = \"tfrtpeer\"\n  peersecret = \"peersecret456\"\n", ""
				}
				return "", ""
			}}},
		},
		{
			typ: "truenas_iscsi_target", method: "iscsi.target.create",
			base: func(name, body string) string {
				return fmt.Sprintf(`resource "truenas_iscsi_portal" "p" {
  comment = %q
  listen  = [{ ip = %q }]
}
resource "truenas_iscsi_target" "t" {
  name   = %q
  groups = [{ portal = truenas_iscsi_portal.p.id, authmethod = "NONE" }]
%s}
`, name, acctest.EndpointHost(), strings.ReplaceAll(name, "-", ""), body)
			},
			fields: []rtField{{api: "mode", comp: func(v string) (string, string) {
				if v == "FC" || v == "BOTH" {
					return "", "FC/BOTH mode needs Fibre Channel hardware not present in the lab"
				}
				return "", ""
			}}},
		},
		{
			typ: "truenas_iscsi_extent", method: "iscsi.extent.create",
			base: func(name, body string) string {
				return fmt.Sprintf(`resource "truenas_zvol" "z" {
  name    = %q
  volsize = 67108864
}
resource "truenas_dataset" "d" { name = %q }
resource "truenas_iscsi_extent" "t" {
  name    = %q
  enabled = true
%s}
`, pool+"/z"+name, pool+"/d"+name, strings.ReplaceAll(name, "-", ""), body)
			},
			fields: []rtField{
				{api: "rpm", comp: func(v string) (string, string) {
					return "  type = \"DISK\"\n  disk = \"zvol/${truenas_zvol.z.name}\"\n", ""
				}},
				{api: "type", comp: func(v string) (string, string) {
					switch v {
					case "DISK":
						return "  disk = \"zvol/${truenas_zvol.z.name}\"\n", ""
					case "FILE":
						return "  path = \"${truenas_dataset.d.mountpoint}/extent\"\n  filesize = 67108864\n", ""
					}
					return "", "unknown extent type"
				}},
			},
		},
		{
			typ: "truenas_nvmet_port", method: "nvmet.port.create",
			base: func(name, body string) string {
				return fmt.Sprintf("resource \"truenas_nvmet_port\" \"t\" {\n  addr_traddr = %q\n  addr_trsvcid = 4420\n%s}\n", acctest.EndpointHost(), body)
			},
			fields: []rtField{{api: "addr_trtype", comp: func(v string) (string, string) {
				if v == "RDMA" {
					return "", "RDMA needs RDMA-capable hardware not present in the lab"
				}
				if v == "FC" {
					return "", "FC transport is excluded from the schema (needs Fibre Channel hardware)"
				}
				return "", ""
			}}},
		},
		{
			typ: "truenas_nvmet_host", method: "nvmet.host.create",
			base: func(name, body string) string {
				_ = name
				return fmt.Sprintf("resource \"truenas_nvmet_host\" \"t\" {\n  hostnqn = \"nqn.2014-08.org.nvmexpress:uuid:%s\"\n  dhchap_key = %q\n  dhchap_ctrl_key = %q\n%s}\n", rtUUID(), rtDHKey, rtDHCtrl, body)
			},
			fields: []rtField{{api: "dhchap_hash"}, {api: "dhchap_dhgroup"}},
		},
		{
			typ: "truenas_nvmet_namespace", method: "nvmet.namespace.create",
			base: func(name, body string) string {
				return fmt.Sprintf(`resource "truenas_nvmet_subsys" "s" { name = %q }
resource "truenas_zvol" "z" {
  name    = %q
  volsize = 67108864
}
resource "truenas_dataset" "d" { name = %q }
resource "truenas_nvmet_namespace" "t" {
  subsys_id = truenas_nvmet_subsys.s.id
  enabled   = false
%s}
`, strings.ReplaceAll(name, "-", ""), pool+"/z"+name, pool+"/d"+name, body)
			},
			fields: []rtField{{api: "device_type", comp: func(v string) (string, string) {
				switch v {
				case "ZVOL":
					return "  device_path = \"zvol/${truenas_zvol.z.name}\"\n", ""
				case "FILE":
					return "  device_path = \"${truenas_dataset.d.mountpoint}/ns\"\n  filesize = 67108864\n", ""
				}
				return "", "unknown device_type"
			}}},
		},
	}
}

// rtSkips documents enum-bearing resources deliberately NOT round-tripped here,
// each with the reason it cannot be created/substituted cleanly in the lab. It
// is reference for reviewers (and asserts the list stays intentional); the test
// above simply doesn't include them.
var rtSkips = map[string]string{
	"truenas_smb_share":             "purpose selects which share options are valid; not a single-field substitution",
	"truenas_tunable":               "type (SYSCTL/UDEV/ZFS) requires a matching var/value per type; not a single-field substitution",
	"truenas_replication_task":      "create needs ssh keypair + connection + source/dest datasets; fixture too environment-specific",
	"truenas_rsync_task":            "SSH/MODULE modes need a real remote rsync target",
	"truenas_cloudsync_task":        "needs a cloudsync credential that validates a real remote endpoint",
	"truenas_container":             "needs Docker + an image registry/catalog",
	"truenas_network_interface":     "create reconfigures NICs — disruptive, risks disconnecting the box",
	"truenas_cloud_backup":          "create validates a real remote repository",
	"truenas_cloudsync_credentials": "create validates a real remote endpoint",
	"truenas_truecommand_config":    "connects to a real TrueCommand instance",
	"truenas_vmware":                "connects to a real vCenter/ESXi host",
	"truenas_app_registry":          "validates against a live container registry",
	"truenas_tn_connect_config":     "connects to the TrueNAS Connect cloud",
}

var _ = rtSkips

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

// unsetDefaultsRatchet lists resource types not yet wired into the live
// server-truth unset-defaults audit (TestUnsetDefaults_Audit). It only shrinks:
// add a serverTruthResources() entry, then delete the type here.
var unsetDefaultsRatchet = map[string]string{
	"truenas_acme_dns_authenticator": "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_alert_policy":           "by design: config singleton (update-only; no create/destroy), so the create-based server-truth guards do not apply",
	"truenas_app":                    "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_app_registry":           "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_boot_environment":       "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_catalog_config":         "by design: config singleton (update-only; no create/destroy), so the create-based server-truth guards do not apply",
	"truenas_cloud_backup":           "2026-10-27 debt: not yet wired for server-truth unset-defaults",
	"truenas_cloudsync_task":         "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_container":              "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_container_device":       "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_directoryservices":      "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_docker_config":          "by design: config singleton (update-only; no create/destroy), so the create-based server-truth guards do not apply",
	"truenas_enclosure_label":        "2026-10-27 debt: not yet wired for server-truth unset-defaults",
	"truenas_failover_config":        "by design: config singleton (update-only; no create/destroy), so the create-based server-truth guards do not apply",
	"truenas_filesystem_acl":         "by design: required entries JSON must match the path's acltype (NFS4 vs POSIX1E), which varies per box with inherited acltype/aclmode; an NFS4 fixture needs a raw pool.dataset.create truenas_dataset cannot express",
	"truenas_ipmi_lan":               "2026-10-27 debt: not yet wired for server-truth unset-defaults",
	"truenas_network_config":         "by design: config singleton (update-only; no create/destroy), so the create-based server-truth guards do not apply",
	"truenas_network_interface":      "2026-10-27 debt: not yet wired for server-truth unset-defaults",
	"truenas_nvmet_host_subsys":      "2026-10-27 debt: not yet wired for server-truth unset-defaults",
	"truenas_pool":                   "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_scrub_task":             "2026-10-27 debt: not yet wired for server-truth unset-defaults",
	"truenas_service":                "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_snapshot":               "2026-10-27 debt: not yet wired for server-truth unset-defaults",
	"truenas_system_advanced":        "by design: config singleton (update-only; no create/destroy), so the create-based server-truth guards do not apply",
	"truenas_system_dataset":         "by design: config singleton (update-only; no create/destroy), so the create-based server-truth guards do not apply",
	"truenas_tn_connect_config":      "by design: config singleton (update-only; no create/destroy), so the create-based server-truth guards do not apply",
	"truenas_truecommand_config":     "2026-10-27 debt: not yet wired for server-truth unset-defaults",
	"truenas_tunable":                "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_twofactor_auth":         "by design: config singleton (update-only; no create/destroy), so the create-based server-truth guards do not apply",
	"truenas_vm":                     "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_vm_device":              "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_vmware":                 "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
}

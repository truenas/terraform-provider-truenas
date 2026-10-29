// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

// unrelatedUpdateRatchet lists resource types that have an in-place update
// but no Guard-1 perturb spec yet (TestUnrelatedUpdate_Audit). Only shrinks:
// add a perturb spec to serverTruthResources() and delete the entry.
// Types with no in-place update (lifecycleRatchet noUpdate) are skipped by
// design and are not listed here.
var unrelatedUpdateRatchet = map[string]string{
	"truenas_keychain_ssh_connection": "2026-10-27 wired for Guard 2; Guard-1 perturb needs a stable-key fixture (stSSHConnectionConfig regenerates the keypair per render, replacing the dependent keypair mid-update)",
	"truenas_acme_dns_authenticator":  "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_alert_policy":            "by design: config singleton (update-only; no create/destroy), so the create-based server-truth guards do not apply",
	"truenas_app":                     "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_app_registry":            "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_boot_environment":        "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_catalog_config":          "by design: config singleton (update-only; no create/destroy), so the create-based server-truth guards do not apply",
	"truenas_cloudsync_task":          "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_container":               "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_container_device":        "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_directoryservices":       "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_docker_config":           "by design: config singleton (update-only; no create/destroy), so the create-based server-truth guards do not apply",
	"truenas_failover_config":         "by design: config singleton (update-only; no create/destroy), so the create-based server-truth guards do not apply",
	"truenas_filesystem_acl":          "by design: required entries JSON must match the path's acltype (NFS4 vs POSIX1E), which varies per box with inherited acltype/aclmode; an NFS4 fixture needs a raw pool.dataset.create truenas_dataset cannot express",
	"truenas_network_config":          "by design: config singleton (update-only; no create/destroy), so the create-based server-truth guards do not apply",
	"truenas_nvmet_port_subsys":       "2026-10-27 debt: no Guard-1 perturb spec yet",
	"truenas_pool":                    "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_scrub_task":              "2026-10-27 debt: no Guard-1 perturb spec yet",
	"truenas_service":                 "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_system_advanced":         "by design: config singleton (update-only; no create/destroy), so the create-based server-truth guards do not apply",
	"truenas_system_dataset":          "by design: config singleton (update-only; no create/destroy), so the create-based server-truth guards do not apply",
	"truenas_tn_connect_config":       "by design: config singleton (update-only; no create/destroy), so the create-based server-truth guards do not apply",
	"truenas_tunable":                 "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_twofactor_auth":          "by design: config singleton (update-only; no create/destroy), so the create-based server-truth guards do not apply",
	"truenas_vm":                      "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_vm_device":               "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
	"truenas_vmware":                  "by design: not creatable on a disposable box (external endpoint / hardware / destructive), out of scope for the server-truth guards",
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"golang.org/x/crypto/ssh"

	"github.com/truenas/terraform-provider-truenas/internal/acctest"
	"github.com/truenas/terraform-provider-truenas/internal/provider"
)

// perturbSpec describes a one-attribute change used by the server-truth
// update guard: apply configBefore, then configAfter, and assert the server
// changed only attr.
type perturbSpec struct {
	attr string
	// changedServerPaths are the server leaf paths the perturbation
	// legitimately moves (the server may nest or alias attr). Empty means
	// apiPath(typ, attr).
	changedServerPaths        []string
	configBefore, configAfter func(name string) string
}

// serverTruthResource registers one resource for the server-truth guards.
//
// Convention: the resource under test is always labeled "t", i.e. the state
// address is "<typ>.t". minimalConfig(name) returns required-only HCL
// (without the provider block) that creates that resource plus any fixtures
// it depends on.
type serverTruthResource struct {
	typ           string
	createMethod  string
	fetch         func(t *testing.T, s *terraform.State) map[string]any
	minimalConfig func(name string) string
	perturb       *perturbSpec
	volatile      []string
	// minMajor/minMinor gate a resource whose API namespace only exists from
	// that TrueNAS version on; the live audits skip it on older servers.
	minMajor, minMinor int
}

// defaultFetch reads the Terraform id at state address addr and returns the
// server's <ns>.get_instance for it. The namespace is createMethod minus
// ".create". Numeric ids are sent as integers (the API keys such resources by
// int); anything else (e.g. a dataset's path) is sent as a string.
func defaultFetch(createMethod, addr string) func(*testing.T, *terraform.State) map[string]any {
	ns := strings.TrimSuffix(createMethod, ".create")
	return func(t *testing.T, s *terraform.State) map[string]any {
		t.Helper()
		rs, ok := s.RootModule().Resources[addr]
		if !ok {
			t.Fatalf("server-truth: %s not in state", addr)
		}
		var id any = rs.Primary.ID
		if n, err := strconv.ParseInt(rs.Primary.ID, 10, 64); err == nil {
			id = n
		}
		return acctest.GetInstance(t, ns, id)
	}
}

// allProviderResourceTypes returns every resource type the provider registers.
func allProviderResourceTypes(t *testing.T) []string {
	t.Helper()
	ctx := context.Background()
	p := provider.New("test")()
	var out []string
	for _, ctor := range providerResources(ctx, p) {
		var md fwresource.MetadataResponse
		ctor().Metadata(ctx, fwresource.MetadataRequest{ProviderTypeName: "truenas"}, &md)
		out = append(out, md.TypeName)
	}
	return out
}

func serverTruthResources() []serverTruthResource {
	return []serverTruthResource{
		{
			typ:           "truenas_dataset",
			createMethod:  "pool.dataset.create",
			fetch:         defaultFetch("pool.dataset.create", "truenas_dataset.t"),
			minimalConfig: func(name string) string { return stDatasetConfig(name, "") },
			// Space accounting moves with pool free space between reads. quota,
			// refquota and reservation must stay out of this list: their
			// .source is what the #61 guard checks.
			volatile: []string{"available", "used", "referenced", "written",
				"usedbychildren", "usedbydataset", "usedbysnapshots",
				"usedbyrefreservation", "logicalused", "logicalreferenced"},
			perturb: &perturbSpec{
				// Perturb compression (not comments) and leave comments UNSET, so
				// that a regression which writes org.freenas:description as a
				// LOCAL "" on an unrelated update is caught here — the original
				// #61/#62 class. comments is nested under user_properties, so if
				// it appeared it would surface as an unrelated-field change.
				attr:               "compression",
				changedServerPaths: []string{"compression"},
				configBefore:       func(name string) string { return stDatasetCompressionConfig(name, "LZ4") },
				configAfter:        func(name string) string { return stDatasetCompressionConfig(name, "ZSTD") },
			},
		},
		{
			typ:           "truenas_certificate",
			createMethod:  "certificate.create",
			fetch:         defaultFetch("certificate.create", "truenas_certificate.t"),
			minimalConfig: func(name string) string { return stCertificateConfig(name, "") },
			// certificate.update only accepts name, renew_days (ACME only) and
			// add_to_trusted_store (not for CSRs); a CSR certificate can only be
			// renamed in place, so name is the one perturbable field.
			perturb: &perturbSpec{
				attr: "name",
				// The on-disk cert files are named after the certificate, so a
				// rename moves them; those path fields change as a legitimate
				// consequence of the name change, not unrelated drift.
				changedServerPaths: []string{"name", "csr_path", "privatekey_path", "certificate_path", "fullchain_path"},
				configBefore:       func(name string) string { return stCertificateConfig(name, "") },
				configAfter:        func(name string) string { return stCertificateConfig(name, "-b") },
			},
		},
		{
			typ:           "truenas_keychain_ssh_connection",
			createMethod:  "keychaincredential.create",
			fetch:         defaultFetch("keychaincredential.create", "truenas_keychain_ssh_connection.t"),
			minimalConfig: func(name string) string { return stSSHConnectionConfig(name, "") },
			// No Guard-1 perturb: stSSHConnectionConfig regenerates the ed25519
			// keypair on every render, so configBefore/configAfter would carry
			// different key material and replace the dependent keypair mid-update
			// (destroy fails while the connection still references it). Guard 2
			// covers this resource; wiring Guard 1 needs a stable-key fixture.
		},
		{
			typ:           "truenas_user",
			createMethod:  "user.create",
			fetch:         defaultFetch("user.create", "truenas_user.t"),
			minimalConfig: func(name string) string { return stUserConfig(name, "TF Server Truth") },
			// By design (#48), user.update re-sends the write-only password from
			// config on every update, so the server rehashes it: unixhash gets a
			// new salt, password_history grows, last_password_change bumps. That
			// is deliberate behavior, not unrelated drift, so these churn fields
			// are volatile here. (See docs-dev note on the _wo_version trade-off.)
			volatile: []string{"unixhash", "password_history", "last_password_change"},
			perturb: &perturbSpec{
				attr:         "full_name",
				configBefore: func(name string) string { return stUserConfig(name, "TF Before") },
				configAfter:  func(name string) string { return stUserConfig(name, "TF After") },
			},
		},
		{
			typ:           "truenas_replication_task",
			createMethod:  "replication.create",
			fetch:         defaultFetch("replication.create", "truenas_replication_task.t"),
			minimalConfig: func(name string) string { return stReplicationConfig(name, "") },
			// Toggling enabled can move the task's runtime state/job summary.
			volatile: []string{"state", "job"},
			perturb: &perturbSpec{
				attr:         "enabled",
				configBefore: func(name string) string { return stReplicationConfig(name, "true") },
				configAfter:  func(name string) string { return stReplicationConfig(name, "false") },
			},
		},
		{
			typ:           "truenas_cronjob",
			createMethod:  "cronjob.create",
			fetch:         defaultFetch("cronjob.create", "truenas_cronjob.t"),
			minimalConfig: func(name string) string { return stCronjobConfig(name, "") },
			perturb: &perturbSpec{
				attr:         "description",
				configBefore: func(name string) string { return stCronjobConfig(name, "before") },
				configAfter:  func(name string) string { return stCronjobConfig(name, "after") },
			},
		},
		{
			typ:           "truenas_init_shutdown_script",
			createMethod:  "initshutdownscript.create",
			fetch:         defaultFetch("initshutdownscript.create", "truenas_init_shutdown_script.t"),
			minimalConfig: func(name string) string { return stInitScriptConfig(name, "") },
			perturb: &perturbSpec{
				attr:         "comment",
				configBefore: func(name string) string { return stInitScriptConfig(name, "before") },
				configAfter:  func(name string) string { return stInitScriptConfig(name, "after") },
			},
		},
		// truenas_scrub_task is intentionally not wired: TrueNAS allows one scrub
		// schedule per pool and the test pool ships with its default one, so a
		// create-from-scratch needs a dedicated throwaway pool.
		{
			typ:           "truenas_ntp_server",
			createMethod:  "system.ntpserver.create",
			fetch:         defaultFetch("system.ntpserver.create", "truenas_ntp_server.t"),
			minimalConfig: func(name string) string { return stNTPConfig(name, "") },
			perturb: &perturbSpec{
				attr:         "prefer",
				configBefore: func(name string) string { return stNTPConfig(name, "false") },
				configAfter:  func(name string) string { return stNTPConfig(name, "true") },
			},
		},
		{
			typ:           "truenas_static_route",
			createMethod:  "staticroute.create",
			fetch:         defaultFetch("staticroute.create", "truenas_static_route.t"),
			minimalConfig: func(name string) string { return stStaticRouteConfig(name, "") },
			perturb: &perturbSpec{
				attr:         "description",
				configBefore: func(name string) string { return stStaticRouteConfig(name, "before") },
				configAfter:  func(name string) string { return stStaticRouteConfig(name, "after") },
			},
		},
		{
			typ:           "truenas_nvmet_host",
			createMethod:  "nvmet.host.create",
			fetch:         defaultFetch("nvmet.host.create", "truenas_nvmet_host.t"),
			minimalConfig: func(name string) string { return stNVMetHostConfig(name, "a") },
			perturb: &perturbSpec{
				// hostnqn is the only field present on every supported version
				// (description is 26.0+); it updates in place.
				attr:         "hostnqn",
				configBefore: func(name string) string { return stNVMetHostConfig(name, "a") },
				configAfter:  func(name string) string { return stNVMetHostConfig(name, "b") },
			},
		},
		{
			typ:           "truenas_cloudsync_credentials",
			createMethod:  "cloudsync.credentials.create",
			fetch:         defaultFetch("cloudsync.credentials.create", "truenas_cloudsync_credentials.t"),
			minimalConfig: func(name string) string { return stCloudsyncCredsConfig(name, "") },
			perturb: &perturbSpec{
				attr:         "name",
				configBefore: func(name string) string { return stCloudsyncCredsConfig(name, "") },
				configAfter:  func(name string) string { return stCloudsyncCredsConfig(name, "-b") },
			},
		},
		{
			typ:           "truenas_filesystem_permissions",
			createMethod:  "filesystem.setperm",
			fetch:         fsStatFetch("truenas_filesystem_permissions.t"),
			minimalConfig: func(name string) string { return stFSPermsConfig(name, "") },
			// filesystem.stat reports per-read timestamps; chmod moves ctime.
			volatile: []string{"atime", "mtime", "ctime"},
			perturb: &perturbSpec{
				attr:         "mode",
				configBefore: func(name string) string { return stFSPermsConfig(name, "0750") },
				configAfter:  func(name string) string { return stFSPermsConfig(name, "0700") },
			},
		},
		// truenas_filesystem_acl is intentionally not wired: its required
		// "entries" JSON must match the path's acltype (NFS4 vs POSIX1E), which
		// depends on the pool root's inherited acltype/aclmode and differs per
		// box; an NFS4 fixture needs a raw pool.dataset.create with
		// aclmode=PASSTHROUGH that truenas_dataset cannot express.
		// truenas_cloudsync_task is intentionally not wired: cloudsync.create
		// validates the bucket/folder against the real remote provider, so it
		// cannot be created on a disposable box without a live endpoint.
		// truenas_app_registry is intentionally not wired: app.registry.create
		// validates username/password/uri against the real registry endpoint
		// before persisting, so it cannot be created on a disposable box.
		{
			typ:           "truenas_nfs_share",
			createMethod:  "sharing.nfs.create",
			fetch:         defaultFetch("sharing.nfs.create", "truenas_nfs_share.t"),
			minimalConfig: func(name string) string { return stNFSShareConfig(name, "") },
			perturb: &perturbSpec{
				attr:         "comment",
				configBefore: func(name string) string { return stNFSShareConfig(name, "before") },
				configAfter:  func(name string) string { return stNFSShareConfig(name, "after") },
			},
		},
		{
			typ:           "truenas_smb_share",
			createMethod:  "sharing.smb.create",
			fetch:         defaultFetch("sharing.smb.create", "truenas_smb_share.t"),
			minimalConfig: func(name string) string { return stSMBShareConfig(name, "") },
			perturb: &perturbSpec{
				attr:         "comment",
				configBefore: func(name string) string { return stSMBShareConfig(name, "before") },
				configAfter:  func(name string) string { return stSMBShareConfig(name, "after") },
			},
		},
		{
			typ:           "truenas_group",
			createMethod:  "group.create",
			fetch:         defaultFetch("group.create", "truenas_group.t"),
			minimalConfig: func(name string) string { return stGroupConfig(name, "") },
			perturb: &perturbSpec{
				attr:         "smb",
				configBefore: func(name string) string { return stGroupConfig(name, "false") },
				configAfter:  func(name string) string { return stGroupConfig(name, "true") },
				// Enabling smb makes the server assign a Windows SID; that is a
				// legitimate downstream effect of the smb change, not an
				// unrelated-field drift.
				changedServerPaths: []string{"smb", "sid"},
			},
		},
		{
			typ:           "truenas_api_key",
			createMethod:  "api_key.create",
			fetch:         defaultFetch("api_key.create", "truenas_api_key.t"),
			minimalConfig: func(name string) string { return stAPIKeyConfig(name, "") },
			perturb: &perturbSpec{
				attr:         "name",
				configBefore: func(name string) string { return stAPIKeyConfig(name, "") },
				configAfter:  func(name string) string { return stAPIKeyConfig(name, "-b") },
			},
		},
		{
			typ:           "truenas_alert_service",
			createMethod:  "alertservice.create",
			fetch:         defaultFetch("alertservice.create", "truenas_alert_service.t"),
			minimalConfig: func(name string) string { return stAlertServiceConfig(name, "WARNING") },
			perturb: &perturbSpec{
				attr:         "level",
				configBefore: func(name string) string { return stAlertServiceConfig(name, "WARNING") },
				configAfter:  func(name string) string { return stAlertServiceConfig(name, "ERROR") },
			},
		},
		{
			typ:           "truenas_privilege",
			createMethod:  "privilege.create",
			fetch:         defaultFetch("privilege.create", "truenas_privilege.t"),
			minimalConfig: func(name string) string { return stPrivilegeConfig(name, "") },
			perturb: &perturbSpec{
				attr:         "name",
				configBefore: func(name string) string { return stPrivilegeConfig(name, "") },
				configAfter:  func(name string) string { return stPrivilegeConfig(name, "-b") },
			},
		},
		{
			typ:           "truenas_iscsi_auth",
			createMethod:  "iscsi.auth.create",
			fetch:         defaultFetch("iscsi.auth.create", "truenas_iscsi_auth.t"),
			minimalConfig: func(name string) string { return stISCSIAuthConfig(name, "") },
			perturb: &perturbSpec{
				attr:         "user",
				configBefore: func(name string) string { return stISCSIAuthConfig(name, "") },
				configAfter:  func(name string) string { return stISCSIAuthConfig(name, "-b") },
			},
		},
		{
			typ:           "truenas_iscsi_initiator",
			createMethod:  "iscsi.initiator.create",
			fetch:         defaultFetch("iscsi.initiator.create", "truenas_iscsi_initiator.t"),
			minimalConfig: func(name string) string { return stISCSIInitiatorConfig(name, "") },
			perturb: &perturbSpec{
				attr:         "comment",
				configBefore: func(name string) string { return stISCSIInitiatorConfig(name, "before") },
				configAfter:  func(name string) string { return stISCSIInitiatorConfig(name, "after") },
			},
		},
		{
			typ:           "truenas_kerberos_realm",
			createMethod:  "kerberos.realm.create",
			fetch:         defaultFetch("kerberos.realm.create", "truenas_kerberos_realm.t"),
			minimalConfig: func(name string) string { return stKerberosRealmConfig(name, "") },
			perturb: &perturbSpec{
				attr:         "admin_server",
				configBefore: func(name string) string { return stKerberosRealmConfig(name, "") },
				configAfter:  func(name string) string { return stKerberosRealmConfig(name, `["192.0.2.89"]`) },
			},
		},
		{
			typ:           "truenas_acl_template",
			createMethod:  "filesystem.acltemplate.create",
			fetch:         defaultFetch("filesystem.acltemplate.create", "truenas_acl_template.t"),
			minimalConfig: func(name string) string { return stACLTemplateConfig(name, "") },
			perturb: &perturbSpec{
				attr:         "comment",
				configBefore: func(name string) string { return stACLTemplateConfig(name, "before") },
				configAfter:  func(name string) string { return stACLTemplateConfig(name, "after") },
			},
		},
		{
			typ:           "truenas_zvol",
			createMethod:  "pool.dataset.create",
			fetch:         defaultFetch("pool.dataset.create", "truenas_zvol.t"),
			minimalConfig: func(name string) string { return stZvolConfig(name, "") },
			// Space accounting moves with pool free space between reads.
			volatile: []string{"available", "used", "referenced", "written",
				"usedbychildren", "usedbydataset", "usedbysnapshots",
				"usedbyrefreservation", "logicalused", "logicalreferenced"},
			perturb: &perturbSpec{
				attr: "comments",
				// Like datasets, the server nests comments under user_properties.
				changedServerPaths: []string{"user_properties.comments"},
				configBefore:       func(name string) string { return stZvolConfig(name, "before") },
				configAfter:        func(name string) string { return stZvolConfig(name, "after") },
			},
		},
		{
			typ:           "truenas_webshare",
			createMethod:  "sharing.webshare.create",
			fetch:         defaultFetch("sharing.webshare.create", "truenas_webshare.t"),
			minimalConfig: func(name string) string { return stWebshareConfig(name, "") },
			minMajor:      26,
			perturb: &perturbSpec{
				attr:         "enabled",
				configBefore: func(name string) string { return stWebshareConfig(name, "true") },
				configAfter:  func(name string) string { return stWebshareConfig(name, "false") },
			},
			// dataset/relative_path are computed fields with a documented
			// create-time null quirk: null on the read right after create, then
			// resolved on a later read (see the schema descriptions). They
			// transition null->resolved between the two guard reads regardless
			// of the perturb, so they are volatile here, not drift.
			volatile: []string{"dataset", "relative_path"},
		},
		{
			typ:           "truenas_keychain_ssh_keypair",
			createMethod:  "keychaincredential.create",
			fetch:         defaultFetch("keychaincredential.create", "truenas_keychain_ssh_keypair.t"),
			minimalConfig: func(name string) string { return stSSHKeypairConfig(name, "") },
			perturb: &perturbSpec{
				attr:         "name",
				configBefore: func(name string) string { return stSSHKeypairConfig(name, "") },
				configAfter:  func(name string) string { return stSSHKeypairConfig(name, "-b") },
			},
		},
		{
			typ:           "truenas_kerberos_keytab",
			createMethod:  "kerberos.keytab.create",
			fetch:         defaultFetch("kerberos.keytab.create", "truenas_kerberos_keytab.t"),
			minimalConfig: func(name string) string { return stKeytabConfig(name, "") },
			perturb: &perturbSpec{
				attr:         "name",
				configBefore: func(name string) string { return stKeytabConfig(name, "") },
				configAfter:  func(name string) string { return stKeytabConfig(name, "-b") },
			},
		},
		{
			typ:           "truenas_iscsi_portal",
			createMethod:  "iscsi.portal.create",
			fetch:         defaultFetch("iscsi.portal.create", "truenas_iscsi_portal.t"),
			minimalConfig: func(name string) string { return stISCSIPortalConfig(name, "") },
			perturb: &perturbSpec{
				attr:         "comment",
				configBefore: func(name string) string { return stISCSIPortalConfig(name, "before") },
				configAfter:  func(name string) string { return stISCSIPortalConfig(name, "after") },
			},
		},
		{
			typ:           "truenas_nvmet_subsys",
			createMethod:  "nvmet.subsys.create",
			fetch:         defaultFetch("nvmet.subsys.create", "truenas_nvmet_subsys.t"),
			minimalConfig: func(name string) string { return stNVMetSubsysConfig(name, "") },
			perturb: &perturbSpec{
				attr:         "allow_any_host",
				configBefore: func(name string) string { return stNVMetSubsysConfig(name, "false") },
				configAfter:  func(name string) string { return stNVMetSubsysConfig(name, "true") },
			},
		},
		{
			typ:           "truenas_nvmet_port",
			createMethod:  "nvmet.port.create",
			fetch:         defaultFetch("nvmet.port.create", "truenas_nvmet_port.t"),
			minimalConfig: func(name string) string { return stNVMetPortConfig(name, "") },
			perturb: &perturbSpec{
				attr:         "max_queue_size",
				configBefore: func(name string) string { return stNVMetPortConfig(name, "128") },
				configAfter:  func(name string) string { return stNVMetPortConfig(name, "256") },
			},
		},
		{
			typ:           "truenas_reporting_exporter",
			createMethod:  "reporting.exporters.create",
			fetch:         defaultFetch("reporting.exporters.create", "truenas_reporting_exporter.t"),
			minimalConfig: func(name string) string { return stReportingExporterConfig(name, 2003) },
			perturb: &perturbSpec{
				attr:               "attributes.destination_port",
				changedServerPaths: []string{"attributes.destination_port"},
				configBefore:       func(name string) string { return stReportingExporterConfig(name, 2003) },
				configAfter:        func(name string) string { return stReportingExporterConfig(name, 2004) },
			},
		},
		{
			typ:           "truenas_iscsi_target",
			createMethod:  "iscsi.target.create",
			fetch:         defaultFetch("iscsi.target.create", "truenas_iscsi_target.t"),
			minimalConfig: func(name string) string { return stISCSITargetConfig(name, "") },
			perturb: &perturbSpec{
				attr:         "alias",
				configBefore: func(name string) string { return stISCSITargetConfig(name, "before") },
				configAfter:  func(name string) string { return stISCSITargetConfig(name, "after") },
			},
		},
		{
			typ:           "truenas_iscsi_extent",
			createMethod:  "iscsi.extent.create",
			fetch:         defaultFetch("iscsi.extent.create", "truenas_iscsi_extent.t"),
			minimalConfig: func(name string) string { return stISCSIExtentConfig(name, "") },
			perturb: &perturbSpec{
				attr:         "comment",
				configBefore: func(name string) string { return stISCSIExtentConfig(name, "before") },
				configAfter:  func(name string) string { return stISCSIExtentConfig(name, "after") },
			},
		},
		{
			// Join resource, but keyed by its own numeric id, so defaultFetch applies.
			typ:           "truenas_iscsi_targetextent",
			createMethod:  "iscsi.targetextent.create",
			fetch:         defaultFetch("iscsi.targetextent.create", "truenas_iscsi_targetextent.t"),
			minimalConfig: func(name string) string { return stISCSITargetExtentConfig(name, "") },
			perturb: &perturbSpec{
				attr:         "lunid",
				configBefore: func(name string) string { return stISCSITargetExtentConfig(name, "1") },
				configAfter:  func(name string) string { return stISCSITargetExtentConfig(name, "2") },
			},
		},
		{
			typ:           "truenas_nvmet_namespace",
			createMethod:  "nvmet.namespace.create",
			fetch:         defaultFetch("nvmet.namespace.create", "truenas_nvmet_namespace.t"),
			minimalConfig: func(name string) string { return stNVMetNamespaceConfig(name, "") },
			perturb: &perturbSpec{
				attr:         "enabled",
				configBefore: func(name string) string { return stNVMetNamespaceConfig(name, "true") },
				configAfter:  func(name string) string { return stNVMetNamespaceConfig(name, "false") },
			},
		},
		{
			// Join resource keyed by its own numeric id. Both attributes are
			// RequiresReplace, so there is no in-place update to perturb: wired for
			// Guard 2 only (create from required attributes).
			typ:           "truenas_nvmet_port_subsys",
			createMethod:  "nvmet.port_subsys.create",
			fetch:         defaultFetch("nvmet.port_subsys.create", "truenas_nvmet_port_subsys.t"),
			minimalConfig: func(name string) string { return stNVMetPortSubsysConfig(name) },
		},
		{
			typ:           "truenas_periodic_snapshot_task",
			createMethod:  "pool.snapshottask.create",
			fetch:         defaultFetch("pool.snapshottask.create", "truenas_periodic_snapshot_task.t"),
			minimalConfig: func(name string) string { return stPeriodicSnapshotConfig(name, 2) },
			perturb: &perturbSpec{
				attr:         "lifetime_value",
				configBefore: func(name string) string { return stPeriodicSnapshotConfig(name, 2) },
				configAfter:  func(name string) string { return stPeriodicSnapshotConfig(name, 3) },
			},
		},
		{
			typ:           "truenas_rsync_task",
			createMethod:  "rsynctask.create",
			fetch:         defaultFetch("rsynctask.create", "truenas_rsync_task.t"),
			minimalConfig: func(name string) string { return stRsyncTaskConfig(name, "") },
			perturb: &perturbSpec{
				attr:         "desc",
				configBefore: func(name string) string { return stRsyncTaskConfig(name, "before") },
				configAfter:  func(name string) string { return stRsyncTaskConfig(name, "after") },
			},
		},
		{
			// Keyed by share_name, not a numeric id: custom fetch via
			// sharing.smb.getacl (the tf id equals share_name).
			typ:           "truenas_smb_share_acl",
			createMethod:  "sharing.smb.setacl",
			fetch:         smbShareACLFetch,
			minimalConfig: func(name string) string { return stSMBShareACLConfig(name, "READ") },
			perturb: &perturbSpec{
				attr: "share_acl",
				// The whole entry list is the field under change; the server
				// resolves each principal into extra fields beneath it.
				changedServerPaths: []string{"share_acl"},
				configBefore:       func(name string) string { return stSMBShareACLConfig(name, "READ") },
				configAfter:        func(name string) string { return stSMBShareACLConfig(name, "CHANGE") },
			},
		},
	}
}

// stGroupConfig: smb is omitted when empty (server default applies).
func stGroupConfig(name, smb string) string {
	return fmt.Sprintf(`
resource "truenas_group" "t" {
  name = %q
%s}
`, name, stOptionalAttr("smb", smb))
}

// stAPIKeyConfig owns the key by the account the tests log in as.
func stAPIKeyConfig(name, nameSuffix string) string {
	user := os.Getenv("TRUENAS_USERNAME")
	if user == "" {
		user = "truenas_admin"
	}
	return fmt.Sprintf(`
resource "truenas_api_key" "t" {
  name     = %q
  username = %q
}
`, name+nameSuffix, user)
}

func stAlertServiceConfig(name, level string) string {
	return fmt.Sprintf(`
resource "truenas_alert_service" "t" {
  name  = %q
  level = %q
  attributes = jsonencode({
    type  = "Mail"
    email = "tfst@example.com"
  })
}
`, name, level)
}

// stPrivilegeConfig: a throwaway group fixture backs local_groups; the
// privilege name varies by nameSuffix for the rename perturb.
func stPrivilegeConfig(name, nameSuffix string) string {
	return fmt.Sprintf(`
resource "truenas_group" "fixture" {
  name = %q
  smb  = false
}

resource "truenas_privilege" "t" {
  name         = %q
  local_groups = [truenas_group.fixture.gid]
  web_shell    = false
}
`, name+"g", name+nameSuffix)
}

// stISCSIAuthConfig: tag 998 matches the resource acceptance test; secret is
// write-only (12-16 chars).
func stISCSIAuthConfig(name, userSuffix string) string {
	return fmt.Sprintf(`
resource "truenas_iscsi_auth" "t" {
  tag    = 998
  user   = %q
  secret = "tfst-secret-1ch"
}
`, name+userSuffix)
}

func stISCSIInitiatorConfig(_ string, comment string) string {
	return fmt.Sprintf(`
resource "truenas_iscsi_initiator" "t" {
%s}
`, stOptionalAttr("comment", quoteIfSet(comment)))
}

// stKerberosRealmConfig: realm is upper-cased; adminServer is an HCL list
// literal, omitted when empty.
func stKerberosRealmConfig(name, adminServer string) string {
	return fmt.Sprintf(`
resource "truenas_kerberos_realm" "t" {
  realm = %q
%s}
`, strings.ToUpper(name)+".LAN", stOptionalAttr("admin_server", adminServer))
}

// stACLTemplateConfig: acl is in the compact sorted-key form the provider
// reads back, so it round-trips byte-identical.
func stACLTemplateConfig(name, comment string) string {
	acl := `[{"flags":{"BASIC":"INHERIT"},"perms":{"BASIC":"FULL_CONTROL"},"tag":"owner@","type":"ALLOW"},` +
		`{"flags":{"BASIC":"INHERIT"},"perms":{"BASIC":"READ"},"tag":"everyone@","type":"ALLOW"}]`
	return fmt.Sprintf(`
resource "truenas_acl_template" "t" {
  name    = %q
  acltype = "NFS4"
  acl     = %q
%s}
`, name, acl, stOptionalAttr("comment", quoteIfSet(comment)))
}

// stCloudsyncCredsConfig: STORJ_IX credentials are stored without contacting
// the provider. nameSuffix lets the perturb rename in place.
func stCloudsyncCredsConfig(name, nameSuffix string) string {
	return fmt.Sprintf(`
resource "truenas_cloudsync_credentials" "t" {
  name = %q
  provider_config = jsonencode({
    type              = "STORJ_IX"
    access_key_id     = "tfst"
    secret_access_key = "tfst"
  })
}
`, name+nameSuffix)
}

// stFSPermsConfig is a dataset plus filesystem_permissions on its mountpoint;
// mode is omitted when empty.
func stFSPermsConfig(name, mode string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name = %q
}

resource "truenas_filesystem_permissions" "t" {
  path = truenas_dataset.fixture.mountpoint
%s}
`, acctest.TestPool()+"/"+name, stOptionalAttr("mode", quoteIfSet(mode)))
}

// fsStatFetch reads filesystem.stat for the resource's id (its path), since
// the resource has no <ns>.get_instance.
func fsStatFetch(addr string) func(*testing.T, *terraform.State) map[string]any {
	return func(t *testing.T, s *terraform.State) map[string]any {
		t.Helper()
		rs, ok := s.RootModule().Resources[addr]
		if !ok {
			t.Fatalf("server-truth: %s not in state", addr)
		}
		raw, err := acctest.Client().Call(context.Background(), "filesystem.stat", rs.Primary.ID)
		if err != nil {
			t.Fatalf("server-truth: filesystem.stat(%s): %v", rs.Primary.ID, err)
		}
		var inst map[string]any
		if err := json.Unmarshal(raw, &inst); err != nil {
			t.Fatalf("server-truth: decode filesystem.stat(%s): %v", rs.Primary.ID, err)
		}
		return inst
	}
}

// stNFSShareConfig is a dataset plus a required-only NFS share on its
// mountpoint; comment is omitted when empty.
func stNFSShareConfig(name, comment string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name = %q
}

resource "truenas_nfs_share" "t" {
  path = truenas_dataset.fixture.mountpoint
%s}
`, acctest.TestPool()+"/"+name, stOptionalAttr("comment", quoteIfSet(comment)))
}

// stSMBShareConfig is a dataset plus a required-only SMB share on its
// mountpoint; comment is omitted when empty.
func stSMBShareConfig(name, comment string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name = %q
}

resource "truenas_smb_share" "t" {
  path = truenas_dataset.fixture.mountpoint
  name = %q
%s}
`, acctest.TestPool()+"/"+name, name, stOptionalAttr("comment", quoteIfSet(comment)))
}

// stDatasetConfig is a bare dataset; comments is omitted when empty.
func stDatasetConfig(name, comments string) string {
	extra := ""
	if comments != "" {
		extra = fmt.Sprintf("  comments = %q\n", comments)
	}
	return fmt.Sprintf(`
resource "truenas_dataset" "t" {
  name = %q
%s}
`, acctest.TestPool()+"/"+name, extra)
}

// stDatasetCompressionConfig sets compression and leaves comments UNSET, so the
// Guard-1 perturb changes an unrelated field and would catch a regression that
// writes comments on update (#62).
func stDatasetCompressionConfig(name, compression string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "t" {
  name        = %q
  compression = %q
}
`, acctest.TestPool()+"/"+name, compression)
}

func stCertificateConfig(name, nameSuffix string) string {
	return fmt.Sprintf(`
resource "truenas_certificate" "t" {
  name        = %q
  create_type = "CERTIFICATE_CREATE_CSR"
  key_type    = "RSA"
  key_length  = 2048
  common      = "tf-st.example.com"
  san         = ["tf-st.example.com"]
}
`, name+nameSuffix)
}

func stUserConfig(name, fullName string) string {
	return fmt.Sprintf(`
resource "truenas_user" "t" {
  username          = %q
  full_name         = %q
  password          = "Tf-Acc-Test-Passw0rd!"
  password_disabled = false
  home              = "/var/empty"
  shell             = "/usr/bin/bash"
  smb               = false
  group_create      = true
}
`, name, fullName)
}

func stReplicationConfig(name, enabled string) string {
	pool := acctest.TestPool()
	return fmt.Sprintf(`
resource "truenas_dataset" "src" {
  name = "%[1]s/%[2]s-src"
}

resource "truenas_dataset" "dst" {
  name = "%[1]s/%[2]s-dst"
}

resource "truenas_replication_task" "t" {
  name             = %[2]q
  direction        = "PUSH"
  transport        = "LOCAL"
  source_datasets  = [truenas_dataset.src.name]
  target_dataset   = truenas_dataset.dst.name
  recursive        = false
  auto             = false
  retention_policy = "SOURCE"

  also_include_naming_schema = ["auto-%%Y-%%m-%%d_%%H-%%M"]
%[3]s}
`, pool, name, stOptionalAttr("enabled", enabled))
}

// stSSHConnectionConfig builds the loopback SSH connection fixture: a
// throwaway user (own home dataset, key authorized), an ed25519 keypair, and
// the connection to the test box itself. The host key comes from a live scan,
// so it is only fetched when TF_ACC is set; without a box the config is built
// with a marker host key and is for construction checks only.
func stSSHConnectionConfig(name, connectTimeout string) string {
	host, hostKey := "unscanned.invalid", "unscanned"
	if os.Getenv("TF_ACC") != "" {
		host = acctest.EndpointHost()
		raw, err := acctest.Client().Call(context.Background(), "keychaincredential.remote_ssh_host_key_scan",
			map[string]any{"host": host, "port": 22})
		if err != nil {
			panic("server-truth: remote_ssh_host_key_scan: " + err.Error())
		}
		if err := json.Unmarshal(raw, &hostKey); err != nil {
			panic("server-truth: parse host key scan: " + err.Error())
		}
	}
	privPEM, _ := stEd25519KeyPair(name)
	// Required-only when connectTimeout is empty: port, username and
	// connect_timeout are left unset so the schema defaults (22 / root / 10)
	// apply.
	return fmt.Sprintf(`
resource "truenas_keychain_ssh_keypair" "kp" {
  name        = %[1]q
  private_key = %[2]q
}

resource "truenas_keychain_ssh_connection" "t" {
  name            = %[3]q
  host            = %[4]q
  private_key_id  = truenas_keychain_ssh_keypair.kp.id
  remote_host_key = %[5]q
%[6]s}
`, name+"-kp", privPEM, name, host, hostKey, stOptionalAttr("connect_timeout", connectTimeout))
}

// stEd25519KeyPair returns an OpenSSH private key PEM and the matching
// authorized_keys line (newline trimmed, as TrueNAS stores it).
func stEd25519KeyPair(comment string) (privPEM, pubLine string) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic("server-truth: ed25519: " + err.Error())
	}
	block, err := ssh.MarshalPrivateKey(priv, comment)
	if err != nil {
		panic("server-truth: marshal key: " + err.Error())
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		panic("server-truth: ssh pubkey: " + err.Error())
	}
	return string(pem.EncodeToMemory(block)), strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub)))
}

// stOptionalAttr renders "  attr = value" (value already HCL) or nothing when
// value is empty, so required-only configs leave the attribute unset.
func stOptionalAttr(attr, hcl string) string {
	if hcl == "" {
		return ""
	}
	return fmt.Sprintf("  %s = %s\n", attr, hcl)
}

func stCronjobConfig(_ string, description string) string {
	return fmt.Sprintf(`
resource "truenas_cronjob" "t" {
  command = "/usr/bin/true"
  user    = "root"
%s}
`, stOptionalAttr("description", quoteIfSet(description)))
}

func quoteIfSet(v string) string {
	if v == "" {
		return ""
	}
	return strconv.Quote(v)
}

func stInitScriptConfig(_ string, comment string) string {
	return fmt.Sprintf(`
resource "truenas_init_shutdown_script" "t" {
  type    = "COMMAND"
  command = "/usr/bin/true"
  when    = "POSTINIT"
%s}
`, stOptionalAttr("comment", quoteIfSet(comment)))
}

// stNTPConfig: force is write-only (skips the reachability check) and is
// needed for a non-resolvable address; prefer is omitted when empty.
func stNTPConfig(name, prefer string) string {
	return fmt.Sprintf(`
resource "truenas_ntp_server" "t" {
  address = "%s.example.com"
  force   = true
%s}
`, name, stOptionalAttr("prefer", prefer))
}

func stStaticRouteConfig(_ string, description string) string {
	return fmt.Sprintf(`
resource "truenas_static_route" "t" {
  destination = "198.51.100.0/24"
  gateway     = "192.168.1.254"
%s}
`, stOptionalAttr("description", quoteIfSet(description)))
}

// stNVMetHostConfig derives a valid uuid-form NQN from name and variant so
// the before/after configs differ only in hostnqn.
func stNVMetHostConfig(name, variant string) string {
	sum := sha256.Sum256([]byte(name + "/" + variant))
	b := sum[:16]
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	nqn := fmt.Sprintf("nqn.2014-08.org.nvmexpress:uuid:%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
	return fmt.Sprintf(`
resource "truenas_nvmet_host" "t" {
  hostnqn = %q
}
`, nqn)
}

// stZvolConfig: required-only zvol (name + volsize) under the test pool.
func stZvolConfig(name, comments string) string {
	return fmt.Sprintf(`
resource "truenas_zvol" "t" {
  name    = %q
  volsize = 67108864
%s}
`, acctest.TestPool()+"/"+name, stOptionalAttr("comments", quoteIfSet(comments)))
}

// stWebshareConfig: a dataset fixture plus a required-only share; enabled is
// omitted when empty so its schema default is what Guard 2 verifies.
func stWebshareConfig(name, enabled string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name = %q
}

resource "truenas_webshare" "t" {
  path = truenas_dataset.fixture.mountpoint
  name = %q
%s}
`, acctest.TestPool()+"/"+name, name, stOptionalAttr("enabled", enabled))
}

func stSSHKeypairConfig(name, nameSuffix string) string {
	return fmt.Sprintf(`
resource "truenas_keychain_ssh_keypair" "t" {
  name     = %q
  generate = true
}
`, name+nameSuffix)
}

// stKeytabB64 returns TRUENAS_DS_KEYTAB_B64 when set, else a syntactically
// valid MIT keytab (version 0x0502, one aes256 entry) built in-process.
func stKeytabB64() string {
	if v := os.Getenv("TRUENAS_DS_KEYTAB_B64"); v != "" {
		return v
	}
	var e []byte
	be16 := func(v uint16) { e = binary.BigEndian.AppendUint16(e, v) }
	be32 := func(v uint32) { e = binary.BigEndian.AppendUint32(e, v) }
	str := func(v string) { be16(uint16(len(v))); e = append(e, v...) }
	be16(2) // component count (realm excluded in the v2 format)
	str("TFTEST.LAN")
	str("host")
	str("tfst.tftest.lan")
	be32(1) // name type: KRB5_NT_PRINCIPAL
	be32(1700000000)
	e = append(e, 1) // kvno
	be16(18)         // aes256-cts-hmac-sha1-96
	be16(32)
	e = append(e, make([]byte, 32)...)
	out := []byte{0x05, 0x02}
	out = binary.BigEndian.AppendUint32(out, uint32(len(e)))
	out = append(out, e...)
	return base64.StdEncoding.EncodeToString(out)
}

func stKeytabConfig(name, nameSuffix string) string {
	return fmt.Sprintf(`
resource "truenas_kerberos_keytab" "t" {
  name = %q
  file = %q
}
`, name+nameSuffix, stKeytabB64())
}

func stISCSIPortalConfig(_ string, comment string) string {
	return fmt.Sprintf(`
resource "truenas_iscsi_portal" "t" {
%s  listen = [
    {
      ip = %q
    }
  ]
}
`, stOptionalAttr("comment", quoteIfSet(comment)), acctest.EndpointHost())
}

func stNVMetSubsysConfig(name, allowAnyHost string) string {
	return fmt.Sprintf(`
resource "truenas_nvmet_subsys" "t" {
  name = %q
%s}
`, name, stOptionalAttr("allow_any_host", allowAnyHost))
}

// stNVMetPortConfig pins addr_trsvcid (no schema default) to a high port so a
// create cannot collide with the box's real NVMe-oF listener on 4420.
func stNVMetPortConfig(_ string, maxQueueSize string) string {
	return fmt.Sprintf(`
resource "truenas_nvmet_port" "t" {
  addr_trtype  = "TCP"
  addr_traddr  = %q
  addr_trsvcid = 14423
%s}
`, acctest.EndpointHost(), stOptionalAttr("max_queue_size", maxQueueSize))
}

func stReportingExporterConfig(name string, port int) string {
	return fmt.Sprintf(`
resource "truenas_reporting_exporter" "t" {
  name    = %q
  enabled = false
  attributes = {
    destination_ip   = "192.0.2.50"
    destination_port = %d
    namespace        = "tfst"
  }
}
`, name, port)
}

func stISCSITargetConfig(name, alias string) string {
	return fmt.Sprintf(`
resource "truenas_iscsi_target" "t" {
  name = %q
%s}
`, name, stOptionalAttr("alias", quoteIfSet(alias)))
}

// stISCSIExtentConfig: a zvol fixture backs a required-only DISK extent.
func stISCSIExtentConfig(name, comment string) string {
	return fmt.Sprintf(`
resource "truenas_zvol" "fixture" {
  name    = %q
  volsize = 67108864
}

resource "truenas_iscsi_extent" "t" {
  name = %q
  type = "DISK"
  disk = "zvol/${truenas_zvol.fixture.name}"
%s}
`, acctest.TestPool()+"/"+name, name, stOptionalAttr("comment", quoteIfSet(comment)))
}

// stISCSITargetExtentConfig: target + zvol + extent fixtures; lunid is omitted
// when empty (server auto-assigns).
func stISCSITargetExtentConfig(name, lunid string) string {
	return fmt.Sprintf(`
resource "truenas_iscsi_target" "fixture" {
  name = %[2]q
}

resource "truenas_zvol" "fixture" {
  name    = %[1]q
  volsize = 67108864
}

resource "truenas_iscsi_extent" "fixture" {
  name = %[2]q
  type = "DISK"
  disk = "zvol/${truenas_zvol.fixture.name}"
}

resource "truenas_iscsi_targetextent" "t" {
  target = truenas_iscsi_target.fixture.id
  extent = truenas_iscsi_extent.fixture.id
%[3]s}
`, acctest.TestPool()+"/"+name, name, stOptionalAttr("lunid", lunid))
}

// stNVMetNamespaceConfig: subsys + zvol fixtures; device_type is set (no schema
// default) as the acceptance test does; enabled is omitted when empty.
func stNVMetNamespaceConfig(name, enabled string) string {
	return fmt.Sprintf(`
resource "truenas_nvmet_subsys" "fixture" {
  name = %[2]q
}

resource "truenas_zvol" "fixture" {
  name    = %[1]q
  volsize = 67108864
}

resource "truenas_nvmet_namespace" "t" {
  subsys_id   = truenas_nvmet_subsys.fixture.id
  device_path = "zvol/${truenas_zvol.fixture.name}"
  device_type = "ZVOL"
%[3]s}
`, acctest.TestPool()+"/"+name, name, stOptionalAttr("enabled", enabled))
}

// stNVMetPortSubsysConfig: port (disabled, high port) + subsys fixtures.
func stNVMetPortSubsysConfig(name string) string {
	return fmt.Sprintf(`
resource "truenas_nvmet_subsys" "fixture" {
  name = %q
}

resource "truenas_nvmet_port" "fixture" {
  addr_trtype  = "TCP"
  addr_traddr  = %q
  addr_trsvcid = 14425
  enabled      = false
}

resource "truenas_nvmet_port_subsys" "t" {
  port_id   = truenas_nvmet_port.fixture.id
  subsys_id = truenas_nvmet_subsys.fixture.id
}
`, name, acctest.EndpointHost())
}

// stPeriodicSnapshotConfig: dataset fixture plus a disabled task so no
// snapshots are taken during the audit.
func stPeriodicSnapshotConfig(name string, lifetimeValue int) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name = %q
}

resource "truenas_periodic_snapshot_task" "t" {
  dataset        = truenas_dataset.fixture.name
  lifetime_value = %d
  lifetime_unit  = "WEEK"
  naming_schema  = "tfst-%%Y%%m%%d-%%H%%M"
  enabled        = false
  schedule = {
    minute = "0"
    hour   = "0"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}
`, acctest.TestPool()+"/"+name, lifetimeValue)
}

// stRsyncTaskConfig: MODULE mode against a documentation-range host, disabled
// and with validate_rpath off so nothing contacts the remote.
func stRsyncTaskConfig(name, desc string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name = %q
}

resource "truenas_rsync_task" "t" {
  path           = truenas_dataset.fixture.mountpoint
  user           = "root"
  remotehost     = "192.0.2.10"
  remotemodule   = "tfst"
  enabled        = false
  validate_rpath = false
  schedule = {
    minute = "0"
    hour   = "3"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
%s}
`, acctest.TestPool()+"/"+name, stOptionalAttr("desc", quoteIfSet(desc)))
}

// stSMBShareACLConfig: dataset + SMB share fixtures and a single-entry share
// ACL (everyone SID) at the given permission level.
func stSMBShareACLConfig(name, perm string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name = %q
}

resource "truenas_smb_share" "fixture" {
  path = truenas_dataset.fixture.mountpoint
  name = %q
}

resource "truenas_smb_share_acl" "t" {
  share_name = truenas_smb_share.fixture.name
  share_acl = [
    { ae_perm = %q, ae_type = "ALLOWED", ae_who_sid = "S-1-1-0" },
  ]
}
`, acctest.TestPool()+"/"+name, name, perm)
}

// smbShareACLFetch reads the share ACL by share name (the tf id), since the
// resource has no <ns>.get_instance.
func smbShareACLFetch(t *testing.T, s *terraform.State) map[string]any {
	t.Helper()
	rs, ok := s.RootModule().Resources["truenas_smb_share_acl.t"]
	if !ok {
		t.Fatalf("server-truth: truenas_smb_share_acl.t not in state")
	}
	raw, err := acctest.Client().Call(context.Background(), "sharing.smb.getacl",
		map[string]any{"share_name": rs.Primary.ID})
	if err != nil {
		t.Fatalf("server-truth: sharing.smb.getacl(%s): %v", rs.Primary.ID, err)
	}
	var inst map[string]any
	if err := json.Unmarshal(raw, &inst); err != nil {
		t.Fatalf("server-truth: decode sharing.smb.getacl(%s): %v", rs.Primary.ID, err)
	}
	return inst
}

func TestServerTruthRegistry_WellFormed(t *testing.T) {
	known := map[string]bool{}
	for _, typ := range allProviderResourceTypes(t) {
		known[typ] = true
	}
	seeds := serverTruthResources()
	if len(seeds) != 38 {
		t.Errorf("seed set has %d entries, want 38", len(seeds))
	}
	for _, r := range seeds {
		if !known[r.typ] {
			t.Errorf("%s: not a provider resource type", r.typ)
		}
		if r.createMethod == "" {
			t.Errorf("%s: empty createMethod", r.typ)
		}
		if r.fetch == nil {
			t.Errorf("%s: nil fetch", r.typ)
		}
		if r.minimalConfig == nil {
			t.Errorf("%s: nil minimalConfig", r.typ)
			continue
		}
		cfg := r.minimalConfig("tfst-probe")
		if strings.TrimSpace(cfg) == "" || !strings.Contains(cfg, `resource "`+r.typ+`" "t"`) {
			t.Errorf("%s: minimalConfig lacks resource %q labeled t", r.typ, r.typ)
		}
		if r.perturb != nil {
			b, a := r.perturb.configBefore("tfst-probe"), r.perturb.configAfter("tfst-probe")
			if b == a || !strings.Contains(b, r.typ) || !strings.Contains(a, r.typ) {
				t.Errorf("%s: perturb configs malformed", r.typ)
			}
		}
	}
}

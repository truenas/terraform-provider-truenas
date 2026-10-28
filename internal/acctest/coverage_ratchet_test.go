// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

// coverageRatchet is the explicit, shrinking allowlist of writable attributes
// not yet exercised by an acceptance config — accepted debt that
// TestAttributeCoverage_Audit tolerates for now. It only ever gets SHORTER:
// when you make an acceptance test set an attribute, delete its entry here.
// Never add a newly-introduced attribute here to dodge the check — new
// resource attributes must be exercised, not ratcheted.
//
// Generated from the current gap set (414 entries). Keyed by "<type_name>.<attr>".
var coverageRatchet = map[string]bool{
	"truenas_truecommand_config.api_key_wo":         true, // write-only; truecommand create needs a real portal key (external)
	"truenas_truecommand_config.api_key_wo_version": true, // paired with api_key_wo
	// write-only "secrets-out-of-state" overlays (#36/#37): the _wo value is
	// absent from state and the _version is its paired trigger; exercised during
	// live verification, not set in the committed create config.
	"truenas_acme_dns_authenticator.attributes_secrets_wo":         true, // external: needs a live ACME account + DNS provider credentials
	"truenas_acme_dns_authenticator.attributes_secrets_wo_version": true, // external: needs a live ACME account + DNS provider credentials
	"truenas_app.custom_app":                                       true, // external: needs the app catalog + a real app release
	"truenas_app.train":                                            true, // external: needs the app catalog + a real app release
	"truenas_app.version":                                          true, // external: needs the app catalog + a real app release
	"truenas_boot_environment.activated":                           true, // unsafe: boot-environment changes risk the boot configuration
	"truenas_cloud_backup.absolute_paths":                          true, // external: needs a live cloud/S3 remote to create a backup task
	"truenas_cloud_backup.cache_path":                              true, // external: needs a live cloud/S3 remote to create a backup task
	"truenas_cloud_backup.password_wo":                             true, // no live S3 bucket fixture; cloud_backup acceptance is a permanent skip
	"truenas_cloud_backup.password_wo_version":                     true, // paired with password_wo above
	"truenas_cloud_backup.description":                             true, // external: needs a live cloud/S3 remote to create a backup task
	"truenas_cloud_backup.exclude":                                 true, // external: needs a live cloud/S3 remote to create a backup task
	"truenas_cloud_backup.include":                                 true, // external: needs a live cloud/S3 remote to create a backup task
	"truenas_cloud_backup.post_script":                             true, // external: needs a live cloud/S3 remote to create a backup task
	"truenas_cloud_backup.pre_script":                              true, // external: needs a live cloud/S3 remote to create a backup task
	"truenas_cloud_backup.rate_limit":                              true, // external: needs a live cloud/S3 remote to create a backup task
	"truenas_cloud_backup.snapshot":                                true, // external: needs a live cloud/S3 remote to create a backup task
	"truenas_cloud_backup.transfer_setting":                        true, // external: needs a live cloud/S3 remote to create a backup task
	"truenas_cloudsync_task.bwlimit":                               true, // external: needs a live cloud remote to create a sync task
	"truenas_cloudsync_task.create_empty_src_dirs":                 true, // external: needs a live cloud remote to create a sync task
	"truenas_cloudsync_task.encryption":                            true, // external: needs a live cloud remote to create a sync task
	"truenas_cloudsync_task.encryption_password":                   true, // external: needs a live cloud remote to create a sync task
	"truenas_cloudsync_task.encryption_salt":                       true, // external: needs a live cloud remote to create a sync task
	"truenas_cloudsync_task.exclude":                               true, // external: needs a live cloud remote to create a sync task
	"truenas_cloudsync_task.filename_encryption":                   true, // external: needs a live cloud remote to create a sync task
	"truenas_cloudsync_task.follow_symlinks":                       true, // external: needs a live cloud remote to create a sync task
	"truenas_cloudsync_task.include":                               true, // external: needs a live cloud remote to create a sync task
	"truenas_cloudsync_task.post_script":                           true, // external: needs a live cloud remote to create a sync task
	"truenas_cloudsync_task.pre_script":                            true, // external: needs a live cloud remote to create a sync task
	"truenas_cloudsync_task.snapshot":                              true, // external: needs a live cloud remote to create a sync task
	"truenas_cloudsync_task.transfers":                             true, // external: needs a live cloud remote to create a sync task
	"truenas_container.capabilities_policy":                        true, // external: needs the container platform with a real pullable image
	"truenas_container.capabilities_state":                         true, // external: needs the container platform with a real pullable image
	"truenas_container.cpuset":                                     true, // external: needs the container platform with a real pullable image
	"truenas_container.idmap":                                      true, // external: needs the container platform with a real pullable image
	"truenas_container.init":                                       true, // external: needs the container platform with a real pullable image
	"truenas_container.initdir":                                    true, // external: needs the container platform with a real pullable image
	"truenas_container.initenv":                                    true, // external: needs the container platform with a real pullable image
	"truenas_container.initgroup":                                  true, // external: needs the container platform with a real pullable image
	"truenas_container.inituser":                                   true, // external: needs the container platform with a real pullable image
	"truenas_container.shutdown_timeout":                           true, // external: needs the container platform with a real pullable image
	"truenas_container.time":                                       true, // external: needs the container platform with a real pullable image
	"truenas_directoryservices.enable_account_cache":               true, // lab AD/LDAP/IPA VMs (pve 210/211/212) exist; drain by joining a disposable box to VM 210 (AD) — slow-boot domain-join fixture, follow-up
	"truenas_directoryservices.enable_dns_updates":                 true, // lab AD/LDAP/IPA VMs (pve 210/211/212) exist; drain by joining a disposable box to VM 210 (AD) — slow-boot domain-join fixture, follow-up
	"truenas_directoryservices.kerberos_realm":                     true, // lab AD/LDAP/IPA VMs (pve 210/211/212) exist; drain by joining a disposable box to VM 210 (AD) — slow-boot domain-join fixture, follow-up
	"truenas_docker_config.address_pools":                          true, // external: configures the app/docker platform; needs a dedicated apps pool
	"truenas_docker_config.cidr_v6":                                true, // external: configures the app/docker platform; needs a dedicated apps pool
	"truenas_docker_config.nvidia":                                 true, // external: configures the app/docker platform; needs a dedicated apps pool
	"truenas_docker_config.pool":                                   true, // external: configures the app/docker platform; needs a dedicated apps pool
	"truenas_docker_config.registry_mirrors":                       true, // external: configures the app/docker platform; needs a dedicated apps pool
	"truenas_failover_config.disabled":                             true, // unsafe: requires an HA controller pair
	"truenas_failover_config.master":                               true, // unsafe: requires an HA controller pair
	"truenas_ipmi_lan.password":                                    true, // write-only BMC password; rotating it risks BMC admin lockout on the shared BMC
	"truenas_lxc_config.preferred_pool":                            true, // changing it migrates the LXC pool; test skips when it is set
	"truenas_network_interface.bridge_members":                     true, // needs a truly-free spare NIC (a bridge/LAG member must have no carrier); exercise on a VM with added virtual NICs (TRUENAS_FREE_NIC)
	"truenas_network_interface.lacpdu_rate":                        true, // needs a truly-free spare NIC (a bridge/LAG member must have no carrier); exercise on a VM with added virtual NICs (TRUENAS_FREE_NIC)
	"truenas_network_interface.lag_ports":                          true, // needs a truly-free spare NIC (a bridge/LAG member must have no carrier); exercise on a VM with added virtual NICs (TRUENAS_FREE_NIC)
	"truenas_network_interface.lag_protocol":                       true, // needs a truly-free spare NIC (a bridge/LAG member must have no carrier); exercise on a VM with added virtual NICs (TRUENAS_FREE_NIC)
	"truenas_network_interface.xmit_hash_policy":                   true, // needs a truly-free spare NIC (a bridge/LAG member must have no carrier); exercise on a VM with added virtual NICs (TRUENAS_FREE_NIC)
	"truenas_nvmet_subsys.ana":                                     true, // requires an HA / ANA-capable platform (rejected on a single controller)
	"truenas_pool.checksum":                                        true, // destructive: pool topology fields are RequiresReplace / destroy-recreate
	"truenas_pool.deduplication":                                   true, // destructive: pool topology fields are RequiresReplace / destroy-recreate
	"truenas_privilege.ds_groups":                                  true, // requires a directory-service (AD/LDAP) group to resolve against
	"truenas_rsync_task.ssh_credentials":                           true, // external: SSH mode needs a keychain SSH credential + a reachable host
	"truenas_service.running":                                      true, // unsafe: toggles a running system service
	"truenas_smb_config.search_protocols":                          true, // valid 26.0 enum values not determinable without a live box probe
	"truenas_system_advanced.overprovision":                        true, // unsafe: toggles risky advanced settings (serial console / kernel options)
	"truenas_system_dataset.pool_exclude":                          true, // unsafe: moves the system dataset
	"truenas_system_general.ds_auth":                               true, // requires directory services configured
	"truenas_system_general.ui_address":                            true, // unsafe: changing it risks the box's network/UI reachability
	"truenas_system_general.ui_allowlist":                          true, // unsafe: changing it risks the box's network/UI reachability
	"truenas_system_general.ui_certificate":                        true, // unsafe: changing it risks the box's network/UI reachability
	"truenas_system_general.ui_httpsport":                          true, // unsafe: changing it risks the box's network/UI reachability
	"truenas_system_general.ui_httpsprotocols":                     true, // unsafe: changing it risks the box's network/UI reachability
	"truenas_system_general.ui_httpsredirect":                      true, // unsafe: changing it risks the box's network/UI reachability
	"truenas_system_general.ui_port":                               true, // unsafe: changing it risks the box's network/UI reachability
	"truenas_system_general.ui_v6address":                          true, // unsafe: changing it risks the box's network/UI reachability
	"truenas_tunable.update_initramfs":                             true, // unsafe: writes a live sysctl/kernel tunable to the running box
	"truenas_twofactor_auth.enabled":                               true, // unsafe: a global 2FA toggle can lock out access
	"truenas_twofactor_auth.services":                              true, // unsafe: a global 2FA toggle can lock out access
	"truenas_webshare_config.groups":                               true, // needs AD/LDAP group names to resolve

	// #48 write-only password rotation triggers: an integer the operator bumps
	// to re-send a rotated write-only secret. Exercising one means a committed
	// config bumps it across apply steps while also rotating the paired
	// write-only secret live, which these resources' acceptance tests don't do
	// (same stance as every other _wo_version above). Ratcheted, paired with
	// the already-ratcheted/exercised secret.
}

func ratchetAllows(typeName, attr string) bool {
	return coverageRatchet[typeName+"."+attr]
}

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
	"truenas_kerberos_keytab.file_wo":               true, // write-only; absent from state, not in committed create config
	"truenas_kerberos_keytab.file_wo_version":       true, // paired with file_wo
	"truenas_truecommand_config.api_key_wo":         true, // write-only; truecommand create needs a real portal key (external)
	"truenas_truecommand_config.api_key_wo_version": true, // paired with api_key_wo
	// write-only "secrets-out-of-state" overlays (#36/#37): the _wo value is
	// absent from state and the _version is its paired trigger; exercised during
	// live verification, not set in the committed create config.
	"truenas_acme_dns_authenticator.attributes_secrets_wo":         true, // external: needs a live ACME account + DNS provider credentials
	"truenas_acme_dns_authenticator.attributes_secrets_wo_version": true, // external: needs a live ACME account + DNS provider credentials
	"truenas_alert_service.attributes_secrets_wo":                  true,
	"truenas_alert_service.attributes_secrets_wo_version":          true,
	"truenas_cloudsync_credentials.provider_secrets_wo":            true,
	"truenas_cloudsync_credentials.provider_secrets_wo_version":    true,
	"truenas_snmp_config.community_wo":                             true,
	"truenas_snmp_config.community_wo_version":                     true,
	"truenas_vm_device.attributes_secrets_wo":                      true, // external: needs a VM + guest
	"truenas_vm_device.attributes_secrets_wo_version":              true, // external: needs a VM + guest
	"truenas_alert_service.enabled":                                true,
	"truenas_api_key.expires_at":                                   true,
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
	"truenas_cronjob.stderr":                                       true,
	"truenas_cronjob.stdout":                                       true,
	"truenas_dataset.encryption_key":                               true, // explicit 64-hex key variant; generate_key path is tested
	"truenas_zvol.encryption_key":                                  true, // explicit 64-hex key variant; generate_key path is tested
	"truenas_directoryservices.enable_account_cache":               true, // external: needs a live AD/LDAP/IPA directory to join
	"truenas_directoryservices.enable_dns_updates":                 true, // external: needs a live AD/LDAP/IPA directory to join
	"truenas_directoryservices.kerberos_realm":                     true, // external: needs a live AD/LDAP/IPA directory to join
	"truenas_docker_config.address_pools":                          true, // external: configures the app/docker platform; needs a dedicated apps pool
	"truenas_docker_config.cidr_v6":                                true, // external: configures the app/docker platform; needs a dedicated apps pool
	"truenas_docker_config.nvidia":                                 true, // external: configures the app/docker platform; needs a dedicated apps pool
	"truenas_docker_config.pool":                                   true, // external: configures the app/docker platform; needs a dedicated apps pool
	"truenas_docker_config.registry_mirrors":                       true, // external: configures the app/docker platform; needs a dedicated apps pool
	"truenas_failover_config.disabled":                             true, // unsafe: requires an HA controller pair
	"truenas_failover_config.master":                               true, // unsafe: requires an HA controller pair
	"truenas_filesystem_permissions.recursive":                     true,
	"truenas_filesystem_permissions.traverse":                      true,
	"truenas_ftp_config.ssltls_certificate":                        true,
	"truenas_group.sudo_commands_nopasswd":                         true,
	"truenas_group.users":                                          true,
	"truenas_init_shutdown_script.script":                          true,
	"truenas_init_shutdown_script.timeout":                         true,
	"truenas_ipmi_lan.apply_remote":                                true, // hardware: needs a real BMC (available on .68); exercise there
	"truenas_ipmi_lan.password":                                    true, // hardware: needs a real BMC (available on .68); exercise there
	"truenas_iscsi_auth.peersecret":                                true,
	"truenas_iscsi_auth.peeruser":                                  true,
	"truenas_iscsi_target.auth_networks":                           true,
	"truenas_kerberos_config.libdefaults_aux":                      true,
	"truenas_kerberos_realm.kpasswd_server":                        true,
	"truenas_kerberos_realm.primary_kdc":                           true,
	"truenas_lxc_config.bridge":                                    true,
	"truenas_lxc_config.preferred_pool":                            true,
	"truenas_lxc_config.v6_network":                                true,
	"truenas_network_config.domain":                                true, // unsafe: mutating global network config can cut connectivity mid-test
	"truenas_network_config.domains":                               true, // unsafe: mutating global network config can cut connectivity mid-test
	"truenas_network_config.hosts":                                 true, // unsafe: mutating global network config can cut connectivity mid-test
	"truenas_network_config.httpproxy":                             true, // unsafe: mutating global network config can cut connectivity mid-test
	"truenas_network_config.ipv4gateway":                           true, // unsafe: mutating global network config can cut connectivity mid-test
	"truenas_network_config.ipv6gateway":                           true, // unsafe: mutating global network config can cut connectivity mid-test
	"truenas_network_config.nameserver1":                           true, // unsafe: mutating global network config can cut connectivity mid-test
	"truenas_network_config.nameserver2":                           true, // unsafe: mutating global network config can cut connectivity mid-test
	"truenas_network_config.nameserver3":                           true, // unsafe: mutating global network config can cut connectivity mid-test
	"truenas_network_config.service_announcement":                  true, // unsafe: mutating global network config can cut connectivity mid-test
	"truenas_network_interface.aliases":                            true, // unsafe: mutating an interface can cut the box's management connectivity mid-test
	"truenas_network_interface.bridge_members":                     true, // unsafe: mutating an interface can cut the box's management connectivity mid-test
	"truenas_network_interface.description":                        true, // unsafe: mutating an interface can cut the box's management connectivity mid-test
	"truenas_network_interface.enable_learning":                    true, // unsafe: mutating an interface can cut the box's management connectivity mid-test
	"truenas_network_interface.ipv4_dhcp":                          true, // unsafe: mutating an interface can cut the box's management connectivity mid-test
	"truenas_network_interface.ipv6_auto":                          true, // unsafe: mutating an interface can cut the box's management connectivity mid-test
	"truenas_network_interface.lacpdu_rate":                        true, // unsafe: mutating an interface can cut the box's management connectivity mid-test
	"truenas_network_interface.lag_ports":                          true, // unsafe: mutating an interface can cut the box's management connectivity mid-test
	"truenas_network_interface.lag_protocol":                       true, // unsafe: mutating an interface can cut the box's management connectivity mid-test
	"truenas_network_interface.mtu":                                true, // unsafe: mutating an interface can cut the box's management connectivity mid-test
	"truenas_network_interface.stp":                                true, // unsafe: mutating an interface can cut the box's management connectivity mid-test
	"truenas_network_interface.vlan_parent_interface":              true, // unsafe: mutating an interface can cut the box's management connectivity mid-test
	"truenas_network_interface.vlan_pcp":                           true, // unsafe: mutating an interface can cut the box's management connectivity mid-test
	"truenas_network_interface.vlan_tag":                           true, // unsafe: mutating an interface can cut the box's management connectivity mid-test
	"truenas_network_interface.xmit_hash_policy":                   true, // unsafe: mutating an interface can cut the box's management connectivity mid-test
	"truenas_ntp_server.maxpoll":                                   true,
	"truenas_ntp_server.minpoll":                                   true,
	"truenas_nvmet_namespace.filesize":                             true,
	"truenas_nvmet_namespace.nsid":                                 true,
	"truenas_nvmet_port.inline_data_size":                          true,
	"truenas_nvmet_port.pi_enable":                                 true,
	"truenas_nvmet_subsys.ana":                                     true,
	"truenas_periodic_snapshot_task.allow_empty":                   true,
	"truenas_periodic_snapshot_task.exclude":                       true,
	"truenas_pool.checksum":                                        true, // destructive: pool topology fields are RequiresReplace / destroy-recreate
	"truenas_pool.deduplication":                                   true, // destructive: pool topology fields are RequiresReplace / destroy-recreate
	"truenas_privilege.ds_groups":                                  true,
	"truenas_resilver_config.begin":                                true,
	"truenas_resilver_config.end":                                  true,
	"truenas_resilver_config.weekday":                              true,
	"truenas_rsync_task.ssh_credentials":                           true, // external: SSH mode needs a keychain SSH credential + a reachable host
	"truenas_service.running":                                      true, // unsafe: toggles a running system service
	"truenas_smb_config.search_protocols":                          true,
	"truenas_system_advanced.overprovision":                        true, // unsafe: toggles risky advanced settings (serial console / kernel options)
	"truenas_system_dataset.pool_exclude":                          true, // unsafe: moves the system dataset
	"truenas_system_general.ds_auth":                               true,
	"truenas_system_general.kbdmap":                                true,
	"truenas_system_general.ui_address":                            true,
	"truenas_system_general.ui_allowlist":                          true,
	"truenas_system_general.ui_certificate":                        true,
	"truenas_system_general.ui_consolemsg":                         true,
	"truenas_system_general.ui_httpsport":                          true,
	"truenas_system_general.ui_httpsprotocols":                     true,
	"truenas_system_general.ui_httpsredirect":                      true,
	"truenas_system_general.ui_port":                               true,
	"truenas_system_general.ui_v6address":                          true,
	"truenas_system_general.ui_x_frame_options":                    true,
	"truenas_system_general.usage_collection":                      true,
	"truenas_tunable.update_initramfs":                             true, // unsafe: writes a live sysctl/kernel tunable to the running box
	"truenas_twofactor_auth.enabled":                               true, // unsafe: a global 2FA toggle can lock out access
	"truenas_twofactor_auth.services":                              true, // unsafe: a global 2FA toggle can lock out access
	"truenas_zvol.refreservation":                                  true, // sparse zvol carries no local reservation (null on 26.0+); needs a non-sparse fixture
	"truenas_zvol.reservation":                                     true,
	"truenas_vm.arch_type":                                         true, // external: needs a bootable guest to exercise meaningfully
	"truenas_vm.bootloader_ovmf":                                   true, // external: needs a bootable guest to exercise meaningfully
	"truenas_vm.cpu_model":                                         true, // external: needs a bootable guest to exercise meaningfully
	"truenas_vm.machine_type":                                      true, // external: needs a bootable guest to exercise meaningfully
	"truenas_webshare.is_home_base":                                true,
	"truenas_webshare_config.bindip":                               true,
	"truenas_webshare_config.groups":                               true,
	"truenas_webshare_config.passkey":                              true,

	// #48 write-only password rotation triggers: an integer the operator bumps
	// to re-send a rotated write-only secret. Exercising one means a committed
	// config bumps it across apply steps while also rotating the paired
	// write-only secret live, which these resources' acceptance tests don't do
	// (same stance as every other _wo_version above). Ratcheted, paired with
	// the already-ratcheted/exercised secret.
	"truenas_ipmi_lan.password_wo_version":     true, // paired with password
	"truenas_iscsi_auth.secret_wo_version":     true, // paired with secret
	"truenas_iscsi_auth.peersecret_wo_version": true, // paired with peersecret
	"truenas_mail.pass_wo_version":             true, // paired with pass
	"truenas_user.password_wo_version":         true, // paired with password
}

func ratchetAllows(typeName, attr string) bool {
	return coverageRatchet[typeName+"."+attr]
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

// importIgnoreJustified records why each attribute excluded from
// ImportStateVerify genuinely cannot round-trip. See TestImportIgnore_Justified.
// "We don't read it" is not a valid reason — that is the GH #32 bug. Categories:
//
//	secret   — write-only; the API never returns it.
//	create   — a create-time-only input/flag, not a stored property the API echoes.
//	server   — a server-owned or server-normalized value that legitimately differs.
//
// The list only shrinks: when a field starts round-tripping, drop its ignore here.
var importIgnoreJustified = map[string]string{
	// --- write-only secrets ---
	"truenas_api_key.key":                               "secret: the key is returned only once at create, never by query",
	"truenas_certificate.passphrase":                    "secret: private-key passphrase, write-only",
	"truenas_cloud_backup.password":                     "secret: write-only repo password",
	"truenas_cloudsync_credentials.provider_config":     "secret: provider_config carries credentials, not returned verbatim",
	"truenas_dataset.encryption_key":                    "secret: write-only hex key",
	"truenas_dataset.encryption_passphrase":             "secret: write-only passphrase",
	"truenas_ipmi_lan.password":                         "secret: write-only BMC password",
	"truenas_iscsi_auth.secret":                         "secret: write-only CHAP secret",
	"truenas_iscsi_auth.peersecret":                     "secret: write-only mutual-CHAP secret",
	"truenas_mail.pass":                                 "secret: write-only SMTP password",
	"truenas_nvmet_host.dhchap_key":                     "secret: write-only DH-HMAC-CHAP key",
	"truenas_nvmet_host.dhchap_ctrl_key":                "secret: write-only DH-HMAC-CHAP controller key",
	"truenas_snmp_config.v3_password":                   "secret: write-only SNMPv3 auth password",
	"truenas_snmp_config.v3_privpassphrase":             "secret: write-only SNMPv3 privacy passphrase",
	"truenas_system_advanced.sed_passwd":                "secret: write-only SED password",
	"truenas_ups_config.monpwd":                         "secret: write-only UPS monitor password",
	"truenas_cloudsync_credentials.provider_secrets_wo": "secret: WriteOnly attribute, never persisted to state",
	"truenas_kerberos_keytab.file_wo":                   "secret: WriteOnly attribute, never persisted to state",
	"truenas_kerberos_keytab.file":                      "secret: import never reads the keytab into state (it cannot tell file from file_wo); a file-managed keytab records it in place on the next apply",

	// --- write-only rotation triggers: config-only counters the API never sees ---
	"truenas_alert_service.attributes_secrets_wo_version":       "secret: _wo_version is a config-only re-send counter, not sent to or returned by the API",
	"truenas_cloudsync_credentials.provider_secrets_wo_version": "secret: _wo_version is a config-only re-send counter, not sent to or returned by the API",
	"truenas_iscsi_auth.secret_wo_version":                      "secret: _wo_version is a config-only re-send counter, not sent to or returned by the API",
	"truenas_iscsi_auth.peersecret_wo_version":                  "secret: _wo_version is a config-only re-send counter, not sent to or returned by the API",
	"truenas_kerberos_keytab.file_wo_version":                   "secret: _wo_version is a config-only re-send counter, not sent to or returned by the API",
	"truenas_ups_config.monpwd_wo_version":                      "secret: _wo_version is a config-only re-send counter, not sent to or returned by the API",
	"truenas_user.password_wo_version":                          "secret: _wo_version is a config-only re-send counter, not sent to or returned by the API",
	"truenas_user.password":                                     "secret: write-only account password",

	// --- create-time-only inputs / flags (not stored properties the API returns) ---
	"truenas_certificate.create_type":          "create: CSR vs IMPORTED_CSR leave identical objects, so a CSR reads back null (ACME/IMPORTED are derived); recorded in place via planmod.ReplaceFromKnownString",
	"truenas_certificate.add_to_trusted_store": "create: install-time action, not a stored cert property",
	"truenas_certificate.ec_curve":             "create: key-generation input, not echoed for an existing cert; recorded in place via planmod.ReplaceFromKnownString",
	"truenas_dataset.encryption_generate_key":  "create: generate-key flag consumed at creation, not returned",
	"truenas_filesystem_acl.recursive":         "create: apply-scope flag for setacl, not a stored property",
	"truenas_filesystem_acl.traverse":          "create: apply-scope flag for setacl, not a stored property",
	"truenas_filesystem_permissions.recursive": "create: apply-scope flag for setperm, not a stored property",
	"truenas_filesystem_permissions.traverse":  "create: apply-scope flag for setperm, not a stored property",
	"truenas_keychain_ssh_keypair.generate":    "create: generate-keypair flag consumed at creation, not returned; recorded in place via planmod.ReplaceFromKnownBool",
	"truenas_ntp_server.force":                 "create: force-add flag, not a stored property",
	"truenas_rsync_task.ssh_keyscan":           "create: one-time host-key-scan flag, not a stored property",
	"truenas_rsync_task.validate_rpath":        "create: one-time remote-path validation flag, not a stored property",
	"truenas_snapshot.recursive":               "create: recursive-create flag, not a stored snapshot property; recorded in place via planmod.ReplaceFromKnownBool",
	"truenas_tunable.update_initramfs":         "create: one-time initramfs-rebuild flag, not a stored property",
	"truenas_ipmi_lan.apply_remote":            "create: targets the remote controller for this call, not stored",
	"truenas_user.group_create":                "create: 'also create a primary group' flag, sent only on create",
	"truenas_zvol.sparse":                      "create: thin-provisioning flag set at creation, not returned",
	"truenas_boot_environment.source":          "create: the clone source, not a property of the resulting BE; recorded in place via planmod.ReplaceFromKnownString",

	// --- server-owned / server-normalized values ---
	"truenas_pool.allocated":                    "server: live usage counter, changes on its own",
	"truenas_pool.free":                         "server: live capacity counter, changes on its own",
	"truenas_container.image":                   "create: container.query/get_instance never return image (it reads back null); recorded in place via planmod.ReplaceFromKnownObject",
	"truenas_nfs_share.networks":                "server: CIDRs are normalized server-side (see nfs/normalize.go)",
	"truenas_acme_dns_authenticator.attributes": "server: provider attributes blob re-normalized server-side",
	"truenas_alert_service.attributes":          "server: service attributes blob re-normalized server-side",
	"truenas_cloud_backup.attributes":           "server: provider attributes blob re-normalized server-side",
	"truenas_vm_device.attributes":              "server: device attributes JSON re-normalized server-side (key order, defaults)",

	"truenas_user.groups":    "server: group_create auto-adds a primary group, so live membership differs from the configured set (groups themselves are read back, model.go)",
	"truenas_user.home_mode": "create: user.query returns only 'home', not 'home_mode' (a filesystem mode applied at create, not stored in the user record) — confirmed live",
}

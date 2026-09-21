#!/usr/bin/env python3
"""Populate the `subcategory:` frontmatter of generated provider docs.

tfplugindocs leaves subcategory empty. The Terraform Registry uses it to
group a provider's resource/data-source pages under headings. This script
runs after tfplugindocs (see the `generate` target in GNUmakefile) and sets
each page's subcategory from the domain map below, keyed by the page's base
filename.

Every generated resource/data-source page must map to a subcategory; the
script exits non-zero if it finds a page whose basename is not in the map,
so a new resource cannot silently ship without a group.
"""
import os
import re
import sys

# Base filename (without .md) -> Registry subcategory heading.
SUBCATEGORY = {
    # Storage
    "pool": "Storage",
    "dataset": "Storage",
    "zvol": "Storage",
    "snapshot": "Storage",
    "periodic_snapshot_task": "Storage",
    "scrub_task": "Storage",
    "resilver_config": "Storage",
    "system_dataset": "Storage",
    # Sharing
    "nfs_share": "Sharing",
    "smb_share": "Sharing",
    "webshare": "Sharing",
    "nfs_config": "Sharing",
    "smb_config": "Sharing",
    "webshare_config": "Sharing",
    # Block storage - iSCSI
    "iscsi_global": "Block Storage (iSCSI)",
    "iscsi_portal": "Block Storage (iSCSI)",
    "iscsi_initiator": "Block Storage (iSCSI)",
    "iscsi_auth": "Block Storage (iSCSI)",
    "iscsi_extent": "Block Storage (iSCSI)",
    "iscsi_target": "Block Storage (iSCSI)",
    "iscsi_targetextent": "Block Storage (iSCSI)",
    # Block storage - NVMe-oF
    "nvmet_global": "Block Storage (NVMe-oF)",
    "nvmet_subsys": "Block Storage (NVMe-oF)",
    "nvmet_port": "Block Storage (NVMe-oF)",
    "nvmet_namespace": "Block Storage (NVMe-oF)",
    "nvmet_host": "Block Storage (NVMe-oF)",
    "nvmet_host_subsys": "Block Storage (NVMe-oF)",
    "nvmet_port_subsys": "Block Storage (NVMe-oF)",
    # Accounts & access
    "user": "Accounts & Access",
    "group": "Accounts & Access",
    "api_key": "Accounts & Access",
    "privilege": "Accounts & Access",
    "twofactor_auth": "Accounts & Access",
    # Directory services & Kerberos
    "directoryservices": "Directory Services & Kerberos",
    "kerberos_config": "Directory Services & Kerberos",
    "kerberos_realm": "Directory Services & Kerberos",
    "kerberos_keytab": "Directory Services & Kerberos",
    # Certificates
    "certificate": "Certificates",
    "acme_dns_authenticator": "Certificates",
    # Keychain & data protection
    "keychain_ssh_keypair": "Keychain & Data Protection",
    "keychain_ssh_connection": "Keychain & Data Protection",
    "replication_task": "Keychain & Data Protection",
    "replication_config": "Keychain & Data Protection",
    "cloudsync_task": "Keychain & Data Protection",
    "cloudsync_credentials": "Keychain & Data Protection",
    "cloud_backup": "Keychain & Data Protection",
    # Scheduled tasks
    "cronjob": "Scheduled Tasks",
    "init_shutdown_script": "Scheduled Tasks",
    "rsync_task": "Scheduled Tasks",
    # Filesystem permissions & ACLs
    "filesystem_permissions": "Filesystem Permissions & ACLs",
    "filesystem_acl": "Filesystem Permissions & ACLs",
    "acl_template": "Filesystem Permissions & ACLs",
    # Apps & containers
    "app": "Apps & Containers",
    "app_registry": "Apps & Containers",
    "docker_config": "Apps & Containers",
    "docker_network": "Apps & Containers",
    "catalog_config": "Apps & Containers",
    "container": "Apps & Containers",
    "container_device": "Apps & Containers",
    "container_image": "Apps & Containers",
    "lxc_config": "Apps & Containers",
    # Virtualization
    "vm": "Virtualization",
    "vm_device": "Virtualization",
    "vmware": "Virtualization",
    # Networking
    "network_config": "Networking",
    "network_interface": "Networking",
    "static_route": "Networking",
    # System
    "system_general": "System",
    "system_advanced": "System",
    "tunable": "System",
    "boot_environment": "System",
    "service": "System",
    "ntp_server": "System",
    # Service configuration
    "ssh_config": "Service Configuration",
    "ftp_config": "Service Configuration",
    "snmp_config": "Service Configuration",
    "ups_config": "Service Configuration",
    "mail": "Service Configuration",
    # Alerts & reporting
    "alert_service": "Alerts & Reporting",
    "alert_policy": "Alerts & Reporting",
    "reporting_exporter": "Alerts & Reporting",
    "audit_config": "Alerts & Reporting",
    # HA & Enterprise
    "failover_config": "HA & Enterprise",
    "ipmi_lan": "HA & Enterprise",
    "enclosure": "HA & Enterprise",
    "enclosure_label": "HA & Enterprise",
    "truecommand_config": "HA & Enterprise",
    "tn_connect_config": "HA & Enterprise",
}

FRONTMATTER_RE = re.compile(r'^(subcategory:)\s*".*?"\s*$', re.MULTILINE)


def main() -> int:
    root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    unmapped = []
    changed = 0
    for sub in ("resources", "data-sources"):
        d = os.path.join(root, "docs", sub)
        if not os.path.isdir(d):
            continue
        for fn in os.listdir(d):
            if not fn.endswith(".md"):
                continue
            base = fn[:-3]
            cat = SUBCATEGORY.get(base)
            if cat is None:
                unmapped.append(f"{sub}/{fn}")
                continue
            p = os.path.join(d, fn)
            s = open(p, encoding="utf-8").read()
            new, n = FRONTMATTER_RE.subn(f'subcategory: "{cat}"', s, count=1)
            if n and new != s:
                open(p, "w", encoding="utf-8").write(new)
                changed += 1

    if unmapped:
        print("ERROR: doc pages with no subcategory mapping:", file=sys.stderr)
        for u in sorted(unmapped):
            print("  " + u, file=sys.stderr)
        print("Add them to SUBCATEGORY in scripts/set-subcategories.py.",
              file=sys.stderr)
        return 1

    print(f"subcategories set on {changed} page(s)")
    return 0


if __name__ == "__main__":
    sys.exit(main())

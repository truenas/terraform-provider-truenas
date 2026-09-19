# NOTE: joining Active Directory is disruptive (creates a computer account
# and DNS records on the domain controller). "credential.password" is
# write-only (Terraform >= 1.11 required): it is sent to TrueNAS on
# create/update but never stored in state. Destroying this resource only
# disables directory services locally (enable = false) — it never leaves
# the domain, so the TrueNAS computer account is left in place on the
# domain controller.
resource "truenas_directoryservices" "ad" {
  service_type = "ACTIVEDIRECTORY"
  enable       = true

  credential = {
    credential_type = "KERBEROS_USER"
    username        = "administrator"
    password        = var.ad_join_password
  }

  configuration_activedirectory = {
    hostname = "truenas"
    domain   = "example.internal"

    # Optional: explicit idmap override. Left unset (the default), TrueNAS
    # assigns its own RID-backend ranges on first join and this provider
    # preserves them on every later update.
    idmap = {
      builtin = {
        range_low  = 90000001
        range_high = 100000000
      }
      idmap_domain = {
        idmap_backend = "RID"
        range_low     = 100000001
        range_high    = 200000000
      }
    }
  }
}

variable "ad_join_password" {
  type      = string
  sensitive = true
}

# NOTE: LDAP directory services bind to a plain LDAP server rather than
# joining a domain. "credential.bindpw" is write-only (Terraform >= 1.11
# required): sent on create/update but never stored in state. Destroying
# this resource only disables directory services locally (enable = false).
resource "truenas_directoryservices" "ldap" {
  service_type = "LDAP"
  enable       = true

  credential = {
    credential_type = "LDAP_PLAIN"
    binddn          = "cn=admin,dc=example,dc=internal"
    bindpw          = var.ldap_bind_password
  }

  configuration_ldap = {
    server_urls           = ["ldaps://ldap.example.internal"]
    basedn                = "dc=example,dc=internal"
    validate_certificates = true
  }
}

variable "ldap_bind_password" {
  type      = string
  sensitive = true
}

# NOTE: joining IPA (FreeIPA) is disruptive in the same way Active
# Directory is (creates a host entry and DNS records on the IPA server).
# "credential.password" is write-only (Terraform >= 1.11 required).
# Destroying this resource only disables directory services locally
# (enable = false) — it never leaves the domain.
resource "truenas_directoryservices" "ipa" {
  service_type = "IPA"
  enable       = true

  credential = {
    credential_type = "KERBEROS_USER"
    username        = "admin"
    password        = var.ipa_join_password
  }

  configuration_ipa = {
    target_server = "ipa.example.internal"
    hostname      = "truenas"
    domain        = "example.internal"
    basedn        = "dc=example,dc=internal"
  }
}

variable "ipa_join_password" {
  type      = string
  sensitive = true
}

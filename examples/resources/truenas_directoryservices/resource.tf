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
  }
}

variable "ad_join_password" {
  type      = string
  sensitive = true
}

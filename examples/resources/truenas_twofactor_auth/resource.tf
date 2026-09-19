# Singleton resource: manages the one two-factor authentication
# configuration on the system. SAFETY: changing "enabled" affects
# password-based logins system-wide (API-key authentication, used by this
# provider, is unaffected). Terraform destroy only removes it from state;
# the configuration is left in place on TrueNAS.
resource "truenas_twofactor_auth" "config" {
  enabled = true
  window  = 30

  services = {
    ssh = false
  }
}

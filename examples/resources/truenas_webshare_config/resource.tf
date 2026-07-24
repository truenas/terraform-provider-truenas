# Singleton resource: manages the one Webshare service configuration on the
# system (TrueNAS 26.0+ only). Terraform destroy only removes it from
# state; the Webshare configuration is left in place as-is.
resource "truenas_webshare_config" "config" {
  search  = true
  passkey = "DISABLED"
}

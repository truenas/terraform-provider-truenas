resource "truenas_smb_share" "data" {
  path    = "/mnt/tank/data"
  name    = "data"
  comment = "Managed by Terraform"
  enabled = true
}

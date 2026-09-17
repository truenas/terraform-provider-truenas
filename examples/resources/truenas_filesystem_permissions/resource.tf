# NOTE: `terraform destroy` does NOT revert permissions -- it only removes
# the resource from Terraform state, leaving mode/uid/gid as last applied.
resource "truenas_filesystem_permissions" "data_dir" {
  path = "/mnt/tank/mydata"
  mode = "0750"
  uid  = 1000
  gid  = 1000
}

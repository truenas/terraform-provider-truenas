resource "truenas_nfs_share" "example" {
  path    = "/mnt/tank/mydata"
  comment = "NFS share managed by Terraform"
  enabled = true
  networks = ["192.168.1.0/24"]
}

output "share_id" {
  value = truenas_nfs_share.example.id
}

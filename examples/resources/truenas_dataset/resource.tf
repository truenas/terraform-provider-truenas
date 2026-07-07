resource "truenas_dataset" "example" {
  name        = "tank/mydata"
  type        = "FILESYSTEM"
  compression = "lz4"
  comments    = "Managed by Terraform"
}

output "mountpoint" {
  value = truenas_dataset.example.mountpoint
}

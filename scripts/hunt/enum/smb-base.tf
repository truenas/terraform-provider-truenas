terraform {
  required_providers {
    truenas = { source = "truenas/truenas" }
  }
}
variable "endpoint" { type = string }
variable "api_key" {
  type      = string
  sensitive = true
}
variable "pool" { type = string }
provider "truenas" {
  endpoint = var.endpoint
  api_key  = var.api_key
  insecure = true
}
resource "truenas_dataset" "d" {
  name = "${var.pool}/tfenum-smb__SUFFIX__"
}
resource "truenas_smb_share" "s" {
  name = "tfenum__SUFFIX__"
  path = truenas_dataset.d.mountpoint
__EXTRA__
}

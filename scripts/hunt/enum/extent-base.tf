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
resource "truenas_zvol" "z" {
  name    = "${var.pool}/tfenum-ext__SUFFIX__"
  volsize = 16777216
  sparse  = true
}
resource "truenas_iscsi_extent" "e" {
  name = "tfenum__SUFFIX__"
  type = "DISK"
  disk = "zvol/${truenas_zvol.z.name}"
__EXTRA__
}

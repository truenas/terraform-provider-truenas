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
  name    = "${var.pool}/tfenum-z__SUFFIX__"
  volsize = 33554432
  sparse  = true
__EXTRA__
}

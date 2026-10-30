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
resource "truenas_certificate" "c" {
  name        = "tfenum__SUFFIX__"
  create_type = "CERTIFICATE_CREATE_CSR"
  common      = "tfenum.example.com"
  san         = ["tfenum.example.com"]
__EXTRA__
}

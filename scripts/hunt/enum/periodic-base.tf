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
  name = "${var.pool}/tfenum-ps__SUFFIX__"
}
resource "truenas_periodic_snapshot_task" "p" {
  dataset        = truenas_dataset.d.name
  lifetime_value = 1
  naming_schema  = "auto-%Y-%m-%d_%H-%M"
  schedule = {
    minute = "0"
    hour   = "0"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
__EXTRA__
}

resource "truenas_ups_config" "config" {
  identifier = "ups"
  mode       = "MASTER"
  driver     = "usbhid-ups$PROTECT UPS"
  port       = "auto"
  shutdown   = "BATT"
  monuser    = "upsmon"
  monpwd     = var.ups_password
}

variable "ups_password" {
  type      = string
  sensitive = true
}

resource "truenas_reporting_exporter" "graphite" {
  name    = "graphite-primary"
  enabled = true

  attributes = {
    destination_ip   = "192.168.1.50"
    destination_port = 2003
    namespace        = "truenas"
    prefix           = "scale"
    update_every     = 10
  }
}

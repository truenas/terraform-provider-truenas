resource "truenas_network_config" "config" {
  hostname    = "truenas"
  domain      = "example.com"
  nameserver1 = "8.8.8.8"
  nameserver2 = "8.8.4.4"

  service_announcement = {
    mdns    = true
    netbios = true
    wsd     = true
  }
}

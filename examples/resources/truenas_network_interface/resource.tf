resource "truenas_network_interface" "br0" {
  name = "br0"
  type = "BRIDGE"

  bridge_members = ["enp8s0"]

  aliases = [
    {
      address = "192.168.10.1"
      netmask = 24
      type    = "INET"
    }
  ]
}

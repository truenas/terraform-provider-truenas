resource "truenas_static_route" "vpn" {
  destination = "10.20.0.0/16"
  gateway     = "192.168.1.254"
  description = "VPN subnet via edge router"
}

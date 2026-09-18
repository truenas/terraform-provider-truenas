# Manages one pre-existing physical BMC/IPMI LAN channel. Channels are
# never created or destroyed via this resource -- only configured; see the
# resource's schema description for the full safety notes around changing
# a live out-of-band BMC network interface, and around "password" and
# "apply_remote".
resource "truenas_ipmi_lan" "channel1" {
  channel   = 1
  dhcp      = false
  ipaddress = "192.168.1.150"
  netmask   = "255.255.255.0"
  gateway   = "192.168.1.1"

  # Optional: write-only, never stored in Terraform state or read back.
  # password = "SuperSecret123!"
}

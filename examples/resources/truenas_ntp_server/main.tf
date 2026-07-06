resource "truenas_ntp_server" "pool" {
  address = "time.cloudflare.com"
  iburst  = true
  prefer  = true
}

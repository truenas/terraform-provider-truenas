resource "truenas_ftp_config" "config" {
  port    = 21
  clients = 10
  tls     = false
}

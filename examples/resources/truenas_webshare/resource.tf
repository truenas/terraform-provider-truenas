resource "truenas_webshare" "data" {
  path    = "/mnt/tank/data"
  name    = "data"
  enabled = true
}

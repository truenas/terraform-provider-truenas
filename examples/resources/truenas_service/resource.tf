resource "truenas_service" "nfs" {
  name    = "nfs"
  enabled = true
  running = true
}

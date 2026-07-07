resource "truenas_snapshot" "backup" {
  dataset   = "tank/mydata"
  name      = "backup-2026-07-01"
  recursive = false
}

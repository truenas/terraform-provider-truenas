resource "truenas_cronjob" "nightly_cleanup" {
  command     = "/usr/local/bin/cleanup.sh"
  user        = "root"
  description = "Nightly tmp cleanup"

  schedule = {
    minute = "30"
    hour   = "2"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }

  enabled = true
  stdout  = true
  stderr  = false
}

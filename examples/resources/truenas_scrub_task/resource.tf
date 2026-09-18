resource "truenas_scrub_task" "tank_weekly" {
  pool      = truenas_pool.tank.id
  threshold = 35
  enabled   = true

  schedule = {
    minute = "0"
    hour   = "0"
    dom    = "*"
    month  = "*"
    dow    = "7" # Sunday
  }
}

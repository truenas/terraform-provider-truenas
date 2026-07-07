resource "truenas_periodic_snapshot_task" "daily" {
  dataset        = "tank/mydata"
  recursive      = true
  lifetime_value = 2
  lifetime_unit  = "WEEK"
  naming_schema  = "auto-%Y-%m-%d_%H-%M"
  enabled        = true

  schedule {
    minute = "0"
    hour   = "0"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}

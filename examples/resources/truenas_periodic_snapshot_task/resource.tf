resource "truenas_dataset" "mydata" {
  name = "tank/mydata"
}

resource "truenas_periodic_snapshot_task" "daily" {
  # Reference the dataset by name rather than hardcoding "tank/mydata", so
  # Terraform creates the dataset before the task that snapshots it (a
  # hardcoded name gives no dependency edge and can fail on the first apply).
  dataset        = truenas_dataset.mydata.name
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

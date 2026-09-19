resource "truenas_dataset" "mydata" {
  name = "tank/mydata"
}

resource "truenas_snapshot" "backup" {
  # Reference the dataset by name rather than hardcoding "tank/mydata", so
  # Terraform creates the dataset before snapshotting it (a hardcoded name
  # gives no dependency edge and can fail on the first apply).
  dataset   = truenas_dataset.mydata.name
  name      = "backup-2026-07-01"
  recursive = false
}

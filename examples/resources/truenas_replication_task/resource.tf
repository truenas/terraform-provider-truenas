resource "truenas_dataset" "data" {
  name = "tank/data"
}

resource "truenas_replication_task" "local_backup" {
  name      = "tank-to-backup"
  direction = "PUSH"
  transport = "LOCAL"
  # Reference the source dataset by name rather than hardcoding "tank/data", so
  # Terraform creates it before the task. (target_dataset is on a separate
  # destination pool not managed by this example, so it stays a literal.)
  source_datasets  = [truenas_dataset.data.name]
  target_dataset   = "backup/data"
  recursive        = true
  auto             = true
  retention_policy = "SOURCE"
  naming_schema    = ["auto-%Y-%m-%d_%H-%M"]

  schedule = {
    minute = "0"
    hour   = "4"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}

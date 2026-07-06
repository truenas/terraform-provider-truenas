resource "truenas_replication_task" "local_backup" {
  name             = "tank-to-backup"
  direction        = "PUSH"
  transport        = "LOCAL"
  source_datasets  = ["tank/data"]
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

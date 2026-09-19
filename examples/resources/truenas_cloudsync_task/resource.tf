resource "truenas_dataset" "data" {
  name = "tank/data"
}

resource "truenas_cloudsync_task" "nightly_backup" {
  description = "Nightly S3 backup"
  # Reference the dataset's mountpoint rather than hardcoding the path (e.g.
  # "/mnt/tank/data"). The reference gives Terraform a dependency edge so the
  # dataset is created before the task; a hardcoded path has no such edge and
  # may fail with "path not found" on the first apply.
  path          = truenas_dataset.data.mountpoint
  credentials   = truenas_cloudsync_credentials.backup_s3.id
  direction     = "PUSH"
  transfer_mode = "SYNC"

  attributes = jsonencode({
    bucket = "my-backup-bucket"
    folder = "truenas"
  })

  schedule = {
    minute = "0"
    hour   = "3"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}

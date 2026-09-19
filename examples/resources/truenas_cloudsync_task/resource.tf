resource "truenas_cloudsync_task" "nightly_backup" {
  description   = "Nightly S3 backup"
  path          = "/mnt/tank/data"
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

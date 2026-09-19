# NOTE: cloud_backup.create validates the credential/bucket against the
# actual remote endpoint at apply time -- a credential that can't reach the
# named bucket fails the apply, it is not accepted silently.
resource "truenas_dataset" "important" {
  name = "tank/important"
}

resource "truenas_cloud_backup" "offsite" {
  description = "offsite-s3-backup"
  # Reference the dataset's mountpoint rather than hardcoding the path (e.g.
  # "/mnt/tank/important"). The reference gives Terraform a dependency edge so
  # the dataset is created before the backup task; a hardcoded path has no such
  # edge and may fail with "path not found" on the first apply.
  path        = truenas_dataset.important.mountpoint
  credentials = truenas_cloudsync_credentials.backup_s3.id

  attributes = jsonencode({
    bucket = "my-backup-bucket"
    folder = "truenas-backups"
  })

  schedule = {
    minute = "0"
    hour   = "3"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }

  password  = var.restic_password
  keep_last = 14
  enabled   = true
}

resource "truenas_cloudsync_credentials" "backup_s3" {
  name = "backup-s3"
  provider_config = jsonencode({
    type              = "S3"
    access_key_id     = var.backup_s3_access_key_id
    secret_access_key = var.backup_s3_secret_access_key
  })
}

variable "restic_password" {
  type      = string
  sensitive = true
}

variable "backup_s3_access_key_id" {
  type      = string
  sensitive = true
}

variable "backup_s3_secret_access_key" {
  type      = string
  sensitive = true
}

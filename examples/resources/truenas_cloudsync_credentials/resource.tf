resource "truenas_cloudsync_credentials" "backup_s3" {
  name = "backup-s3"
  provider_config = jsonencode({
    type              = "S3"
    access_key_id     = "AKIA..."
    secret_access_key = "..."
  })
}

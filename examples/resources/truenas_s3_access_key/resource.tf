# A key whose secret TrueNAS generates. The secret is stored once, from the
# create response, as the Sensitive `secret` attribute; it is never read back.
resource "truenas_s3_access_key" "ci" {
  name           = "ci-state"
  username       = "svc_terraform" # a local user whose group holds SHARING_S3_WRITE
  manage_buckets = true            # may create buckets through the S3 protocol
  expires_at     = "2030-01-01T00:00:00Z"
}

output "ci_access_key" {
  value = truenas_s3_access_key.ci.access_key
}

output "ci_secret" {
  value     = truenas_s3_access_key.ci.secret
  sensitive = true
}

# A key whose access key id and secret come from the configuration. The
# secret is write-only: it never reaches state or a saved plan. Bumping
# secret_wo_version recreates the key with the new secret.
variable "backup_secret" {
  type      = string
  sensitive = true
}

resource "truenas_s3_access_key" "backup" {
  name              = "backup"
  username          = "svc_backup"
  access_key        = "BACKUPKEY0000000001"
  secret_wo         = var.backup_secret
  secret_wo_version = 1
}

# Rotate a generated secret by bumping rotate_version: the access key id is
# kept, `secret` gets the new value.
resource "truenas_s3_access_key" "rotated" {
  name           = "rotated"
  username       = "svc_terraform"
  rotate_version = 2
}

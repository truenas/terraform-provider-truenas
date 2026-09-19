resource "truenas_rsync_task" "offsite_backup" {
  path       = "/mnt/tank/mydata"
  user       = "backupuser"
  mode       = "SSH"
  remotehost = "backup.example.com"
  remotepath = "/backups/truenas"
  direction  = "PUSH"
  desc       = "Nightly offsite backup"
  enabled    = true

  schedule = {
    minute = "0"
    hour   = "2"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}

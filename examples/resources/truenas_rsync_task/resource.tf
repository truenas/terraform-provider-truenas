resource "truenas_dataset" "mydata" {
  name = "tank/mydata"
}

resource "truenas_rsync_task" "offsite_backup" {
  # Reference the dataset's mountpoint rather than hardcoding the path (e.g.
  # "/mnt/tank/mydata"). The reference gives Terraform a dependency edge so the
  # dataset is created before the task; a hardcoded path has no such edge and
  # may fail with "path not found" on the first apply.
  path       = truenas_dataset.mydata.mountpoint
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

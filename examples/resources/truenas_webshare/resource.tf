resource "truenas_dataset" "data" {
  name = "tank/data"
}

resource "truenas_webshare" "data" {
  # Reference the dataset's mountpoint rather than hardcoding the path (e.g.
  # "/mnt/tank/data"). The reference gives Terraform a dependency edge so the
  # dataset is created before the share; a hardcoded path has no such edge and
  # may fail with "path not found" on the first apply.
  path    = truenas_dataset.data.mountpoint
  name    = "data"
  enabled = true
}

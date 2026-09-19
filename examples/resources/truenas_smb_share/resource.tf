resource "truenas_dataset" "data" {
  name = "tank/data"
}

resource "truenas_smb_share" "data" {
  # Reference the dataset's mountpoint rather than hardcoding the path. The
  # reference makes Terraform create the dataset before the share; a hardcoded
  # path (e.g. "/mnt/tank/data") has no dependency edge, so the share may be
  # created before the dataset exists and fail with "path not found".
  path    = truenas_dataset.data.mountpoint
  name    = "data"
  comment = "Managed by Terraform"
  enabled = true
}

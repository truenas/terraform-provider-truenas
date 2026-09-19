resource "truenas_dataset" "media" {
  name = "tank/media"
}

resource "truenas_nfs_share" "example" {
  # Reference the dataset's mountpoint rather than hardcoding the path (e.g.
  # "/mnt/tank/media"). The reference gives Terraform a dependency edge so it
  # creates the dataset before the share; a hardcoded path has no such edge, so
  # Terraform may create the share before the dataset exists and fail with
  # "[ENOENT] Path ... not found".
  path     = truenas_dataset.media.mountpoint
  comment  = "NFS share managed by Terraform"
  enabled  = true
  networks = ["192.168.1.0/24"]
}

output "share_id" {
  value = truenas_nfs_share.example.id
}

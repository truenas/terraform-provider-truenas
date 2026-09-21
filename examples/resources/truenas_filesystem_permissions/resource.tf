# NOTE: `terraform destroy` does NOT revert permissions -- it only removes
# the resource from Terraform state, leaving mode/uid/gid as last applied.
resource "truenas_dataset" "data_dir" {
  name = "tank/mydata"
}

resource "truenas_filesystem_permissions" "data_dir" {
  # Reference the dataset's mountpoint rather than hardcoding the path (e.g.
  # "/mnt/tank/mydata"). The reference gives Terraform a dependency edge so it
  # creates the dataset before setting permissions on it; a hardcoded path has
  # no such edge, so Terraform may run filesystem.setperm before the dataset
  # exists and fail with "[ENOENT] Path ... not found". Referencing the pool
  # name is not enough -- it is the DATASET that creates this path.
  path = truenas_dataset.data_dir.mountpoint
  mode = "0750"
  uid  = 1000
  gid  = 1000
}

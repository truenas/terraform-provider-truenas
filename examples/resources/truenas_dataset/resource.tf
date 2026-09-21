# A dataset on a pre-existing pool: reference the pool by its literal name.
resource "truenas_dataset" "example" {
  name        = "tank/mydata"
  type        = "FILESYSTEM"
  compression = "lz4"
  comments    = "Managed by Terraform"
}

output "mountpoint" {
  value = truenas_dataset.example.mountpoint
}

# When Terraform ALSO manages the pool in the same configuration, build the
# dataset name from the pool resource instead of hardcoding it. That
# interpolation is what creates the dependency edge, so Terraform finishes
# creating the pool before it creates datasets on it. Without it, the two are
# created in parallel and the dataset loses the race against its own pool
# ("parent not found" on the first apply, works on the second).
resource "truenas_pool" "tank" {
  name = "tank"
  topology = {
    # Identify disks by stable serial, not the volatile sdX kernel name.
    data = [{ type = "MIRROR", disks = ["WD-WCC7K5PACL0V", "WD-WCC7K6ABXYZ1"] }]
  }
}

resource "truenas_dataset" "media" {
  name = "${truenas_pool.tank.name}/media"
}

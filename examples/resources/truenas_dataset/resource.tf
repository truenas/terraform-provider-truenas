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

# Route blocks smaller than 16 KiB to the pool's special allocation class
# vdev, leaving larger blocks on the data vdevs. The value must be 0 or a
# power of two no larger than the dataset's record size, and the pool needs a
# special vdev for it to have any effect.
#
# Omitting the attribute leaves the property inherited from the parent
# dataset, which is the default, and state then records "INHERIT". Once a
# size is applied, removing the attribute keeps it, and changing it to
# "INHERIT" fails the plan; set the size you want explicitly instead.
resource "truenas_dataset" "database" {
  name                     = "tank/database"
  special_small_block_size = 16384
}

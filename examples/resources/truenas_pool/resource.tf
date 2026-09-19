# Create a pool from scratch. Pool creation requires knowing the exact disk
# identifiers on the box (see the truenas_disk data source to discover them).
#
# `topology` is a nested attribute, so it takes the `= { ... }` assignment
# form (not a bare `topology { ... }` block).
resource "truenas_pool" "tank" {
  name     = "tank"
  autotrim = false
  topology = {
    data = [
      { type = "MIRROR", disks = ["sda", "sdb"] },
      { type = "MIRROR", disks = ["sdc", "sdd"] },
    ]
  }
}

# Datasets managed alongside the pool should build their name from the pool
# resource, so Terraform creates the pool before its datasets rather than
# racing them. See the truenas_dataset example for the full pattern.
resource "truenas_dataset" "data" {
  name = "${truenas_pool.tank.name}/data"
}

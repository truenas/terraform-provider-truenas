# Create a pool from scratch.
#
# Identify disks by their STABLE SERIAL, not the kernel device name (sdX).
# Kernel names are reassigned across reboots and controller changes; the
# provider stores the serial in state and resolves any accepted form (serial,
# TrueNAS identifier, /dev/disk/by-id path, or sdX) to the same physical disk,
# so a renumber never plans a pool replacement. Discover serials with
# `midclt call disk.query '[]' '{"select": ["name", "serial"]}'` on the box.
#
# `topology` is a nested attribute, so it takes the `= { ... }` assignment
# form (not a bare `topology { ... }` block). Use "MIRROR"/"RAIDZ2"/etc.;
# for a single-disk vdev use "STRIPE".
resource "truenas_pool" "tank" {
  name     = "tank"
  autotrim = false
  topology = {
    data = [
      { type = "MIRROR", disks = ["WD-WCC7K5PACL0V", "WD-WCC7K6ABXYZ1"] },
      { type = "MIRROR", disks = ["WD-WCC7K7CDEFG2", "WD-WCC7K8HIJKL3"] },
    ]
  }
}

# Datasets managed alongside the pool should build their name from the pool
# resource, so Terraform creates the pool before its datasets rather than
# racing them. See the truenas_dataset example for the full pattern.
resource "truenas_dataset" "data" {
  name = "${truenas_pool.tank.name}/data"
}

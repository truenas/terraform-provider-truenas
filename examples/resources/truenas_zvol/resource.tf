resource "truenas_zvol" "iso" {
  name    = "tank/iso"
  volsize = 10737418240 # 10 GiB
}

# Renaming: changing `name` renames the zvol in place (its snapshots move with
# it, `guid` is unchanged). TrueNAS performs NO safety checks: an iSCSI extent,
# NVMe-oF namespace or VM disk that uses the old device path keeps it and the
# LUN goes offline until re-pointed. The provider refuses the rename while
# anything references the zvol unless allow_rename_in_use = true.
resource "truenas_zvol" "vm1" {
  name    = "tank/vms/vm1" # was "tank/vm1": applied as a rename
  volsize = 21474836480
  sparse  = true
}

# Cloning from a golden image: keep the golden as a zvol with a snapshot and
# boot each VM from a clone of it. The clone shares the snapshot's blocks until
# it diverges and may be larger than the golden. `origin` is create-only (a
# different origin is a different disk, so changing it replaces the zvol; the
# destroy half needs force_destroy = true). ZFS refuses to destroy a snapshot
# while clones exist: referencing the snapshot resource in `origin` makes
# Terraform destroy the clones first.
resource "truenas_zvol" "golden" {
  name    = "tank/golden/debian13"
  volsize = 10737418240
}

resource "truenas_snapshot" "golden" {
  dataset = truenas_zvol.golden.name
  name    = "v1"
}

resource "truenas_zvol" "vm2" {
  name    = "tank/vms/vm2"
  origin  = truenas_snapshot.golden.id # "tank/golden/debian13@v1"
  volsize = 21474836480               # grown beyond the golden
}

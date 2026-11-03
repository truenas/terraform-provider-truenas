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

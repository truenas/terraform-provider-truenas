resource "truenas_iscsi_extent" "vm_disk" {
  name    = "vm-disk"
  type    = "DISK"
  disk    = "zvol/tank/vms/vm1"
  enabled = true
}

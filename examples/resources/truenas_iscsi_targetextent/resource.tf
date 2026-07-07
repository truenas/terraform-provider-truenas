resource "truenas_iscsi_target" "vm" {
  name = "vm-storage"
  mode = "ISCSI"
  groups = [
    {
      portal     = truenas_iscsi_portal.main.id
      authmethod = "NONE"
    }
  ]
}

resource "truenas_iscsi_extent" "vm_disk" {
  name    = "vm-disk"
  type    = "DISK"
  disk    = "zvol/tank/vms/vm1"
  enabled = true
}

# Associate the extent with the target as LUN 0.
resource "truenas_iscsi_targetextent" "vm_lun0" {
  target = truenas_iscsi_target.vm.id
  extent = truenas_iscsi_extent.vm_disk.id
  lunid  = 0
}

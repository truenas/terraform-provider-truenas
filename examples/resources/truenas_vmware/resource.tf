# NOTE: vmware.create validates "hostname"/"username"/"password" against
# the real vCenter/ESXi endpoint synchronously -- fabricated credentials or
# an unreachable hostname are rejected before anything is persisted, so
# point this at a VMware host you actually control.
resource "truenas_vmware" "backup_coordination" {
  hostname   = var.vcenter_hostname
  username   = var.vcenter_username
  password   = var.vcenter_password # write-only: never stored in state
  datastore  = "datastore1"
  filesystem = "tank/vmware-backups"
}

variable "vcenter_hostname" {
  type = string
}

variable "vcenter_username" {
  type = string
}

variable "vcenter_password" {
  type      = string
  sensitive = true
}

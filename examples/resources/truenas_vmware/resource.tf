# NOTE: vmware.create validates "hostname"/"username"/"password" against
# the real vCenter/ESXi endpoint synchronously -- fabricated credentials or
# an unreachable hostname are rejected before anything is persisted, so
# point this at a VMware host you actually control.
resource "truenas_dataset" "vmware_backups" {
  name = "tank/vmware-backups"
}

resource "truenas_vmware" "backup_coordination" {
  hostname  = var.vcenter_hostname
  username  = var.vcenter_username
  password  = var.vcenter_password # write-only: never stored in state
  datastore = "datastore1"
  # Reference the dataset by name rather than hardcoding "tank/vmware-backups",
  # so Terraform creates it before this resource.
  filesystem = truenas_dataset.vmware_backups.name
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

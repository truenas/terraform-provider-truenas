# A VM whose power state is managed separately from the VM, so it boots only
# once its disk and NIC exist. Leave `running` unset on the truenas_vm.
resource "truenas_vm" "web" {
  name   = "web"
  memory = 2048
  vcpus  = 2
}

resource "truenas_vm_device" "web_disk" {
  vm = truenas_vm.web.id
  attributes = jsonencode({
    dtype = "DISK"
    path  = "/dev/zvol/tank/vms/web"
    type  = "VIRTIO"
  })
}

resource "truenas_vm_device" "web_nic" {
  vm = truenas_vm.web.id
  attributes = jsonencode({
    dtype      = "NIC"
    type       = "VIRTIO"
    nic_attach = "br0"
  })
}

# Applied after both devices: the VM starts with them attached. A change to
# any restart_triggers value (here: the disk device or the VM's memory) while
# running restarts the VM so the guest picks it up. Works in OpenTofu too,
# which has no actions.
resource "truenas_vm_power" "web" {
  vm_id   = truenas_vm.web.id
  running = true
  restart_triggers = {
    disk   = truenas_vm_device.web_disk.attributes
    memory = tostring(truenas_vm.web.memory)
  }
  depends_on = [truenas_vm_device.web_disk, truenas_vm_device.web_nic]
}

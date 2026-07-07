resource "truenas_vm_device" "worker_disk" {
  vm = truenas_vm.worker.id
  attributes = jsonencode({
    dtype = "DISK"
    path  = "/dev/zvol/tank/vms/worker1"
    type  = "VIRTIO"
  })
}

resource "truenas_vm_device" "worker_nic" {
  vm = truenas_vm.worker.id
  attributes = jsonencode({
    dtype      = "NIC"
    type       = "VIRTIO"
    nic_attach = "eno1"
  })
}

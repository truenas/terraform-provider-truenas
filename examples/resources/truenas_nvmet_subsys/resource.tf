resource "truenas_nvmet_subsys" "vm" {
  name           = "vm-storage"
  allow_any_host = false
}

resource "truenas_nvmet_port" "tcp" {
  addr_trtype  = "TCP"
  addr_traddr  = "192.0.2.10" # the box IP NVMe-oF should listen on
  addr_trsvcid = 4420
}

resource "truenas_nvmet_namespace" "vm_disk" {
  subsys_id   = truenas_nvmet_subsys.vm.id
  device_type = "ZVOL"
  device_path = "zvol/tank/vms/vm1"
}

resource "truenas_nvmet_host" "initiator" {
  hostnqn = "nqn.2014-08.org.nvmexpress:uuid:aaaa-bbbb"
}

resource "truenas_nvmet_host_subsys" "grant" {
  host_id   = truenas_nvmet_host.initiator.id
  subsys_id = truenas_nvmet_subsys.vm.id
}

resource "truenas_nvmet_port_subsys" "expose" {
  port_id   = truenas_nvmet_port.tcp.id
  subsys_id = truenas_nvmet_subsys.vm.id
}

resource "truenas_nvmet_port" "tcp" {
  addr_trtype  = "TCP"
  addr_traddr  = "192.0.2.10" # the box IP NVMe-oF should listen on
  addr_trsvcid = 4420
}

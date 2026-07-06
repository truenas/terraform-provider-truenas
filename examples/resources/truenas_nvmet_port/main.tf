resource "truenas_nvmet_port" "tcp" {
  addr_trtype  = "TCP"
  addr_traddr  = "192.168.1.68"
  addr_trsvcid = 4420
}

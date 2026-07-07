resource "truenas_nvmet_host" "initiator" {
  hostnqn     = "nqn.2014-08.org.nvmexpress:uuid:aaaa-bbbb"
  description = "Proxmox node 1"
}

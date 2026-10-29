resource "truenas_vm" "worker" {
  name       = "worker1"
  memory     = 4096 # MiB (4 GiB)
  vcpus      = 2
  cores      = 2
  bootloader = "UEFI"
  autostart  = true
  running    = true
}

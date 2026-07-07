resource "truenas_vm" "worker" {
  name       = "worker1"
  memory     = 4294967296 # 4 GiB in bytes
  vcpus      = 2
  cores      = 2
  bootloader = "UEFI"
  autostart  = true
  running    = true
}

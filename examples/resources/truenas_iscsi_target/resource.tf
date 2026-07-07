resource "truenas_iscsi_target" "vm" {
  name = "vm-storage"
  mode = "ISCSI"
  groups = [
    {
      portal     = 1
      authmethod = "NONE"
    }
  ]
}

resource "truenas_nvmet_namespace" "disk" {
  subsys_id   = 1
  device_type = "ZVOL"
  device_path = "zvol/tank/vms/vm1"
}

resource "truenas_tunable" "arc_max" {
  type    = "SYSCTL"
  var     = "vm.swappiness"
  value   = "10"
  comment = "Managed by Terraform"
}

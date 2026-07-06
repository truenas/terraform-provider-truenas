resource "truenas_boot_environment" "pre_upgrade" {
  name   = "pre-upgrade-backup"
  source = "26.0.0-BETA.1"
  keep   = true
}

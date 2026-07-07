resource "truenas_system_advanced" "config" {
  motd        = "Managed by Terraform"
  consolemenu = true
  boot_scrub  = 7
}

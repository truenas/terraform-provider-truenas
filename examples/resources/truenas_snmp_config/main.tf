resource "truenas_snmp_config" "config" {
  community = "public"
  location  = "Rack 4"
  contact   = "ops@example.com"
  traps     = false
}

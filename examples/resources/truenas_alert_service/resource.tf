# Email alert service — sends WARNING and above to the ops mailbox.
resource "truenas_alert_service" "ops_mail" {
  name  = "ops-email"
  level = "WARNING"
  attributes = jsonencode({
    type  = "Mail"
    email = "ops@example.com"
  })
}

# Slack alert service — attributes shape depends on the "type" key.
resource "truenas_alert_service" "ops_slack" {
  name    = "ops-slack"
  level   = "CRITICAL"
  enabled = true
  attributes = jsonencode({
    type = "Slack"
    url  = "https://hooks.slack.com/services/T000/B000/XXXX"
  })
}

# SNMP trap receiver.
resource "truenas_alert_service" "snmp" {
  name  = "noc-snmp"
  level = "ERROR"
  attributes = jsonencode({
    type      = "SNMPTrap"
    host      = "192.168.1.10"
    port      = 162
    community = "public"
    v3        = false
  })
}

# Look up an existing alert service by name.
data "truenas_alert_service" "existing" {
  name = "SNMP Trap"
}

output "existing_alert_level" {
  value = data.truenas_alert_service.existing.level
}

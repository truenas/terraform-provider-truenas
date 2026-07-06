resource "truenas_alert_service" "ops_mail" {
  name  = "ops-email"
  level = "WARNING"
  attributes = jsonencode({
    type  = "Mail"
    email = "ops@example.com"
  })
}

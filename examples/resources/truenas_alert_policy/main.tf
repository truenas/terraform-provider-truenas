resource "truenas_alert_policy" "default" {
  classes = jsonencode({
    UPSBatteryLow = {
      level  = "CRITICAL"
      policy = "IMMEDIATELY"
    }
  })
}

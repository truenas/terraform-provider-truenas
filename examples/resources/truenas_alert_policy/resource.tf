# Alert policy — SINGLETON wrapping the system-wide alert class overrides.
# Only declare one truenas_alert_policy per configuration.
#
# classes maps alert class name -> {level, policy}.
#   level:  INFO, NOTICE, WARNING, ERROR, CRITICAL, ALERT, EMERGENCY
#   policy: IMMEDIATELY, HOURLY, DAILY, NEVER
resource "truenas_alert_policy" "default" {
  classes = jsonencode({
    UPSBatteryLow = {
      level  = "CRITICAL"
      policy = "IMMEDIATELY"
    }
    ScrubFinished = {
      level  = "INFO"
      policy = "DAILY"
    }
    SMART = {
      level  = "CRITICAL"
      policy = "IMMEDIATELY"
    }
  })
}

# Read the current policy without managing it.
data "truenas_alert_policy" "current" {}

output "current_alert_classes" {
  value = data.truenas_alert_policy.current.classes
}

# Destroying this resource resets all class overrides to TrueNAS defaults
# (classes = {}).

# Boot environments are created by CLONING an existing one.
# Take a safety snapshot of the running system before an upgrade:
resource "truenas_boot_environment" "pre_upgrade" {
  name   = "pre-upgrade-backup"
  source = "26.0.0-BETA.1" # BE to clone from (usually the active one)
  keep   = true            # survive automatic cleanup
}

# Clone and make it the boot default on next reboot.
# NOTE: activated can only go false -> true. To deactivate, activate a
# different boot environment instead.
resource "truenas_boot_environment" "rollback_target" {
  name      = "known-good"
  source    = "26.0.0-BETA.1"
  activated = false
}

# Look up the currently active BE.
data "truenas_boot_environment" "current" {
  name = "26.0.0-BETA.1"
}

output "current_be" {
  value = {
    dataset    = data.truenas_boot_environment.current.dataset
    active     = data.truenas_boot_environment.current.active
    used_bytes = data.truenas_boot_environment.current.used_bytes
  }
}

# Deleting the resource destroys the BE — refused if it is the active or
# activated one.

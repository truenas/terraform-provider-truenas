# Sysctl tunable.
resource "truenas_tunable" "swappiness" {
  type    = "SYSCTL"
  var     = "vm.swappiness"
  value   = "10"
  comment = "Managed by Terraform"
}

# ZFS module parameter.
resource "truenas_tunable" "l2arc_noprefetch" {
  type  = "ZFS"
  var   = "l2arc_noprefetch"
  value = "0"
}

# UDEV rule tunable; update_initramfs is write-only (applies on change).
resource "truenas_tunable" "udev_example" {
  type             = "UDEV"
  var              = "10-local"
  value            = "ACTION==\"add\", SUBSYSTEM==\"block\", ATTR{queue/scheduler}=\"none\""
  enabled          = true
  update_initramfs = true
}

# Disable a tunable without deleting it.
resource "truenas_tunable" "disabled_example" {
  type    = "SYSCTL"
  var     = "kernel.nmi_watchdog"
  value   = "0"
  enabled = false
}

# Look up a tunable by variable name; orig_value holds the pre-tunable value.
data "truenas_tunable" "swappiness" {
  var = "vm.swappiness"
}

output "swappiness_original" {
  value = data.truenas_tunable.swappiness.orig_value
}

# var and type force replacement when changed.

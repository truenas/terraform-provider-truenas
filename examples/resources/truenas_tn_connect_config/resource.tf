# Singleton resource: manages the one TrueNAS Connect service configuration
# on the system. Terraform destroy only removes it from state; the
# configuration is left in place on TrueNAS.
#
# SAFETY: setting "enabled" to true starts TrueNAS Connect cloud enrollment
# (registers this system with the TrueNAS Connect account service and
# begins periodic heartbeat reporting) — a real external side effect, not a
# mere local configuration change. Leave "enabled" unconfigured (as below)
# to bring this resource under Terraform management purely for its
# read-only status attributes (status, status_reason, tier, ...) without
# asserting any particular enrollment state, or set it explicitly to false
# to ensure TrueNAS Connect stays disabled.
resource "truenas_tn_connect_config" "config" {
}

# Singleton resource: manages the one Enterprise HA failover configuration
# on the system. Terraform destroy only removes it from state; the
# configuration is left in place on TrueNAS.
#
# SAFETY: "disabled" and "master" are direct controls over live HA state,
# not cosmetic settings — see the resource's schema description for the
# full safety notes, including a real controlled failover exercised
# against a disposable Enterprise HA pair during this resource's
# development. Leave both unconfigured (as below) to bring this resource
# under Terraform management purely for its "timeout" field, without
# asserting any particular HA-enabled or mastership state.
resource "truenas_failover_config" "config" {
  timeout = 2
}

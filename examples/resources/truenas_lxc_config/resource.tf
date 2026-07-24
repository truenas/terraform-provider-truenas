# Singleton resource: manages the one LXC service configuration on the
# system (TrueNAS 26.0+ only). Terraform destroy only removes it from
# state; the LXC configuration is left in place as-is.
#
# "preferred_pool" selects the ZFS pool LXC uses for instance/image
# datasets -- changing it on a box where LXC is already in use migrates
# that storage, so treat it with the same care as truenas_docker_config's
# "pool". Omit it (or reference it read-only) to leave LXC's current pool
# untouched.
resource "truenas_lxc_config" "config" {
  v4_network = "172.200.0.0/24"
  v6_network = "fd42:4c58:43ae::/64"
}

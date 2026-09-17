# Singleton resource: manages the one Docker service configuration on the
# system. Terraform destroy only removes it from state; Docker is left
# configured and running as-is.
#
# "nvidia" is writable only on TrueNAS 25.10 and earlier -- TrueNAS
# 26.0+ dropped it from docker.update's accepted fields, so setting it
# explicitly against a 26.0+ target is an apply-time error. Omit it (or
# reference it read-only) when targeting 26.0+.
resource "truenas_docker_config" "config" {
  pool                 = "tank"
  enable_image_updates = true
  cidr_v6              = "fdd0::/64"

  address_pools = [
    {
      base = "172.17.0.0/12"
      size = 24
    },
  ]

  registry_mirrors = [
    {
      url      = "https://mirror.example.internal"
      insecure = false
    },
  ]
}

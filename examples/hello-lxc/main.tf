# Hello-world LXC container on TrueNAS 26.0+ — fully Terraform-native.
#
# Configures the LXC subsystem, resolves a current image version from the
# registry (versions are pruned upstream — never hardcode one), creates the
# container, and starts it.

terraform {
  required_providers {
    truenas = {
      source = "truenas/truenas"
    }
  }
}

variable "truenas_api_key" {
  type      = string
  sensitive = true
}

variable "truenas_endpoint" {
  type        = string
  description = "e.g. wss://truenas.example.com/api/current (TrueNAS 26.0+)"
}

provider "truenas" {
  endpoint = var.truenas_endpoint
  api_key  = var.truenas_api_key
  insecure = true # self-signed certificate
}

# Adjust pool/network below to your box (pool_choices lists valid pools).
# 1. Point the LXC subsystem at a pool.
resource "truenas_lxc_config" "this" {
  preferred_pool = "tank"
  v4_network     = "172.200.0.0/24"
}

# 2. Resolve the newest available image build for Alpine 3.22.
#    The upstream registry prunes old builds, so always take
#    latest_version instead of pinning one.
data "truenas_container_image" "alpine" {
  name = "alpine:3.22:amd64:default"
}

# 3. The container itself — created on tank, started immediately.
resource "truenas_container" "hello" {
  name = "tf-hello-lxc"
  pool = truenas_lxc_config.this.preferred_pool

  image = {
    name    = data.truenas_container_image.alpine.name
    version = data.truenas_container_image.alpine.latest_version
  }

  autostart = false # do not start on boot; Terraform controls state
  running   = true  # start it now

  description = "hello world"
}

output "container" {
  value = {
    id      = truenas_container.hello.id
    dataset = truenas_container.hello.dataset
    network = truenas_container.hello.default_network
    status  = truenas_container.hello.status
  }
}

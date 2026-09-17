# LXC container on TrueNAS 26.0+. Look up a current image version
# through truenas_container_image rather than hardcoding one -- the
# upstream registry (images.linuxcontainers.org) prunes old builds, so a
# pinned version can 404 on download once pruned.
data "truenas_container_image" "alpine" {
  name = "alpine:3.22:amd64:default"
}

resource "truenas_container" "hello" {
  name      = "hello-lxc"
  pool      = "tank"
  autostart = false
  running   = true

  image = {
    name    = "alpine:3.22:amd64:default"
    version = data.truenas_container_image.alpine.latest_version
  }
}

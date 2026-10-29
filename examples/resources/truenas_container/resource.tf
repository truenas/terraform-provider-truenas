# LXC container on TrueNAS 26.0+. Look up a current image version
# through truenas_container_image rather than hardcoding one -- the
# upstream registry (images.linuxcontainers.org) prunes old builds, so a
# pinned version can 404 on download once pruned.
#
# The image is used only at create, and changing it replaces the
# container (TrueNAS cannot change a container's image in place). With
# latest_version, ignore later image changes so a new registry build does
# not replace the container; remove ignore_changes to rebuild on purpose.
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

  lifecycle {
    ignore_changes = [image]
  }
}

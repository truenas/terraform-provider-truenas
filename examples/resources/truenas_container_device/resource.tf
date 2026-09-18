# FILESYSTEM device: bind-mounts a ZFS dataset into an LXC container.
# "source" must resolve under a pool mount point (e.g. /mnt/tank/...);
# always set "target" explicitly -- omitting it falls back to a nonsensical
# API-side default.
resource "truenas_dataset" "hello_data" {
  name = "tank/hello-lxc-data"
}

resource "truenas_container_device" "hello_fs" {
  container = truenas_container.hello.id
  attributes = jsonencode({
    dtype  = "FILESYSTEM"
    source = truenas_dataset.hello_data.mountpoint
    target = "/data"
  })
}

# NIC device: attaches the container to a host bridge.
resource "truenas_container_device" "hello_nic" {
  container = truenas_container.hello.id
  attributes = jsonencode({
    dtype      = "NIC"
    nic_attach = "truenasbr0"
  })
}

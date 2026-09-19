data "truenas_container_image" "alpine" {
  name = "alpine:3.22:amd64:default"
}

output "alpine_latest_version" {
  value = data.truenas_container_image.alpine.latest_version
}

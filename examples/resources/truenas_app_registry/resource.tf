# app.registry.create validates "username"/"password"/"uri" against the
# real container registry endpoint synchronously -- fabricated credentials
# or an unreachable uri are rejected before anything is persisted, so point
# this at a registry you actually control.
resource "truenas_app_registry" "dockerhub" {
  name        = "dockerhub-mirror"
  description = "Docker Hub credentials for pulling private images"
  uri         = "https://index.docker.io/v1/"
  username    = var.registry_username
  password    = var.registry_password # write-only: never stored in state
}

variable "registry_username" {
  type = string
}

variable "registry_password" {
  type      = string
  sensitive = true
}

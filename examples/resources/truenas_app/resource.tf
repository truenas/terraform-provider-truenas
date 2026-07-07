resource "truenas_app" "syncthing" {
  name        = "syncthing"
  catalog_app = "syncthing"
  train       = "stable"

  values = jsonencode({
    network = {
      web_port = 20910
    }
  })
}

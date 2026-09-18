resource "truenas_iscsi_portal" "main" {
  comment = "Default portal"
  listen = [
    {
      ip   = "0.0.0.0"
      port = 3260
    }
  ]
}

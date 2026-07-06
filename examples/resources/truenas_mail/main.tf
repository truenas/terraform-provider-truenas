resource "truenas_mail" "config" {
  fromemail      = "truenas@example.com"
  fromname       = "TrueNAS"
  outgoingserver = "smtp.example.com"
  port           = 587
  security       = "TLS"
  smtp           = true
  user           = "truenas@example.com"
  pass           = "app-password"
}

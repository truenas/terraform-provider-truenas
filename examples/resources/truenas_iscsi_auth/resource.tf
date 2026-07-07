resource "truenas_iscsi_auth" "chap" {
  tag    = 1
  user   = "initiator-user"
  secret = var.chap_secret # 12-16 characters

  discovery_auth = "CHAP"
}

variable "chap_secret" {
  type      = string
  sensitive = true
}

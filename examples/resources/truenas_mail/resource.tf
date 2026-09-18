# System mail configuration — SINGLETON. Declare at most one per config.
resource "truenas_mail" "config" {
  fromemail      = "truenas@example.com"
  fromname       = "TrueNAS"
  outgoingserver = "smtp.example.com"
  port           = 587
  security       = "TLS" # PLAIN, SSL, TLS
  smtp           = true  # enable SMTP authentication
  user           = "truenas@example.com"
  pass           = var.smtp_password # write-only; never stored in state reads
}

variable "smtp_password" {
  type      = string
  sensitive = true
}

# Clear the SMTP user by setting it to an explicit empty string:
#   user = ""
#
# Destroying this resource leaves the mail settings on TrueNAS untouched
# (state is dropped with a warning).

# Read the current mail config (pass is never exposed).
data "truenas_mail" "current" {}

output "mail_server" {
  value = "${data.truenas_mail.current.outgoingserver}:${data.truenas_mail.current.port}"
}

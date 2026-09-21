resource "truenas_ssh_config" "config" {
  tcpport      = 22
  passwordauth = false
  tcpfwd       = true
}

# System Kerberos configuration — SINGLETON. Declare at most one per config.
resource "truenas_kerberos_config" "config" {
  libdefaults_aux = "forwardable = true"
}

resource "truenas_kerberos_realm" "example" {
  realm = "EXAMPLE.COM"
  kdc   = ["kdc1.example.com", "kdc2.example.com"]

  # Left unset: admin_server and kpasswd_server fall back to DNS lookups.
}

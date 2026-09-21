# "file" must be base64-encoded keytab data — filebase64() reads the file
# and encodes it in one step, so do not base64encode() the result again.
# Export a keytab for a service principal with, e.g.:
#   samba-tool domain exportkeytab extra.keytab --principal=host/truenas.example.internal
resource "truenas_kerberos_keytab" "extra" {
  name = "extra-service-keytab"
  file = filebase64("${path.module}/extra.keytab")
}

# Option 1: have TrueNAS generate a fresh RSA key pair.
resource "truenas_keychain_ssh_keypair" "generated" {
  name     = "backup-target-key"
  generate = true
}

# Option 2: import an existing OpenSSH-format private key. Exactly one of
# "generate" or "private_key" must be set.
resource "truenas_keychain_ssh_keypair" "imported" {
  name        = "imported-backup-key"
  private_key = var.ssh_private_key
}

variable "ssh_private_key" {
  type      = string
  sensitive = true
}

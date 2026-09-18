resource "truenas_keychain_ssh_keypair" "backup" {
  name     = "backup-target-key"
  generate = true
}

# "remote_host_key" is not looked up automatically -- obtain it out of band
# (e.g. `ssh-keyscan`, or keychaincredential.remote_ssh_host_key_scan) and
# pass it in via a variable.
resource "truenas_keychain_ssh_connection" "backup_target" {
  name            = "backup-target"
  host            = "backup.example.internal"
  port            = 22
  username        = "replicator"
  private_key_id  = truenas_keychain_ssh_keypair.backup.id
  remote_host_key = var.backup_target_host_key
  connect_timeout = 10
}

variable "backup_target_host_key" {
  type = string
}

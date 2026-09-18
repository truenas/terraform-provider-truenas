resource "truenas_api_key" "automation" {
  name       = "terraform-automation"
  username   = "admin"
  expires_at = "2030-01-01T00:00:00Z" # omit for a key that never expires
}

# The plaintext key is only ever available in the create response; TrueNAS
# never returns it again on read. Capture it once, then treat this output
# as sensitive.
output "automation_key" {
  value     = truenas_api_key.automation.key
  sensitive = true
}

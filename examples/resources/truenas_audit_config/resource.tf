# Singleton resource: manages the one audit configuration on the system.
# Terraform destroy only removes it from state; the configuration is left
# in place on TrueNAS.
resource "truenas_audit_config" "config" {
  retention           = 30
  reservation         = 0
  quota               = 20
  quota_fill_warning  = 70
  quota_fill_critical = 90
}

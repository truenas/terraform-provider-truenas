resource "truenas_privilege" "backup_operators" {
  name         = "Backup Operators"
  local_groups = [1000] # GIDs of local groups
  roles        = ["READONLY_ADMIN", "SHARING_READ"]
  web_shell    = false
}

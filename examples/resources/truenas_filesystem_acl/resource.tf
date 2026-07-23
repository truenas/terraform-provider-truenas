# NOTE: `terraform destroy` strips the ACL back to a trivial (mode-only)
# one via filesystem.setacl's stripacl option -- it does not merely forget
# the resource. See the resource description for the full delete-semantics
# rationale.
resource "truenas_filesystem_acl" "shared_dir" {
  path = "/mnt/tank/shared"

  entries = jsonencode([
    {
      tag   = "owner@"
      type  = "ALLOW"
      perms = { BASIC = "FULL_CONTROL" }
      flags = { BASIC = "INHERIT" }
    },
    {
      tag   = "group@"
      type  = "ALLOW"
      perms = { BASIC = "MODIFY" }
      flags = { BASIC = "INHERIT" }
    },
    {
      tag   = "everyone@"
      type  = "ALLOW"
      perms = { BASIC = "READ" }
      flags = { BASIC = "INHERIT" }
    },
  ])

  uid = 1000
  gid = 1000
}

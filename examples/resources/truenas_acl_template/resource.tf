resource "truenas_acl_template" "nfs4_readonly" {
  name    = "nfs4-readonly"
  acltype = "NFS4"
  comment = "Read-only access for group@ and everyone@"

  acl = jsonencode([
    {
      tag   = "owner@"
      type  = "ALLOW"
      perms = { BASIC = "FULL_CONTROL" }
      flags = { BASIC = "INHERIT" }
    },
    {
      tag   = "group@"
      type  = "ALLOW"
      perms = { BASIC = "READ" }
      flags = { BASIC = "INHERIT" }
    },
    {
      tag   = "everyone@"
      type  = "ALLOW"
      perms = { BASIC = "READ" }
      flags = { BASIC = "INHERIT" }
    },
  ])
}

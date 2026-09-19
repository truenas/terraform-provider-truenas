# NOTE: `terraform destroy` strips the ACL back to a trivial (mode-only)
# one via filesystem.setacl's stripacl option -- it does not merely forget
# the resource. See the resource description for the full delete-semantics
# rationale.
resource "truenas_dataset" "shared_dir" {
  name = "tank/shared"
}

resource "truenas_filesystem_acl" "shared_dir" {
  # Reference the dataset's mountpoint rather than hardcoding the path (e.g.
  # "/mnt/tank/shared"). The reference gives Terraform a dependency edge so it
  # creates the dataset before setting the ACL; a hardcoded path has no such
  # edge, so Terraform may run filesystem.setacl before the dataset exists and
  # fail with "path not found". Referencing the pool name is not enough -- it
  # is the DATASET that creates this path.
  path = truenas_dataset.shared_dir.mountpoint

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

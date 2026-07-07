resource "truenas_nfs_config" "config" {
  protocols         = ["NFSV3", "NFSV4"]
  servers           = 8
  userd_manage_gids = false
}

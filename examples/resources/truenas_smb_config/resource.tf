resource "truenas_smb_config" "config" {
  netbiosname      = "truenas"
  workgroup        = "WORKGROUP"
  description      = "TrueNAS Server"
  minimum_protocol = "SMB2"
}

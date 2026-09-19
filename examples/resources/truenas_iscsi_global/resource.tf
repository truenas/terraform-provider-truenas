resource "truenas_iscsi_global" "config" {
  basename    = "iqn.2005-10.org.freenas.ctl"
  listen_port = 3260
  alua        = false
}

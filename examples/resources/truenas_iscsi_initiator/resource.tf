resource "truenas_iscsi_initiator" "all" {
  comment    = "Allow all initiators"
  initiators = []
}

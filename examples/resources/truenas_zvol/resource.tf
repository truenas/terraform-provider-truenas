resource "truenas_zvol" "iso" {
  name    = "tank/iso"
  volsize = 10737418240 # 10 GiB
}

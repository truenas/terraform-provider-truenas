# Import an existing pool rather than creating one from scratch.
# Pool creation requires knowing exact disk identifiers.
# Run: terraform import truenas_pool.tank tank

resource "truenas_pool" "tank" {
  name     = "tank"
  autotrim = false
  topology {
    data = [
      { type = "MIRROR", disks = ["sda", "sdb"] },
      { type = "MIRROR", disks = ["sdc", "sdd"] },
    ]
  }
}

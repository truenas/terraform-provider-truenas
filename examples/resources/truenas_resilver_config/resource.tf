# Pool resilver priority schedule — SINGLETON. Declare at most one per config.
resource "truenas_resilver_config" "config" {
  enabled = true
  begin   = "18:00"
  end     = "09:00" # rolls over midnight
  weekday = [1, 2, 3, 4, 5, 6, 7]
}

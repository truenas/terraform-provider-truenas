# Preferred NTP server.
resource "truenas_ntp_server" "primary" {
  address = "time.cloudflare.com"
  iburst  = true
  prefer  = true
}

# Secondary with custom polling interval (log2 seconds: 6 = 64s, 10 = 1024s).
resource "truenas_ntp_server" "secondary" {
  address = "pool.ntp.org"
  iburst  = true
  minpoll = 6
  maxpoll = 10
}

# force is a write-only validation bypass — use when the server is
# temporarily unreachable at apply time.
resource "truenas_ntp_server" "internal" {
  address = "ntp.internal.example.com"
  force   = true
}

# Look up an existing NTP server by address.
data "truenas_ntp_server" "debian_pool" {
  address = "0.debian.pool.ntp.org"
}

output "debian_pool_id" {
  value = data.truenas_ntp_server.debian_pool.id
}

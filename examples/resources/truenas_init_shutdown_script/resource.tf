resource "truenas_init_shutdown_script" "mount_check" {
  type    = "SCRIPT"
  script  = "/usr/local/bin/mount-check.sh"
  when    = "POSTINIT"
  enabled = true
  timeout = 15
  comment = "Verify external mounts are present after boot"
}

# Old-state upgrade probe config (CRUD resources only; attrs valid on both
# v1.5.24 and current). Driven by run-upgrade.sh.

terraform {
  required_providers {
    truenas = { source = "truenas/truenas" }
  }
}

variable "endpoint" { type = string }
variable "api_key" {
  type      = string
  sensitive = true
}
variable "pool" { type = string }
variable "suffix" { type = string }
variable "probe_password" {
  type      = string
  sensitive = true
}

provider "truenas" {
  endpoint = var.endpoint
  api_key  = var.api_key
  insecure = true
}

# --- storage ---------------------------------------------------------------
resource "truenas_dataset" "parent" {
  name        = "${var.pool}/tfupg-${var.suffix}"
  compression = "lz4"
  atime       = "OFF"
  comments    = "upgrade probe"
  acltype     = "nfsv4"       # pool default is POSIX on 25.10; NFSv4 ACL entries need this
  aclmode     = "PASSTHROUGH"
}
resource "truenas_dataset" "child" {
  name       = "${var.pool}/tfupg-${var.suffix}/child"
  recordsize = "1M"
  sync       = "DISABLED"
  depends_on = [truenas_dataset.parent]
}
resource "truenas_zvol" "z" {
  name       = "${var.pool}/tfupg-${var.suffix}/zvol"
  volsize    = 33554432
  sparse     = true
  depends_on = [truenas_dataset.parent]
}
resource "truenas_snapshot" "snap" {
  dataset = truenas_dataset.child.name
  name    = "tfupg-${var.suffix}"
}
resource "truenas_periodic_snapshot_task" "p" {
  dataset        = truenas_dataset.parent.name
  recursive      = true
  lifetime_value = 2
  lifetime_unit  = "WEEK"
  naming_schema  = "auto-%Y-%m-%d_%H-%M"
  enabled        = false
  schedule = {
    minute = "0"
    hour   = "0"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}

# --- accounts --------------------------------------------------------------
resource "truenas_group" "g" {
  name = "tfupg${var.suffix}"
}
resource "truenas_user" "u" {
  username     = "tfupg${var.suffix}"
  full_name    = "Upgrade Probe"
  group_create = true
  password     = var.probe_password
  smb          = true
  shell        = "/usr/bin/bash"
  groups       = [truenas_group.g.id]
}
resource "truenas_api_key" "k" {
  name     = "tfupg-${var.suffix}"
  username = "root"
}
resource "truenas_privilege" "pr" {
  name         = "tfupg${var.suffix}"
  local_groups = [truenas_group.g.gid]
  roles        = ["READONLY_ADMIN"]
  web_shell    = false
}

# --- sharing ---------------------------------------------------------------
resource "truenas_smb_share" "s" {
  name      = "tfupg${var.suffix}"
  path      = truenas_dataset.parent.mountpoint
  comment   = "upgrade probe"
  browsable = true
  ro        = false
}
resource "truenas_nfs_share" "n" {
  path     = truenas_dataset.child.mountpoint
  comment  = "upgrade probe"
  networks = ["192.0.2.0/24"]
  ro       = true
}
resource "truenas_acl_template" "acl" {
  name    = "tfupg${var.suffix}"
  acltype = "NFS4"
  comment = "upgrade probe"
  acl = jsonencode([
    { tag = "owner@", type = "ALLOW", perms = { BASIC = "FULL_CONTROL" }, flags = { BASIC = "INHERIT" } },
    { tag = "group@", type = "ALLOW", perms = { BASIC = "MODIFY" }, flags = { BASIC = "INHERIT" } },
  ])
}

# --- iSCSI -----------------------------------------------------------------
resource "truenas_iscsi_target" "t" {
  name = "tfupg${var.suffix}"
}
resource "truenas_iscsi_extent" "e" {
  name = "tfupg${var.suffix}"
  type = "DISK"
  disk = "zvol/${truenas_zvol.z.name}"
}
resource "truenas_iscsi_initiator" "i" {
  comment = "upgrade probe ${var.suffix}"
}

# --- tasks / misc ----------------------------------------------------------
resource "truenas_cronjob" "c" {
  command     = "echo tfupg-${var.suffix}"
  user        = "root"
  description = "upgrade probe ${var.suffix}"
  enabled     = false
  schedule = {
    minute = "0"
    hour   = "2"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}
resource "truenas_init_shutdown_script" "init" {
  type    = "COMMAND"
  command = "echo tfupg-${var.suffix}"
  when    = "POSTINIT"
  enabled = false
  comment = "upgrade probe"
  timeout = 10
}
resource "truenas_tunable" "tun" {
  type    = "SYSCTL"
  var     = "kernel.hung_task_timeout_secs"
  value   = "121"
  comment = "upgrade probe ${var.suffix}"
  enabled = false
}
resource "truenas_static_route" "r" {
  destination = "198.51.100.0/24"
  gateway     = "192.0.2.1"
  description = "upgrade probe ${var.suffix}"
}
resource "truenas_alert_service" "a" {
  name    = "tfupg${var.suffix}"
  level   = "WARNING"
  enabled = false
  attributes = jsonencode({
    type = "Slack"
    url  = "https://hooks.slack.invalid/tfupg"
  })
}
resource "truenas_cloudsync_credentials" "cred" {
  name = "tfupg${var.suffix}"
  provider_config = jsonencode({
    type              = "S3"
    access_key_id     = "AKIATFUPG"
    secret_access_key = "tfupg-placeholder"
    endpoint          = "http://192.0.2.9:9000"
    region            = "us-east-1"
    skip_region       = false
    signatures_v2     = false
  })
}
resource "truenas_keychain_ssh_keypair" "kp" {
  name     = "tfupg${var.suffix}"
  generate = true
}
resource "truenas_certificate" "csr" {
  name        = "tfupg${var.suffix}"
  create_type = "CERTIFICATE_CREATE_CSR"
  key_type    = "RSA"
  key_length  = 2048
  common      = "tfupg.example.com"
  san         = ["tfupg.example.com"]
}

# --- resources whose attributes went write-only in 1.6.0 (F5 blast check) ---
resource "truenas_filesystem_permissions" "perm" {
  path      = truenas_dataset.child.mountpoint
  mode      = "0750"
  uid       = 0
  gid       = 0
  recursive = false
  traverse  = false
}
resource "truenas_filesystem_acl" "acl" {
  path      = truenas_dataset.parent.mountpoint
  uid       = 0
  gid       = 0
  recursive = false
  entries = jsonencode([
    { tag = "owner@", type = "ALLOW", perms = { BASIC = "FULL_CONTROL" }, flags = { BASIC = "INHERIT" } },
    { tag = "group@", type = "ALLOW", perms = { BASIC = "MODIFY" }, flags = { BASIC = "INHERIT" } },
    { tag = "everyone@", type = "ALLOW", perms = { BASIC = "READ" }, flags = { BASIC = "NOINHERIT" } },
  ])
}
resource "truenas_rsync_task" "rs" {
  path           = truenas_dataset.child.mountpoint
  user           = "root"
  mode           = "MODULE"
  remotehost     = "192.0.2.10"
  remotemodule   = "tfupg"
  direction      = "PUSH"
  enabled        = false
  validate_rpath = false
  ssh_keyscan    = false
  schedule = {
    minute = "0"
    hour   = "3"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}

# --- never-probed CRUD resources 1.5.24 supported ---------------------------
resource "truenas_reporting_exporter" "exp" {
  name    = "tfupg${var.suffix}"
  enabled = false
  attributes = {
    destination_ip   = "192.0.2.50"
    destination_port = 2004
    namespace        = "tfupg"
    prefix           = "tfupgprefix"
    update_every     = 7
  }
}
resource "truenas_keychain_ssh_keypair" "kp2" {
  name     = "tfupg${var.suffix}-kp"
  generate = true
}
resource "truenas_keychain_ssh_connection" "conn" {
  name            = "tfupg${var.suffix}-conn"
  host            = "192.0.2.10"
  username        = "tfupguser"
  private_key_id  = truenas_keychain_ssh_keypair.kp2.id
  remote_host_key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
  connect_timeout = 15
}
resource "truenas_vm" "vm" {
  name   = "tfupg${var.suffix}vm"
  memory = 512
  vcpus  = 1
}

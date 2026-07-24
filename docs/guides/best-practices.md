---
page_title: "Best Practices"
subcategory: "Guides"
description: |-
  Recommended patterns for running the TrueNAS provider safely in production:
  authentication, secrets, singletons, imports, versioning, and CI.
---

# Best Practices

Recommendations for using this provider against a real TrueNAS box. They come
from live testing against TrueNAS 25.10 and 26.0 and from how the TrueNAS
middleware actually behaves, not from generic Terraform advice.

## Authentication

**Use an API key, not a password.** Create a dedicated key for Terraform
(**user icon → API Keys → Add** in the UI, or `midclt call api_key.create`)
so it can be revoked independently of any user account:

```hcl
provider "truenas" {
  endpoint = "wss://truenas.example.com/api/current"
  api_key  = var.truenas_api_key
  username = "svc_terraform" # key owner; enables SCRAM-SHA-512 on 26.0+
}
```

**Set `username` with your API key.** On TrueNAS 26.0+ this switches
authentication to SCRAM-SHA-512: the raw key never crosses the wire (only a
proof of possession does) and the server proves it knows the key too, so a
middlebox cannot harvest credentials even if TLS is intercepted. On older
servers the username is ignored and the plain key login is used. SCRAM adds
around a second per session (500,000 PBKDF2 iterations by design).

**Mind the login rate limit.** TrueNAS allows roughly 20 authentications per
minute per source IP, and every `terraform plan` or `apply` opens one
session. The provider retries with backoff when it hits the limit, but you
can avoid the stall entirely:

- Prefer one workspace per box over many small ones that each dial in.
- In CI, run plans against the same box sequentially, not fan-out.
- Don't wrap Terraform in shell loops that reconnect per resource.

## Secrets

All password and key attributes (`truenas_user.password`,
`truenas_iscsi_auth.secret`/`peersecret`, `truenas_mail.pass`,
`truenas_nvmet_host.dhchap_key`/`dhchap_ctrl_key`,
`truenas_ups_config.monpwd`, `truenas_snmp_config.v3_password`/
`v3_privpassphrase`, `truenas_system_advanced.sed_passwd`) are
**write-only**: Terraform 1.11+ never stores them in state or plan files.
Source them from ephemeral variables:

```hcl
variable "user_password" {
  type      = string
  ephemeral = true
}

resource "truenas_user" "svc" {
  username  = "svcacct"
  full_name = "Service account"
  password  = var.user_password
}
```

Two consequences to plan around:

- **State files stay clean.** You can share state without leaking secrets,
  though treating state as sensitive is still wise.
- **No drift detection on secrets.** Because the value never lands in
  state, editing only the secret produces an empty plan. To rotate a
  secret, change it together with a tracked attribute, or taint the
  resource (`terraform apply -replace=truenas_user.svc`), or keep a
  `comment`/`description` field that you bump alongside rotations.

## Singleton resources adopt, they don't create

Service and system configuration resources (`truenas_nfs_config`,
`truenas_smb_config`, `truenas_mail`, `truenas_system_advanced`, and the
other `*_config` singletons) manage configuration that always exists on the
box. Terraform "create" **adopts and updates** the live config;
`terraform destroy` only forgets it from state and warns — it never resets
the box. Practical rules:

- Import them instead of creating where possible
  (`terraform import truenas_nfs_config.this nfs_config` — any import ID is
  accepted), so the first plan shows you the diff against reality.
- Let exactly **one** workspace own each box's singletons. Two workspaces
  both managing `truenas_system_advanced` will fight forever.
- Set only the attributes you mean to manage. Unset attributes follow the
  server, so a minimal config makes small, reviewable diffs.

## Resources that can cut off access

A few resources change things your own connection may depend on:

- `truenas_network_interface` follows the TrueNAS staged
  commit/checkin flow, so a bad address rolls back automatically instead of
  stranding the box — but keep console access available anyway.
- `truenas_system_general` manages the UI ports and certificate the
  provider itself connects through. Changing `ui_port`/`ui_httpsport` or
  the certificate can break the endpoint in your provider block mid-apply.

Apply changes to these from a context where you can reach the box console
(physical, IPMI, or hypervisor) if something goes wrong.

## iSCSI specifics

- **Portal listen addresses must be statically configured on the box.**
  TrueNAS rejects a portal listen IP that is only held via DHCP
  (`IP ... not configured on this system`). Give storage interfaces static
  addresses before managing portals.
- **Extent deletion preserves data.** Destroying a `truenas_iscsi_extent`
  removes the extent object but keeps the backing zvol or file. Destroy the
  backing `truenas_zvol` too if you want the space back.

## Version compatibility

The provider works against TrueNAS 25.10 and 26.0 and detects the server
release at runtime. Fields that exist only on newer releases fail fast with
a clear error instead of the middleware's generic "Extra inputs are not
permitted" — for example `truenas_nvmet_host.description` requires 26.0.
When one configuration targets a mixed fleet, stick to the attributes the
oldest release supports.

## Imports and adoption

Every resource is importable. Numeric-ID resources import by ID
(`terraform import truenas_dataset.data tank/data` for path-identified ones,
`terraform import truenas_iscsi_extent.disk1 4` for numeric); singletons
accept any import ID. When bringing an existing box under management:

1. Import the pools' datasets and shares you intend to manage.
2. Run `terraform plan` and reconcile the diff — either adjust the config to
   match the box, or accept that the first apply normalizes the box to the
   config.
3. Leave everything else out of state. Terraform ignores what it doesn't
   manage; you don't need to import the whole box.

Prefer **data sources** over imports when you only need to reference an
object (for example, looking up a pool or an existing user's UID) — data
sources never mutate and can't accidentally destroy anything.

## Apps and VMs

- Pin `truenas_app` versions explicitly. A version change in config is what
  triggers an upgrade; leaving it unpinned invites surprise upgrades on
  unrelated applies.
- App installs pull container images, so first applies can take minutes and
  need working DNS/egress from the box.
- `truenas_vm` display devices (SPICE) need a password and distinct ports
  per VM.

## Replication and cloud sync

- `truenas_cloudsync_credentials` separates the provider type from its
  attributes object; keep the secrets inside `attributes` sourced from
  variables, not literals.
- Replication tasks need a snapshot naming source: link them to a
  `truenas_periodic_snapshot` task or set `naming_schema` explicitly. A
  schema must include time components (`%Y%m%d%H%M` style) — TrueNAS
  rejects date-only schemas.
- `truenas_replication_task`'s `transport` defaults to `LOCAL`; set it to
  `SSH` for remote replication, which requires `ssh_credentials` (the id of
  a `truenas_keychain_ssh_connection`) — the two must agree, `SSH` without
  credentials and `LOCAL` with them are both config-time errors.
  `compression`/`speed_limit` are accepted only under `SSH`.
- Passwordless `sudo` on the remote system's SSH user is required for real
  ZFS transfers over `SSH` transport unless that user is `root`: without it
  `replication.run` fails partway through with a destination-unmount
  permission error (verified live), even though the task itself creates and
  updates without complaint. Set `sudo = true` and configure passwordless
  sudo for the remote user, or use a `root`-owned SSH credential.

```hcl
resource "truenas_keychain_ssh_keypair" "repl" {
  name     = "replication-key"
  generate = true
}

resource "truenas_keychain_ssh_connection" "backup_target" {
  name            = "backup-target"
  host            = "backup.example.com"
  username        = "replication_user"
  private_key_id  = truenas_keychain_ssh_keypair.repl.id
  remote_host_key = var.backup_target_host_key # see note below
}

resource "truenas_replication_task" "offsite" {
  name             = "offsite-backup"
  direction        = "PUSH"
  transport        = "SSH"
  ssh_credentials  = truenas_keychain_ssh_connection.backup_target.id
  sudo             = true # remote user must have passwordless sudo, or be root
  compression      = "LZ4"
  source_datasets  = ["tank/data"]
  target_dataset   = "backup/tank-data"
  recursive        = true
  auto             = false
  retention_policy = "SOURCE"

  also_include_naming_schema = ["auto-%Y-%m-%d_%H-%M"]
}
```

  (The remote host key isn't looked up automatically — obtain it out of
  band, e.g. via `keychaincredential.remote_ssh_host_key_scan` in a
  provisioning script, and pass it in as shown; this provider doesn't call
  that method itself.)

## Running in CI

- Serialize applies to the same box (state locking plus a single runner
  queue). The middleware serializes many operations server-side anyway;
  parallel workspaces mostly buy you rate-limit stalls.
- The provider retries transient read failures and reconnects dropped
  WebSocket sessions automatically, so short middleware restarts (e.g.
  during an update) usually surface as a slow plan rather than a failed one.
  A failed *write* is never retried — rerun the pipeline after checking
  the box.
- Keep the API key in your CI secret store and pass it via the
  `TRUENAS_API_KEY` environment variable — the provider reads it directly,
  so the key never appears in configuration. `TRUENAS_ENDPOINT` works the
  same way. Never commit either.

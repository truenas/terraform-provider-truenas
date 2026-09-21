# Terraform Provider for TrueNAS

A Terraform provider that manages [TrueNAS](https://www.truenas.com/truenas-scale/)
through its WebSocket API. Built on the
[Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework);
no REST calls, no external client dependency — a self-contained JSON-RPC 2.0
client lives inside the provider.

## What it manages

85 resources (plus three data-source-only namespaces, `truenas_docker_network`,
`truenas_container_image`, and `truenas_enclosure` — see below), 84 with a
matching data source (`truenas_enclosure_label` is the one resource with no
matching data source — see Enterprise & HA):

| Area | Resources |
|------|-----------|
| **Storage** | `truenas_pool`, `truenas_dataset`, `truenas_zvol`, `truenas_snapshot`, `truenas_periodic_snapshot_task`, `truenas_scrub_task`, `truenas_resilver_config` |
| **File shares** | `truenas_nfs_share`, `truenas_smb_share`, `truenas_webshare` (TrueNAS 26.0+ only), `truenas_webshare_config` (Webshare service singleton — bind IPs/search/passkey/groups; TrueNAS 26.0+ only) |
| **Filesystem permissions & ACLs** | `truenas_filesystem_permissions`, `truenas_filesystem_acl`, `truenas_acl_template` |
| **iSCSI** | `truenas_iscsi_target`, `truenas_iscsi_extent`, `truenas_iscsi_initiator`, `truenas_iscsi_portal`, `truenas_iscsi_targetextent`, `truenas_iscsi_auth`, `truenas_iscsi_global` |
| **NVMe-oF** | `truenas_nvmet_subsys`, `truenas_nvmet_port`, `truenas_nvmet_namespace`, `truenas_nvmet_host`, `truenas_nvmet_host_subsys`, `truenas_nvmet_port_subsys`, `truenas_nvmet_global` |
| **Accounts** | `truenas_user`, `truenas_group` |
| **Access management** | `truenas_api_key`, `truenas_privilege`, `truenas_twofactor_auth` |
| **Certificates & ACME** | `truenas_certificate`, `truenas_acme_dns_authenticator` |
| **Directory services & Kerberos** | `truenas_directoryservices` (Active Directory, LDAP, and IPA join; explicit AD idmap configuration), `truenas_kerberos_config`, `truenas_kerberos_realm`, `truenas_kerberos_keytab` |
| **Apps, containers & VMs** | `truenas_app`, `truenas_vm`, `truenas_vm_device`, `truenas_docker_config` (Docker service singleton), `truenas_app_registry` (private container registry credentials), `truenas_catalog_config` (app catalog trains singleton), `truenas_lxc_config` (LXC service singleton — pool/bridge/network CIDRs; TrueNAS 26.0+ only), `truenas_container` (LXC container lifecycle — create/start/stop/delete; TrueNAS 26.0+ only), `truenas_container_device` (per-container device attachment — FILESYSTEM/NIC/USB; TrueNAS 26.0+ only); `truenas_docker_network` and `truenas_container_image` (LXC image registry lookup) are **data source only** — Docker networks are managed by Docker itself, not by TrueNAS's config surface, and container images live in an upstream registry, not TrueNAS-managed state |
| **Replication & sync** | `truenas_replication_task` (local push and remote SSH transport), `truenas_cloudsync_task`, `truenas_cloudsync_credentials`, `truenas_cloud_backup`, `truenas_rsync_task` |
| **Keychain** | `truenas_keychain_ssh_keypair`, `truenas_keychain_ssh_connection` |
| **Scheduled tasks** | `truenas_cronjob`, `truenas_init_shutdown_script` |
| **Network** | `truenas_network_interface`, `truenas_static_route`, `truenas_network_config` |
| **Services** | `truenas_service`, plus per-service configuration: `truenas_ssh_config`, `truenas_ftp_config`, `truenas_snmp_config`, `truenas_ups_config`, `truenas_smb_config`, `truenas_nfs_config` |
| **Alerts** | `truenas_alert_service`, `truenas_alert_policy` |
| **System** | `truenas_boot_environment`, `truenas_tunable`, `truenas_ntp_server`, `truenas_mail`, `truenas_system_general`, `truenas_system_advanced`, `truenas_system_dataset`, `truenas_replication_config`, `truenas_audit_config`, `truenas_reporting_exporter`, `truenas_tn_connect_config` (TrueNAS Connect service singleton — enrollment status only; SAFETY: enabling starts real cloud enrollment, see the resource's own docs) |
| **Enterprise & HA** | `truenas_failover_config` (HA controller pair singleton — disabled/master/timeout; datasource adds status/node/disabled_reasons), `truenas_ipmi_lan` (per-channel BMC LAN configuration), `truenas_enclosure_label` (the one mutable field an enclosure exposes; `truenas_enclosure` is a **data source only** lookup of enclosure hardware), `truenas_truecommand_config` (TrueCommand connection singleton — enrollment (`enabled=true`) untested, no TC instance available), `truenas_vmware` (VMware snapshot coordination; **create validates against a real vCenter/ESXi endpoint**, so its acceptance test is a documented, permanent skip — schema and units only) |

Both block-storage stacks are expressible end-to-end in HCL: iSCSI
(portal → target → extent → LUN association → CHAP auth) and NVMe-oF
(port → subsystem → namespace → host grant → port binding). Remote
replication is expressible end-to-end too: `truenas_keychain_ssh_keypair` →
`truenas_keychain_ssh_connection` → `truenas_replication_task` (SSH
transport).

Working examples for every resource are under [`examples/resources/`](examples/resources/).

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.11 (write-only secret attributes)
- [Go](https://go.dev/doc/install) >= 1.25 (to build from source)
- TrueNAS 25.04+ (the provider speaks the versioned JSON-RPC 2.0 API
  at `/api/current`, introduced in 25.04; older releases only offer the
  legacy WebSocket endpoint and cannot connect)

### Tested TrueNAS versions

The full acceptance suite runs against live TrueNAS boxes on these releases:

| TrueNAS release | Status |
|---|---|
| 26.0 | Fully tested |
| 25.10 | Fully tested. `truenas_nvmet_host.description` is 26.0+ only; the provider rejects it with a clear error on older releases |
| 25.04 | Connects (has `/api/current`) but untested — schema drift is possible |

The provider detects the server release at runtime (`system.version_short`)
and gates version-specific fields, so a single configuration can target
either release as long as it avoids the newer fields.

The Enterprise & HA resources (`truenas_failover_config`, `truenas_ipmi_lan`,
`truenas_enclosure`/`truenas_enclosure_label`, `truenas_truecommand_config`,
`truenas_vmware`) were additionally verified live on a disposable TrueNAS
25.10.4 Enterprise HA controller pair, including one real controlled
failover exercise — see TESTING.md's HA / Enterprise test environment
section.

## Initial setup

### 1. Install the provider

The provider is published to the Terraform Registry as
[`truenas/truenas`](https://registry.terraform.io/providers/truenas/truenas).
Declare it (see [step 3](#3-configure-the-provider)) and `terraform init`
downloads and signature-verifies it — no build step needed.

#### Build from source (optional)

For contributors, or an air-gapped install, build and install into your local
plugin directory instead:

```sh
git clone https://github.com/truenas/terraform-provider-truenas
cd terraform-provider-truenas
make install
```

This builds the binary and copies it to
`~/.terraform.d/plugins/registry.terraform.io/truenas/truenas/<version>/<os>_<arch>/`
(default version `0.1.0`; override with `make install VERSION=1.0.0`).
Terraform prefers this local copy over the Registry, so pin a matching
`version` in `required_providers`.

For provider development without reinstalling on every build, use a
[dev override](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides)
in `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "truenas/truenas" = "/path/to/terraform-provider-truenas"
  }
  direct {}
}
```

Then `make build` is enough; skip `terraform init` for the overridden provider.

### 2. Create a TrueNAS API key

In the TrueNAS UI: **user icon → API Keys → Add**, or with `midclt` on the box:

```sh
midclt call api_key.create '{"name": "terraform", "username": "root"}'
```

Store the key outside your Terraform files (environment variable or a secrets
manager). Username/password auth also works but an API key is preferred.

On TrueNAS 26.0+, set `username` (the key owner) alongside `api_key` and the
provider authenticates via SCRAM-SHA-512: a challenge-response exchange
with mutual verification where the raw key never crosses the wire. Older
servers, or an api_key without username, use the plain key login.

### 3. Configure the provider

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 1.0"
    }
  }
}

provider "truenas" {
  endpoint = "wss://truenas.example.com/api/current"
  api_key  = var.truenas_api_key
  username = "svc_terraform" # key owner; enables SCRAM-SHA-512 on 26.0+

  # TLS: pick one (or neither, for a system-trusted certificate)
  insecure = true                # skip verification (self-signed certs)
  # ca_cert = "/path/to/ca.pem"  # or trust a specific CA
}

variable "truenas_api_key" {
  type      = string
  sensitive = true
}
```

Every provider argument can come from the environment instead:

| Argument | Environment variable |
|----------|---------------------|
| `endpoint` | `TRUENAS_ENDPOINT` |
| `api_key`  | `TRUENAS_API_KEY` |
| `username` | `TRUENAS_USERNAME` |
| `password` | `TRUENAS_PASSWORD` |

Exactly one auth method is required: `api_key` **or** `username`+`password`.
(`username` alongside `api_key` is not a second method — it names the key
owner so SCRAM can be used.)

### 4. First resources

```hcl
resource "truenas_dataset" "media" {
  name = "tank/media"
}

resource "truenas_nfs_share" "media" {
  path    = "/mnt/tank/media"
  comment = "Media library"
}

data "truenas_pool" "tank" {
  name = "tank"
}

output "pool_free_bytes" {
  value = data.truenas_pool.tank.free
}
```

```sh
export TRUENAS_API_KEY="1-xxxx..."
terraform init   # skip if using dev_overrides
terraform plan
terraform apply
```

## Design notes

- **Sync vs. jobs.** TrueNAS methods are either synchronous calls or
  long-running jobs. The client exposes `Call` and `CallJob`; job progress is
  tracked over the same WebSocket subscription until success or failure.
- **Singletons.** System-wide configs (`truenas_mail`,
  `truenas_ssh_config`, `truenas_system_general`, `truenas_iscsi_global`, …)
  have no create/delete on TrueNAS. Terraform "create" adopts and updates the
  config; "destroy" removes it from state and leaves the settings in place
  (with a warning).
- **Write-only secrets.** Passwords, CHAP secrets, and DH-CHAP keys are marked
  `Sensitive`, are never read back from the API into state, and never appear
  in data sources. After `terraform import`, the first plan re-proposes them.
- **Free-form objects.** App config values, VM device attributes, cloud-sync
  provider settings, and alert-service attributes are JSON documents — write
  them with `jsonencode({...})`. The provider validates the JSON and performs
  drift-aware reads that ignore server-added defaults.
- **Staged network changes.** `truenas_network_interface` follows the TrueNAS
  commit/checkin protocol: changes are staged, committed with automatic
  rollback armed, and confirmed only if connectivity survives.
- **Directory services never leave a domain on destroy.** `truenas_directoryservices`
  supports joining Active Directory, LDAP, or IPA (`service_type`), plus
  explicit UID/GID idmap configuration on the Active Directory block.
  Destroying the resource disables directory services locally; it never
  calls the leave-domain/unjoin API, so an AD computer account or IPA host
  entry stays on the domain controller — leaving requires an administrator
  credential this provider does not assume is available at destroy time.
- **Filesystem permissions and ACLs are path-keyed, not TrueNAS-tracked
  objects.** `truenas_filesystem_permissions` and `truenas_filesystem_acl`
  wrap `filesystem.setperm`/`filesystem.setacl` on an existing path; the
  resource ID is the path itself. `terraform destroy` on
  `truenas_filesystem_permissions` never reverts mode/uid/gid — it only
  forgets the resource, with a warning. `truenas_filesystem_acl` differs:
  destroy strips the ACL back to a trivial mode-only one
  (`options.stripacl`), since that conversion was confirmed clean live on
  both NFS4 and POSIX1E. `truenas_acl_template` manages reusable named ACL
  templates and never touches TrueNAS's built-in templates unless one is
  explicitly imported.
- **Certificates and remote replication over SSH.** `truenas_certificate`
  supports importing a cert+key pair, generating a CSR on-box, importing an
  externally-generated CSR, or ACME. ACME issuance is live-tested end to end
  (a full DNS-01 order against a real ACME CA); automated renewal polling is
  not yet exercised. `truenas_keychain_ssh_keypair`
  and `truenas_keychain_ssh_connection` store SSH credentials in the
  TrueNAS keychain; wiring a connection's ID into `truenas_replication_task`'s
  `ssh_credentials` unblocks its SSH transport for remote (not just local
  push) replication.
- **Two-factor auth is a safety-sensitive singleton.**
  `truenas_twofactor_auth`'s `enabled` field controls system-wide 2FA for
  password-based logins; this provider's own API-key authentication is
  unaffected by it (confirmed live), but changing it deliberately can lock
  out other users. Change it deliberately, same as any other singleton
  above.

## Development

```sh
make build      # go build
make test       # unit tests (no TrueNAS needed)
make testacc    # acceptance tests — needs a live box, see below
make fmt        # gofmt
make install    # build + install to ~/.terraform.d/plugins
```

### Layout

```
main.go                 provider entry point
internal/client/        WebSocket client: dial, auth (API key / password), job polling, TLS
internal/provider/      provider schema, Configure(), resource registration
internal/resources/<r>/ one package per resource:
                        model.go, schema.go, resource.go, datasource.go + tests
internal/acctest/       shared acceptance-test helpers
cmd/debug_api/          CLI for exploring the live API:
                        go run ./cmd/debug_api/ <section>|methods <prefix>|namespaces
examples/               HCL example per resource
```

### Testing

Full documentation of the test suite — tiers, environment variables, safety
rules, per-package coverage, and how to add tests — lives in
[TESTING.md](TESTING.md).

Unit tests run against payload builders, schema shapes, and response mappers —
no TrueNAS required:

```sh
go test ./...
```

Acceptance tests exercise a **live TrueNAS box** in three tiers:

```sh
export TRUENAS_ENDPOINT="wss://truenas.example.com/api/current"  # required
export TRUENAS_API_KEY="..."
export TRUENAS_TEST_POOL="tank"                     # pool for test fixtures

make testacc-safe        # Tier 1: TF_ACC=1 — full CRUD lifecycles on own
                         # tf-acc-* objects; never touches existing config
make testacc-disruptive  # Tier 2: adds TRUENAS_DISRUPTIVE=1 — singleton
                         # configs mutated with set-and-restore (original
                         # values re-applied and API-restored on failure)
TRUENAS_APPS=1 ...       # opt-in: app tests (pull container images)
```

Never-run tier: tests that could cut management access or migrate system
state (network gateways/hostname, management UI, SSH config, system dataset)
stay unconditionally skipped, each with in-file instructions for manual runs
against a disposable box. Set-and-restore tests also self-skip when the
target singleton is unconfigured (nothing to restore to). Run packages
sequentially — TrueNAS rate-limits authentication, and the provider retries
with backoff but parallel suites will still trip it.

### Adding a resource

1. Discover the API shape against a live box:
   `go run ./cmd/debug_api/ methods <namespace>.create` (and `.update`, `.delete`
   — check the `"job"` flag to pick `Call` vs `CallJob`).
2. Copy the closest existing package: plain int64 CRUD → `iscsi_extent`;
   singleton → `mail`; write-only secrets → `iscsi_auth`; JSON-document field →
   `vm_device`; association → `iscsi_targetextent`.
3. Follow the house conventions: read-back after create via `get_instance`;
   `IsNotFound` → remove from state; Optional+Computed fields guarded in
   payloads and given `UseStateForUnknown`; API `null` maps to Terraform null,
   not zero values; every data source gets a schema↔struct match test.
4. Register in `internal/provider/provider.go`, add an example under
   `examples/resources/`, and run `make test`.

# Terraform Provider for TrueNAS SCALE

A Terraform provider that manages [TrueNAS SCALE](https://www.truenas.com/truenas-scale/)
through its WebSocket API. Built on the
[Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework);
no REST calls, no external client dependency — a self-contained JSON-RPC 2.0
client lives inside the provider.

## What it manages

49 resources, each with a matching data source:

| Area | Resources |
|------|-----------|
| **Storage** | `truenas_pool`, `truenas_dataset`, `truenas_zvol`, `truenas_snapshot`, `truenas_periodic_snapshot_task` |
| **File shares** | `truenas_nfs_share`, `truenas_smb_share` |
| **iSCSI** | `truenas_iscsi_target`, `truenas_iscsi_extent`, `truenas_iscsi_initiator`, `truenas_iscsi_portal`, `truenas_iscsi_targetextent`, `truenas_iscsi_auth`, `truenas_iscsi_global` |
| **NVMe-oF** | `truenas_nvmet_subsys`, `truenas_nvmet_port`, `truenas_nvmet_namespace`, `truenas_nvmet_host`, `truenas_nvmet_host_subsys`, `truenas_nvmet_port_subsys`, `truenas_nvmet_global` |
| **Accounts** | `truenas_user`, `truenas_group` |
| **Apps & VMs** | `truenas_app`, `truenas_vm`, `truenas_vm_device` |
| **Replication & sync** | `truenas_replication_task`, `truenas_cloudsync_task`, `truenas_cloudsync_credentials` |
| **Network** | `truenas_network_interface`, `truenas_static_route`, `truenas_network_config` |
| **Services** | `truenas_service`, plus per-service configuration: `truenas_ssh_config`, `truenas_ftp_config`, `truenas_snmp_config`, `truenas_ups_config`, `truenas_smb_config`, `truenas_nfs_config` |
| **Alerts** | `truenas_alert_service`, `truenas_alert_policy` |
| **System** | `truenas_boot_environment`, `truenas_tunable`, `truenas_ntp_server`, `truenas_mail`, `truenas_system_general`, `truenas_system_advanced`, `truenas_system_dataset`, `truenas_replication_config` |

Both block-storage stacks are expressible end-to-end in HCL: iSCSI
(portal → target → extent → LUN association → CHAP auth) and NVMe-oF
(port → subsystem → namespace → host grant → port binding).

Working examples for every resource are under [`examples/resources/`](examples/resources/).

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.11 (write-only secret attributes)
- [Go](https://go.dev/doc/install) >= 1.25 (to build from source)
- TrueNAS SCALE 24.10+ (`truenas_app` and `truenas_nvmet_*` need 24.10/25.x APIs;
  the rest works on earlier SCALE releases)

### Tested TrueNAS versions

The full acceptance suite runs against live TrueNAS boxes on these releases:

| SCALE release | Status |
|---|---|
| 26.0 | Fully tested |
| 25.10 | Fully tested. `truenas_nvmet_host.description` is 26.0+ only; the provider rejects it with a clear error on older releases |

The provider detects the server release at runtime (`system.version_short`)
and gates version-specific fields, so a single configuration can target
either release as long as it avoids the newer fields.

## Initial setup

### 1. Build and install the provider

The provider is not published to the Terraform Registry; install it into your
local plugin directory:

```sh
git clone https://github.com/truenas/terraform-provider-truenas
cd terraform-provider-truenas
make install
```

This builds the binary and copies it to
`~/.terraform.d/plugins/registry.terraform.io/truenas/truenas/0.1.0/<os>_<arch>/`.

Alternatively, for a development workflow without reinstalling on every build,
use a [dev override](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides)
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

### 3. Configure the provider

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "0.1.0"
    }
  }
}

provider "truenas" {
  endpoint = "wss://truenas.example.com/api/current"
  api_key  = var.truenas_api_key

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

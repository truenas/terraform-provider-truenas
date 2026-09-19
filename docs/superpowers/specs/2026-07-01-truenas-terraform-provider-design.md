# TrueNAS Terraform Provider — Design Spec

**Date:** 2026-07-01  
**Status:** Approved

---

## Summary

Build a Terraform provider for TrueNAS SCALE that manages the full scope of resources exposed by the TrueNAS WebSocket API. Uses the Terraform Plugin Framework (not SDKv2). Targets `github.com/truenas/terraform-provider-truenas`.

---

## Architecture

Domain-package monorepo (Option B). Single Go module. No external TrueNAS client dependency — client lives inside the provider.

```
terraform-provider-truenas/
  main.go
  internal/
    client/
      client.go          # WebSocket dial, reconnect, request mux
      auth.go            # API key + SCRAM-SHA-256 auth
      jobs.go            # long-running job poller
      tls.go             # TLS config builder
    provider/
      provider.go        # schema, Configure(), resource/datasource registration
    resources/
      dataset/           # resource.go, datasource.go, schema.go, model.go, resource_test.go
      pool/
      snapshot/
      nfs/
      smb/
      user/
      group/
      app/
      replication/
      cloudsync/
      network/interface/
      network/route/
      service/
      periodic_snapshot/
      boot_environment/
      certificate/
      acme_dns/
      vm/
      iscsi/
      ...
  docs/                  # auto-generated via tfplugindocs
  examples/              # HCL usage examples per resource
  GNUmakefile
```

---

## Client Package

**`internal/client`** has no Terraform dependencies. It is a self-contained TrueNAS WebSocket client.

### Connection lifecycle
- Single persistent WebSocket connection per provider instance.
- Reconnect with exponential backoff on disconnect.
- Request multiplexing: each call gets a UUID; responses are routed by ID.

### Auth
- **API key:** send `auth.login_with_api_key` after connect.
- **SCRAM:** full SCRAM-SHA-256 exchange. Reuse proven logic from `~/api_client_golang`.

### Job tracking
- Many TrueNAS methods return a job ID, not a result.
- `jobs.go` subscribes to `core.subscribe "job"` events and fans out to per-job waiters.
- `client.CallJob(ctx, method, params...)` blocks until job SUCCESS or FAILURE.

### TLS
- `tls.go` builds `*tls.Config` from provider config:
  - `insecure = true` → `InsecureSkipVerify: true`
  - `ca_cert = "/path"` → load PEM, append to system cert pool

### Public API (only surface resources touch)
```go
func (c *Client) Call(ctx context.Context, method string, params ...any) (json.RawMessage, error)
func (c *Client) CallJob(ctx context.Context, method string, params ...any) (json.RawMessage, error)
```

---

## Provider Configuration

```hcl
provider "truenas" {
  endpoint = "wss://truenas.example.com/websocket"   # required; env: TRUENAS_ENDPOINT

  # Exactly one auth method required:
  api_key  = "..."    # env: TRUENAS_API_KEY

  username = "root"   # env: TRUENAS_USERNAME
  password = "..."    # env: TRUENAS_PASSWORD  (sensitive)

  # Exactly one TLS option (or neither for system-trusted certs):
  insecure = true
  ca_cert  = "/path/to/ca.pem"
}
```

### Validation in `Configure()`
- `endpoint` required.
- Exactly one of `api_key` OR (`username` + `password`) — error if both or neither.
- `insecure` and `ca_cert` are mutually exclusive.
- `password` and `api_key` are `Sensitive: true` in schema.
- All fields have `schema.EnvDefaultFunc` for corresponding env vars.

Configured `*client.Client` stored in provider data; passed to resources via `req.ProviderData`.

---

## Resource & Datasource Pattern

Each domain package (e.g. `resources/dataset/`) contains:

| File | Purpose |
|------|---------|
| `resource.go` | Implements `resource.Resource`: CRUD + ImportState |
| `datasource.go` | Implements `datasource.DataSource`: Read only |
| `schema.go` | Shared schema attrs reused by both |
| `model.go` | Go structs matching TrueNAS API JSON |
| `resource_test.go` | Acceptance tests |

### CRUD → API mapping (dataset example)
| Terraform | TrueNAS method |
|-----------|----------------|
| Create | `pool.dataset.create` |
| Read | `pool.dataset.get_instance` |
| Update | `pool.dataset.update` |
| Delete | `pool.dataset.delete` |

### Schema conventions
- TrueNAS-computed fields (mountpoint, id, guid, etc.) → `Computed: true`.
- Required user-supplied fields → `Required: true`.
- Optional with TrueNAS defaults → `Optional: true, Computed: true`.

### Import
All resources implement `ImportState` using the TrueNAS object identifier (name or numeric ID, per resource).

### Job-aware resources
Resources where the API returns a job ID (replication, cloud sync, pool wipe, etc.) call `client.CallJob()`. Job errors surface as Terraform diagnostics with the full job error message.

### Drift detection
`Read()` always fetches fresh state from API. If the object is gone (not-found error), `Read()` calls `resp.State.RemoveResource(ctx)` — Terraform will plan to recreate it.

---

## Error Handling

- TrueNAS WebSocket error format: `{"error": {"errname": "...", "reason": "..."}}`
- Client wraps these as `*APIError{Code, Message}`.
- Resources convert `*APIError` → `diag.Diagnostics` with the API reason as detail.
- Not-found errors in `Read()` → remove from state (not a diagnostic error).
- Context cancellation propagated through all `Call` / `CallJob` paths.

---

## Testing

### Unit tests (`make test`)
- Client package: mock WebSocket server.
- Verify SCRAM exchange, job polling, reconnect, TLS config building.

### Acceptance tests (`make testacc`)
- Require `TF_ACC=1` and live TrueNAS at `the target box`.
- Each resource: Create / Read / Update / Delete / Import test cases.
- No mocking of the TrueNAS API — real calls only.
- Shared helpers in `internal/acctest/`: test provider config, client setup.

---

## Build & Tooling

```makefile
generate   # go generate + tfplugindocs
fmt        # gofmt + goimports
lint       # golangci-lint
test       # unit tests
testacc    # acceptance tests (TF_ACC=1 required)
build      # go build ./...
install    # installs binary to ~/.terraform.d/plugins/registry.terraform.io/truenas/truenas/<ver>/<os_arch>/
```

Go version: 1.22+  
Key dependencies:
- `github.com/hashicorp/terraform-plugin-framework` (latest)
- `github.com/hashicorp/terraform-plugin-testing` (acceptance tests)
- `github.com/gorilla/websocket`
- `golang.org/x/crypto` (SCRAM-SHA-256)

---

## Resources in Scope

Everything the TrueNAS WebSocket API supports. Initial resource list (non-exhaustive — expand as API is surveyed):

- `truenas_dataset` / `truenas_pool`
- `truenas_nfs_share` / `truenas_smb_share`
- `truenas_user` / `truenas_group`
- `truenas_app`
- `truenas_vm` / `truenas_vm_device`
- `truenas_iscsi_target` / `truenas_iscsi_extent` / `truenas_iscsi_initiator`
- `truenas_replication_task` / `truenas_cloudsync_task`
- `truenas_periodic_snapshot_task` / `truenas_snapshot`
- `truenas_network_interface` / `truenas_static_route`
- `truenas_service` (NFS, SMB, SSH, iSCSI, etc.)
- `truenas_boot_environment`
- `truenas_certificate` / `truenas_certificate_authority` / `truenas_acme_dns_authenticator`
- `truenas_alert_service` / `truenas_alert_policy`
- `truenas_tunable` / `truenas_sysctl`
- `truenas_ntp_server` / `truenas_mail`
- `truenas_idmap` / `truenas_kerberos_realm` / `truenas_kerberos_keytab`
- `truenas_zvol`

Each resource also gets a corresponding `data "truenas_*" {}` datasource for read-only lookup.

---

## Out of Scope (v0.1)

- Terraform Cloud / remote backend specifics
- Publishing to Terraform Registry (can add GPG signing + goreleaser later)
- TrueNAS CORE (FreeBSD) — target is SCALE only

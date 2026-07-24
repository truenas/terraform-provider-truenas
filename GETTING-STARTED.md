# Getting Started with the TrueNAS Terraform Provider

This guide has two parts:

- **[A. Before the provider is in the Terraform Registry](#a-before-the-provider-is-in-the-terraform-registry)** —
  install from source into a local plugin directory.
- **[B. After the provider is published to the Terraform Registry](#b-after-the-provider-is-in-the-terraform-registry)** —
  the standard `terraform init` download.

Everything else — authentication, TLS, and your first configuration — is the
same either way and is covered in **[Common setup](#common-setup)** below.

## Requirements

- **Terraform** 1.11 or newer (the provider uses write-only attributes for
  secrets, introduced in 1.11).
- **TrueNAS** 25.04 or newer (the provider speaks JSON-RPC over the
  `/api/current` WebSocket endpoint). Verified against 25.10 and 26.0; TrueNAS
  26.0+ additionally supports SCRAM-SHA-512 API-key authentication.
- For building from source (Part A only): **Go 1.25** or newer.
- Network reachability from where Terraform runs to the TrueNAS box over
  the WebSocket port (443 by default, `wss://`).

---

## A. Before the provider is in the Terraform Registry

Until the provider is published, Terraform cannot download it. Build it from
source and place it in Terraform's local plugin directory; from then on,
`terraform init` finds it locally and normal version pinning works — no
`dev_overrides`, no special CLI config.

### A.1 Build and install

```sh
git clone https://github.com/truenas/terraform-provider-truenas.git
cd terraform-provider-truenas
make install
```

`make install` builds the `terraform-provider-truenas` binary and copies it to

```
~/.terraform.d/plugins/registry.terraform.io/truenas/truenas/<version>/<os>_<arch>/terraform-provider-truenas_v<version>
```

(the default version is `0.1.0`; override with `make install VERSION=x.y.z`).
That path is Terraform's implied local filesystem mirror for the
`truenas/truenas` provider address, so Terraform treats the local build as if
it came from the registry.

To build for a different machine than the one you build on, set `GOOS`/`GOARCH`:

```sh
make install GOOS=linux GOARCH=amd64
```

### A.2 Declare the provider

Use a normal `required_providers` block with a version constraint that matches
what you installed:

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "0.1.0"    # must match the installed version
    }
  }
}
```

Then run `terraform init` — it resolves `truenas/truenas` from the local
plugin directory. Continue at [Common setup](#common-setup).

> **Alternative for provider developers (not end users):** if you are
> iterating on the provider's own source, a `dev_overrides` CLI configuration
> points Terraform straight at a freshly built binary and skips
> `terraform init` entirely. See `examples/hello-lxc/run.sh` in this repo for
> a working example. Do not use `dev_overrides` for real infrastructure — it
> disables version pinning and dependency locking.

---

## B. After the provider is in the Terraform Registry

Once published, no build step is needed. Declare the provider and let
Terraform download it:

```hcl
terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 1.0"    # use the published version constraint you want
    }
  }
}
```

```sh
terraform init
```

`terraform init` downloads the provider from
[registry.terraform.io/truenas/truenas](https://registry.terraform.io/providers/truenas/truenas)
and records it in `.terraform.lock.hcl`. Continue at
[Common setup](#common-setup).

If you previously installed a local build (Part A), remove it from
`~/.terraform.d/plugins/registry.terraform.io/truenas/truenas/` first so
Terraform doesn't prefer the local copy over the registry release.

---

## Common setup

### 1. Create an API key on TrueNAS

In the TrueNAS UI: **Credentials → API Keys → Add**, name it, and copy the
generated key (shown once). The key has the form `<id>-<secret>`.

### 2. Configure the provider

The provider needs an `endpoint` and one authentication method. Supply them
either in the `provider` block or through environment variables (recommended
for secrets, so they stay out of your configuration files and state).

**Environment variables** (recommended):

```sh
export TRUENAS_ENDPOINT='wss://truenas.example.com/api/current'
export TRUENAS_API_KEY='1-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'
```

```hcl
provider "truenas" {
  # endpoint and api_key read from TRUENAS_ENDPOINT / TRUENAS_API_KEY
}
```

**Or inline** (use Terraform variables for the secret, never a literal):

```hcl
variable "truenas_api_key" {
  type      = string
  sensitive = true
}

provider "truenas" {
  endpoint = "wss://truenas.example.com/api/current"
  api_key  = var.truenas_api_key
}
```

### 3. Authentication options

| Method | Set | Notes |
|---|---|---|
| API key (plain) | `api_key` | Works on all supported releases. |
| API key with SCRAM | `api_key` + `username` (the key's owner) | On TrueNAS 26.0+, the raw key never crosses the wire — the client proves possession via SCRAM-SHA-512. On 25.10 this transparently falls back to plain API-key login. Recommended on 26.0+. |
| Username + password | `username` + `password` | Mutually exclusive with `api_key`. |

Provider arguments (all also available as `TRUENAS_*` env vars):

| Argument | Env | Purpose |
|---|---|---|
| `endpoint` | `TRUENAS_ENDPOINT` | `wss://<host>/api/current`. A legacy `/websocket` path is rewritten automatically. |
| `api_key` | `TRUENAS_API_KEY` | API key (`<id>-<secret>`). Sensitive. |
| `username` | `TRUENAS_USERNAME` | Key owner (enables SCRAM) or password-auth user. |
| `password` | `TRUENAS_PASSWORD` | Password (requires `username`). Sensitive. |
| `insecure` | — | `true` skips TLS verification. Mutually exclusive with `ca_cert`. |
| `ca_cert` | — | Path to a PEM CA certificate. Mutually exclusive with `insecure`. |

### 4. TLS

TrueNAS ships a self-signed certificate by default. For a quick start against
such a box:

```hcl
provider "truenas" {
  endpoint = "wss://truenas.example.com/api/current"
  api_key  = var.truenas_api_key
  insecure = true    # skip verification of the self-signed cert
}
```

For a box with a certificate signed by a private CA, point at the CA instead
of disabling verification:

```hcl
provider "truenas" {
  endpoint = "wss://truenas.example.com/api/current"
  api_key  = var.truenas_api_key
  ca_cert  = "/etc/ssl/certs/my-truenas-ca.pem"
}
```

> **`insecure = true` also weakens authentication.** Certificate verification
> is what proves you are talking to the real box. With it off, a machine that
> intercepts the connection can present any mechanism list it likes, which
> defeats SCRAM's on-wire key protection: the client will fall back to plain
> API-key login and send the reusable key (inside TLS, but to an unverified
> peer). Use `insecure = true` only against a box you trust on a trusted
> network — for anything else, use `ca_cert` (or a publicly-trusted
> certificate) so the peer is verified and SCRAM's protection holds.

### 5. First configuration

A small, safe first step — create a dataset under an existing pool, then read
it back. Replace `tank` with a pool that exists on your box.

```hcl
resource "truenas_dataset" "hello" {
  name = "tank/tf-hello"
}

output "dataset_mountpoint" {
  value = truenas_dataset.hello.mountpoint
}
```

```sh
terraform plan     # review what will be created
terraform apply    # create it
terraform show     # inspect state
```

Verify in the TrueNAS UI under **Datasets**, then clean up:

```sh
terraform destroy
```

### 6. Where to go next

- **Resource and data-source reference** — the generated docs under `docs/`
  (or the provider's Registry documentation once published).
- **`examples/`** — runnable configurations, including
  `examples/hello-lxc/` (a full LXC container on TrueNAS 26.0+).
- **Best-practices guide** — `docs/guides/best-practices.md`.

---

## Troubleshooting

- **`terraform init` can't find the provider (Part A):** the installed
  binary's version must match the `version` in `required_providers`, and its
  `<os>_<arch>` directory must match the machine running Terraform. Re-run
  `make install` (optionally with `GOOS`/`GOARCH`).
- **`Cannot connect to TrueNAS`:** check the `endpoint` scheme (`wss://`) and
  path (`/api/current`), and that the WebSocket port is reachable.
- **TLS/certificate errors:** the box likely has a self-signed cert — set
  `insecure = true` for a quick start, or `ca_cert` for a private CA.
- **`Rate Limit Exceeded`:** TrueNAS rate-limits authentication
  (~20 logins/minute). Each Terraform run opens a fresh connection; the
  provider retries with backoff, but very large parallel runs can still trip
  it. Prefer fewer, larger applies.
- **Auth rejected with `api_key` + `username` on 25.10:** SCRAM is 26.0+; the
  provider falls back to plain login automatically, so this should just work —
  if it doesn't, confirm the `username` is the key's actual owner.

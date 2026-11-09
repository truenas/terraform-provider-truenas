---
page_title: "S3 state backend"
subcategory: "Guides"
description: |-
  Using the TrueNAS S3 service (TrueNAS 27.0 and later) as the S3 state
  backend for Terraform or OpenTofu, and managing the service and its access
  keys with this provider.
---

# S3 state backend

TrueNAS 27.0 added a native S3 service. It works as the `backend "s3"` of
Terraform and OpenTofu: a TrueNAS you run can hold your state, and the
configuration whose state it holds does not have to manage TrueNAS at all.

This guide covers the backend settings that work against TrueNAS, and how to
set the service and its access keys up with `truenas_s3_config`,
`truenas_s3_access_key` and `truenas_service`.

## Backend settings

The S3 backend assumes AWS unless told otherwise. These settings point it at
TrueNAS and switch off the AWS-only checks:

```hcl
terraform {
  backend "s3" {
    bucket = "tofu-state"
    key    = "infra/terraform.tfstate"
    region = "us-east-1" # required by the backend, ignored by TrueNAS

    endpoints = {
      s3 = "http://truenas.example.com:9000"
    }
    use_path_style              = true
    skip_credentials_validation = true
    skip_region_validation      = true
    skip_requesting_account_id  = true
    skip_metadata_api_check     = true
    use_lockfile                = true # S3-native locking, no DynamoDB
  }
}
```

Credentials come from the usual S3 backend sources: `access_key` /
`secret_key` in the block, `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY`, or
a shared credentials file. Use a TrueNAS S3 access key (below), not a TrueNAS
API key.

With a TLS listener, use `https://` and, for a certificate the client does
not trust, the backend's `insecure = true`.

## Setting the service up with this provider

The configuration that manages the TrueNAS (not the one storing its state
there) turns the service on, points bucket creation at a dataset, and creates
the access key:

```hcl
resource "truenas_dataset" "s3" {
  name = "tank/s3"
}

resource "truenas_s3_config" "s3" {
  listeners            = [{ address = "0.0.0.0", port = 9000 }]
  managed_root_dataset = truenas_dataset.s3.name
}

resource "truenas_service" "s3" {
  service = "s3"
  enable  = true
  running = true
}

# The key's owner must be a local user whose group holds the SHARING_S3_WRITE
# role for manage_buckets to be allowed.
resource "truenas_group" "s3admins" {
  name = "s3admins"
}

resource "truenas_privilege" "s3admins" {
  name         = "s3admins"
  local_groups = [truenas_group.s3admins.gid]
  roles        = ["SHARING_S3_WRITE"]
  web_shell    = false
}

resource "truenas_user" "svc_state" {
  username     = "svc_state"
  full_name    = "Terraform state"
  group_create = true
  groups       = [truenas_group.s3admins.id]
  password_disabled = true
  shell        = "/usr/sbin/nologin"
  home         = "/var/empty"
  depends_on   = [truenas_privilege.s3admins]
}

resource "truenas_s3_access_key" "state" {
  name           = "terraform-state"
  username       = truenas_user.svc_state.username
  manage_buckets = true
}

output "state_access_key" {
  value = truenas_s3_access_key.state.access_key
}

output "state_secret" {
  value     = truenas_s3_access_key.state.secret
  sensitive = true
}
```

`secret` is stored once, from the create response; TrueNAS can return it on
later reads but the provider never does. To keep it out of state entirely,
supply your own with the write-only `secret_wo` (plus `secret_wo_version`).
`rotate_version` rotates a generated secret in place.

## Buckets

TrueNAS has no API for buckets. They are created through the S3 protocol
(`CreateBucket`) by an access key with `manage_buckets = true`, and each gets
a dataset under `managed_root_dataset`. Create the state bucket with any S3
client:

```bash
AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=... \
  aws --endpoint-url http://truenas.example.com:9000 s3 mb s3://tofu-state
```

or, in Terraform, with the `aws` provider pointed at the TrueNAS endpoint
(`endpoints { s3 = ... }`, `s3_use_path_style = true`, the same `skip_*`
settings as the backend) and an `aws_s3_bucket`, or with the `minio` provider.

## Bootstrapping

The configuration that manages the TrueNAS cannot store its own state on
that TrueNAS before the service exists. Either keep that configuration's
state elsewhere, or bring the S3 service up by hand (or with a local state),
then import `truenas_s3_config` (any id, it is a singleton) and the key, and
migrate the state with `terraform init -migrate-state`.

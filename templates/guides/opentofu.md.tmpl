---
page_title: "OpenTofu"
subcategory: "Guides"
description: |-
  What works in OpenTofu, what is Terraform-only (actions), and the resource or
  attribute that covers each Terraform-only operation.
---

# OpenTofu

Terraform and OpenTofu share the core language this provider is built on:
resources, data sources, list resources, resource identity, ephemeral resources
and write-only attributes all work in both, and the provider is tested with
Terraform. One feature does not carry over: **actions** (`action` blocks run by
a `lifecycle { action_trigger {} }`), added in Terraform 1.14. OpenTofu has no
actions as of 1.13 ([opentofu/opentofu#3309](https://github.com/opentofu/opentofu/issues/3309)
tracks it) and rejects the blocks with `Blocks of type "action" are not
expected here`.

The provider's rule, so that OpenTofu users are never second-class:

- A capability is built on what both tools support. A new language feature is
  adopted once both have it (write-only attributes and ephemeral resources are
  examples); until then it is only ever an *extra*.
- Anything a configuration needs **as part of a plan** is a resource or an
  attribute. Actions are offered for one-off operations (run this cron job now,
  roll this dataset back) that do not belong in a plan.

## Terraform-only operations and their OpenTofu form

| Operation | Action (Terraform 1.14+) | Resource or attribute (both tools) |
|---|---|---|
| Start a VM after its devices exist | `truenas_vm_start` triggered after each `truenas_vm_device` | `truenas_vm_power` with `running = true` and `depends_on` the devices; leave `running` unset on the `truenas_vm` |
| Stop a VM | `truenas_vm_stop` | `truenas_vm_power` with `running = false`, or `running = false` on the `truenas_vm` |
| Restart a VM so a settings change takes effect | `truenas_vm_restart` (unconditional) | `restart_on_update = true` on `truenas_vm` (restarts only when a setting changed), or `restart_triggers` on `truenas_vm_power` (restarts when a referenced value changes, for example a device's `attributes`) |
| Clone a zvol or dataset from a golden snapshot | `truenas_snapshot_clone` (the clone is not managed) | `origin = "pool/golden@snap"` on `truenas_zvol` / `truenas_dataset`: the clone is a managed resource, destroyed with its VM, and the snapshot stays until its clones are gone |
| Rename a dataset or zvol | `truenas_dataset_rename` | change `name`: renamed in place (see the Data Safety guide for the in-use check) |
| Start, stop or redeploy an app | `truenas_app_start` / `truenas_app_stop` / `truenas_app_redeploy` | `running` on `truenas_app` (note that a new app always runs once at create: TrueNAS has no create-stopped option) |
| Control a service | `truenas_service_control` | `truenas_service` (`enable`, `running`) |

Everything else offered as an action (scrub, cron job, replication, cloud
sync and snapshot-task runs; snapshot hold, release, rollback and rename;
dataset lock, unlock, promote and set-quota; VM clone and reset; UI restart)
is a one-off operation. In OpenTofu run it through the TrueNAS UI or API
(`midclt call`), outside the plan.

## Example: a VM that boots with its disk attached

A `truenas_vm` with `running = true` starts as soon as it is created, before
its disks and NICs (separate `truenas_vm_device` resources that need the VM's
id) exist, and the VM cannot depend on its own devices. `truenas_vm_power`
puts the start after them:

```hcl
resource "truenas_vm" "web" {
  name   = "web"
  memory = 2048 # MiB
  vcpus  = 2
  # running is left unset: truenas_vm_power owns the power state
}

resource "truenas_vm_device" "web_disk" {
  vm = truenas_vm.web.id
  attributes = jsonencode({
    dtype = "DISK"
    path  = "/dev/zvol/tank/vms/web"
    type  = "VIRTIO"
  })
}

resource "truenas_vm_power" "web" {
  vm_id      = truenas_vm.web.id
  running    = true
  depends_on = [truenas_vm_device.web_disk]
  restart_triggers = {
    disk = truenas_vm_device.web_disk.attributes # a disk change restarts the guest
  }
}
```

## Example: a VM disk cloned from a golden image

```hcl
resource "truenas_snapshot" "golden" {
  dataset = "tank/golden/debian13"
  name    = "v1"
}

resource "truenas_zvol" "web_disk" {
  name    = "tank/vms/web"
  volsize = 64 * 1024 * 1024 * 1024
  origin  = truenas_snapshot.golden.id # "tank/golden/debian13@v1"
}
```

`origin` is create-only: changing it replaces the zvol, which is a new disk
(the destroy half is guarded by `force_destroy`, see the Data Safety guide).
ZFS will not destroy a snapshot that still has clones, and the reference above
makes the clones depend on the snapshot, so `terraform destroy` removes them
first.

## State backend

TrueNAS's own S3 service can hold the Terraform or OpenTofu state, for this or
any other configuration. See the S3 state backend guide.

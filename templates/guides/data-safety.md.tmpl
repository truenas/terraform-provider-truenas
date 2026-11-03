---
page_title: "Data Safety"
subcategory: "Guides"
description: |-
  Every way this provider can destroy data on a TrueNAS system, the guard that
  stands in front of each one, and the explicit opt-in that removes the guard.
---

# Data Safety

Terraform's model is "make the system match the configuration", and for a
storage system that can mean destroying data: a dataset that disappears from
the configuration is destroyed, a change to an attribute that cannot be
updated in place replaces the resource (destroy, then create), and a pool
destroy erases everything on it. This provider treats every such path as
something the operator must ask for explicitly. The table below is the
complete list; a unit test (`TestDestructiveCalls_Guarded`) fails the build if
a new destructive API call appears anywhere else.

## Guards and opt-ins

| What would destroy data | Guard (default behaviour) | Opt-in |
|---|---|---|
| `terraform destroy` of a `truenas_pool`, or removing it from the configuration | Refused while the pool holds any dataset, zvol, snapshot or file; the error lists them | `force_destroy = true` on the pool |
| `truenas_pool` `name` or `topology` change | Refused at plan time; the pool is never replaced | none: rename or reshape the pool on TrueNAS and `terraform apply -refresh-only` |
| `terraform destroy` of a `truenas_dataset` or `truenas_zvol`, or removing it from the configuration | Refused while it has child datasets, snapshots, or files (dataset) / written blocks (zvol) | `force_destroy = true` |
| Replacement of a `truenas_dataset` / `truenas_zvol` (a change to `type`, `acltype`, `share_type`, encryption settings, `volblocksize`) | Plans a replace; the destroy half hits the same guard | `force_destroy = true` |
| Renaming a `truenas_dataset` / `truenas_zvol` | In place (`pool.dataset.rename`): data, children and snapshots move. Refused while anything on TrueNAS references the dataset or a descendant: SMB and NFS shares, iSCSI extents, NVMe-oF namespaces, periodic snapshot, replication, rsync, cloud sync and cloud backup tasks, VM disk devices. A rename into another pool, or into itself, is refused at plan time | `allow_rename_in_use = true` (skips the whole check) |
| Replacement of a `truenas_snapshot` (a change to `dataset`, `name` or `recursive`) | Refused at plan time: the point-in-time copy would be lost | `allow_recreate = true` |
| `terraform destroy` of a `truenas_snapshot` | Destroys that snapshot only (never recursive unless it was taken recursive) | none |
| `terraform destroy` of a `truenas_vm` | Keeps the VM's zvols (`zvols = false`) | none: destroy the zvols as their own resources |
| `terraform destroy` of a `truenas_vm_device` | Keeps backing zvols and raw files | none |
| `terraform destroy` of a `truenas_iscsi_extent` / `truenas_nvmet_namespace` | Keeps the backing file or zvol | none |
| `terraform destroy` of a `truenas_app` | Keeps the app's ix-volumes (`remove_ix_volumes = false`); removes images | none |
| `terraform destroy` of a `truenas_container` | The container's own root filesystem goes with it, like a VM's definition; disks attached from datasets are not touched | n/a |
| `terraform destroy` of a `truenas_boot_environment` | Refused for the active or activated environment | none |
| Actions `truenas_dataset_rename`, `truenas_snapshot_rename`, `truenas_snapshot_rollback` | Run only when invoked; `snapshot_rollback` discards everything written after the snapshot and says so | the invocation itself; `force` is passed through from the action's configuration |

## What the guards cannot do

- **TrueNAS performs no safety checks on rename.** `pool.dataset.rename`
  refuses every call without `force`, and with it renames regardless of use.
  Shares, iSCSI extents, replication tasks and snapshot tasks that point at the
  old path are not updated by TrueNAS. The provider refuses the rename while
  other TrueNAS objects reference the dataset or a descendant (and also when
  it cannot check, for example because a query failed); with
  `allow_rename_in_use = true` it renames and those objects break until
  re-pointed. Other Terraform resources that reference the old path plan their
  own update on the *next* apply, so there is a window with a broken path.
  Renaming a zvol under a live iSCSI extent takes the LUN offline. A
  `truenas_snapshot` of the renamed dataset plans a replacement: the old
  point-in-time copy stays on the renamed dataset, unmanaged, and a new
  snapshot is taken (its `allow_recreate` guard applies).
- **`force_destroy` is sticky.** Once applied, a later replacement destroys
  without further warning. Set it immediately before the destroy that needs it
  and remove it afterwards.
- **A snapshot destroy is a destroy.** `allow_recreate` only governs
  replacement; removing a `truenas_snapshot` from the configuration destroys
  it.
- **Replication and cloud sync can discard data by design.**
  `truenas_replication_task` with `allow_from_scratch = true` destroys the
  target's snapshots when the incremental base is missing; `retention_policy`
  prunes target snapshots; `truenas_cloudsync_task` with `transfer_mode =
  "SYNC"` deletes files at the destination that are absent at the source. These
  are TrueNAS behaviours the configuration asks for; the provider passes them
  through unchanged and defaults none of them on.
- **`lifecycle { prevent_destroy = true }`** is Terraform's own guard and works
  independently of the provider's. Use it on pools and on datasets that hold
  data you cannot recreate.

## Recovering from a refused destroy

A refusal is an error at apply time and changes nothing on the system. Read
what the error lists, then either move or delete that data deliberately, or
add the opt-in attribute, apply once so it is recorded in state, and run the
destroy again.

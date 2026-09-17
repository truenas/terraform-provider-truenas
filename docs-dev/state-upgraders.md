# State Schema Versioning and State Upgraders

A developer guide to how this provider keeps existing Terraform state working
across provider upgrades when a resource's schema changes shape. This is the
mechanism behind the state-migration commitment in `VERSIONING.md`.

## The problem it solves

Terraform stores each resource's state as a JSON object whose structure is
dictated by that resource's **schema** at the time it was written. State also
records a **`SchemaVersion`** integer per resource (default `0`).

When a user upgrades the provider and the new version's schema for a resource
no longer matches the shape of their stored state, Terraform cannot simply
read it — the old JSON does not fit the new schema. Depending on the mismatch
the user gets a hard error (a prior-state/decoding mismatch), a spurious forced
replacement, or — worst — silently wrong values. The user did nothing wrong;
their state was written by an older version of the provider.

A **StateUpgrader** is the migration function that takes state written at
schema version *N* and rewrites it into version *N+1*'s shape, so Terraform can
read it. Terraform runs it **automatically** on the first plan/refresh after
the upgrade — the user never runs manual `terraform state` surgery.

## When you need one vs. when you don't

No upgrader is needed for backward-compatible changes:

- **Adding** a new Optional attribute — old state has no value for it; it reads
  as null.
- **Removing** an attribute — the framework drops the extra key.

An upgrader (and a `SchemaVersion` bump) **is** needed when the shape or
meaning changes:

- **Rename** an attribute (`old_name` -> `new_name`): old state has `old_name`,
  the new schema wants `new_name`; without migration the value is lost and
  `new_name` plans as a change.
- **Type change**: e.g. a string that becomes a list, or an int that becomes a
  nested object.
- **Restructure**: flatten a nested block into top-level attributes or the
  reverse; split one attribute into two; merge two into one.
- **Change an ID format** or a computed value's structure.

## How it works in this provider (plugin-framework)

This provider is built on terraform-plugin-framework (not SDKv2). A resource
that needs migration implements `ResourceWithUpgradeState`: bump its
`SchemaVersion` and provide an `UpgradeState` method returning a map keyed by
the **prior** version number.

```go
func (r *ThingResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
    resp.Schema = schema.Schema{
        Version: 1, // was 0; bumped because the shape changed
        Attributes: map[string]schema.Attribute{ /* new shape */ },
    }
}

func (r *ThingResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
    return map[int64]resource.StateUpgrader{
        // migrate state written at version 0 -> the current version (1)
        0: {
            PriorSchema: &schema.Schema{ /* the OLD v0 shape */ },
            StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
                var old thingModelV0
                resp.Diagnostics.Append(req.State.Get(ctx, &old)...)
                // transform old fields into the new model...
                updated := thingModelV1{NewName: old.OldName /* etc */}
                resp.Diagnostics.Append(resp.State.Set(ctx, updated)...)
            },
        },
    }
}
```

Terraform sees the state is at version 0 and the provider is now at version 1,
finds the `0:` entry, decodes the old state with `PriorSchema`, runs the
transform, and stores the result at version 1. It is **per-resource** — only
resources whose shape changed need this, each on its own version counter.

## A concrete example

`truenas_vm_device`'s `attributes` is an opaque JSON string that can contain a
DISPLAY device's `password`. Suppose a future MINOR release splits that
password into a dedicated write-only attribute (`display_password`) for a
cleaner diff. Existing users' state has the password *inside* the `attributes`
blob; the new schema expects it in `display_password`. That is a shape change:
bump `SchemaVersion` to 1 and write an upgrader that parses the old
`attributes` JSON, lifts the password out into `display_password`, and stores
the rest. Without it, every `vm_device` in existing state breaks on upgrade.

During pre-1.0 development this class of change happened a few times (for
example, the `directoryservices` idmap block was restructured), but pre-1.0
makes no compatibility promise, so state churn was acceptable. After `v1.0.0`
it is not.

## Why there is nothing to write yet

Every resource currently ships at `SchemaVersion` 0 (the framework default),
and no breaking schema change has been made against a released version — so
there is nothing to migrate *from*. Writing an upgrader now would have no prior
version to upgrade.

The commitment (see `VERSIONING.md`) is: the first change that alters a
released resource's shape bumps that resource's `SchemaVersion` and ships the
matching `StateUpgrader` in the same release. Each upgrader is tested — the
framework provides state-upgrade test helpers so a test can assert that a given
old-state input produces the expected new-state output.

## Testing an upgrader

When one is added, cover it with a test that feeds representative version-*N*
state through the upgrader and asserts the version-*N+1* result — including
edge cases (a null/absent old field, an empty nested block). The acceptance
suite can additionally run a real `terraform apply` with an older state fixture
to confirm the migration end-to-end against a live box, consistent with this
project's no-mocks policy.

# Versioning, Compatibility, and Deprecation Policy

This provider follows [Semantic Versioning](https://semver.org/) (`MAJOR.MINOR.PATCH`).
This document states what each part of the version means, which TrueNAS
releases a provider version supports, how state survives upgrades, and how
attributes and resources are deprecated and removed.

## What the version numbers mean

- **PATCH** (`x.y.Z`) — bug fixes and internal changes with no
  configuration-visible effect. Always safe to upgrade.
- **MINOR** (`x.Y.z`) — new resources, data sources, or attributes, and other
  backward-compatible additions. Existing configurations continue to plan and
  apply unchanged. May introduce deprecations (see below), but does not remove
  or change the meaning of anything.
- **MAJOR** (`X.y.z`) — changes that can break existing configurations or
  state: removing a resource/attribute, renaming, changing an attribute's type
  or default, or changing behavior in a way that alters a plan. Major releases
  document required migration steps.

Pre-1.0 (`0.y.z`) releases make no compatibility promise; treat every
`0.y` bump as potentially breaking.

## TrueNAS release compatibility

The provider speaks JSON-RPC 2.0 over the `/api/current` WebSocket endpoint.

- **Minimum:** TrueNAS 25.04 (the first release serving `/api/current`).
- **Verified:** each release is tested against the TrueNAS versions listed in
  its release notes / `README.md`. Newer-release-only fields are version-gated
  in the provider: on an older release they are omitted from API payloads and
  the resource surfaces a clear diagnostic if they are set explicitly.
- The provider identifies itself to the middleware with a `User-Agent` of
  `terraform-provider-truenas/<version>`.

When a TrueNAS release changes a wire schema, the provider adapts (version
gate or dual-shape decode) rather than breaking; such adaptations ship in a
MINOR or PATCH release.

## State compatibility across upgrades

Terraform state written by one provider version continues to be readable by
later versions within the same MAJOR line. When a resource's schema shape
changes in a way that would otherwise invalidate existing state, the resource's
`SchemaVersion` is incremented and a `StateUpgrader` is added so existing state
is migrated automatically on the next plan. Users do not need to run manual
state surgery for supported upgrades.

See `docs-dev/state-upgraders.md` for the developer guide to writing these.

All resources currently ship at `SchemaVersion` 0; the first change that
requires it will introduce the corresponding upgrader.

## Deprecation process

Before anything that a configuration can reference is removed, it is
deprecated first:

1. In a MINOR release, the attribute or resource is marked deprecated (a
   `DeprecationMessage` / deprecated attribute description). Terraform emits a
   warning on plan/apply, pointing to the replacement. The item keeps working.
2. The deprecation is noted in `CHANGELOG.md` with the replacement and the
   earliest version in which removal may occur.
3. Removal happens only in a subsequent MAJOR release, never sooner.

New replacements are added alongside the deprecated item so users can migrate
before the removal.

## Releasing

Releases are cut by tagging a semver version (`vX.Y.Z`); the release workflow
builds, signs, and publishes the artifacts the Terraform Registry ingests.
`CHANGELOG.md` is updated in the same change that cuts the release.

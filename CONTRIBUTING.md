# Contributing

Thanks for your interest in the TrueNAS Terraform provider.

## Development requirements

- Go (see `go.mod` for the required version)
- Terraform 1.11+
- For acceptance tests: access to a real TrueNAS box (this project
  does not mock the server — see below)

## Building

```sh
make build       # build the provider binary
make install     # build and install into the local Terraform plugin dir
make generate    # regenerate docs from schema (run after schema changes)
make fmt         # gofmt
make lint        # golangci-lint
```

To browse the generated docs locally — grouped by subcategory and rendered
as the Terraform Registry would show them — run
`scripts/serve-docs-preview.sh` and open the printed URL. It builds a
self-contained page into `docs-preview/` (gitignored) from the current
`docs/`. Requires the `markdown` Python package.

## Testing

The testing model is documented in full in **TEST-PLAN.md**; **TESTING.md**
is the operator quick reference.

- **Unit tests** need no TrueNAS box: `make test`. Unit tests are
  pure-function only (payload builders, response mappers, schema shape).
- **Acceptance tests** run against a **real** TrueNAS box — there are
  no mock servers anywhere in this project, by policy. They are gated on
  `TF_ACC=1` plus connection env vars and skip cleanly when unset:

  ```sh
  export TRUENAS_ENDPOINT='wss://<box>/api/current'
  export TRUENAS_API_KEY='<id>-<secret>'
  export TRUENAS_TEST_POOL='tank'
  make testacc-safe          # Tier 1: creates/destroys only its own objects
  make testacc-disruptive    # Tier 2: singleton set-and-restore
  ```

Some suites need additional infrastructure (directory servers, an HA pair)
and additional gates (`TRUENAS_DS`, `TRUENAS_HA`); see TEST-PLAN.md.

## Conventions

- **Probe before coding.** Verify every wire assumption (field names, job
  flags, response shapes, secret read-back) against the live API before
  writing the code that depends on it. `go run ./cmd/debug_api/ methods
  <method>` dumps a method's schema. Record the evidence in code comments.
- **Never commit secrets.** No real API keys, passwords, keytabs, or private
  keys in code, tests, examples, or docs — use placeholders and
  environment variables. Test fixtures that need a secret generate a
  synthetic one.
- **Follow the sibling pattern.** New resources mirror the structure of the
  closest existing package (`resource.go`, `datasource.go`, `schema.go`,
  `model.go`, unit tests, `acceptance_test.go`). Register in
  `internal/provider/provider.go`.
- **Docs are generated.** Edit schema `Description` strings and
  `examples/`, then run `make generate`; do not hand-edit `docs/`.
- Run `make fmt lint` and `go build ./... && go vet ./... && go test ./...`
  before opening a pull request.

## Release builds

Releases are produced by [GoReleaser](https://goreleaser.com/) and cut by
pushing a semver tag (`vX.Y.Z`); see `.github/workflows/release.yml`. To
exercise the release build locally (GoReleaser must be installed):

```sh
make release-check     # validate .goreleaser.yml
make build-snapshot    # build for the current platform only
make release-snapshot  # full dry run (all platforms, no sign/publish) -> dist/
```

## Pull requests

Keep changes focused. Describe what was probed and how the change was
verified against a live box (which release(s)). CI runs build, vet, gofmt, lint, unit tests, a docs-up-to-date check, a
license-header check, and a GoReleaser config check; acceptance tests run
separately on lab hardware.

## License

See `VERSIONING.md` for the versioning, compatibility, and deprecation
policy.

By contributing, you agree that your contributions are licensed under the
Mozilla Public License 2.0 (see `LICENSE`).

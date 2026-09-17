<!--
Thanks for contributing. Please describe the change and how it was verified.
See CONTRIBUTING.md for the project's conventions (probe-before-code,
no-mocks testing, generated docs).
-->

## What this changes

<!-- A short description of the change and why. Link any related issue. -->

## How it was verified

- [ ] `go build ./... && go vet ./...` clean
- [ ] `make fmt` and `make lint` clean
- [ ] `make test` (unit) passes
- [ ] Acceptance tested against a live TrueNAS box (state which release(s), or
      explain why not applicable):
- [ ] `make generate` run and generated docs committed (if schema changed)

## Notes for reviewers

<!-- Wire assumptions probed, edge cases, anything that needs a careful look.
     For a new resource, note the release(s) it was verified against and any
     version-gated fields. -->

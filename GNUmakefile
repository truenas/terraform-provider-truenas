BINARY_NAME   = terraform-provider-truenas
VERSION      ?= 0.1.0
OS_ARCH       = $(shell go env GOOS)_$(shell go env GOARCH)
INSTALL_DIR   = ~/.terraform.d/plugins/registry.terraform.io/truenas/truenas/$(VERSION)/$(OS_ARCH)

.PHONY: default build install test testacc testacc-safe testacc-disruptive generate fmt lint \
        test-inventory api-calls go-deps sbom \
        release-check build-snapshot release-snapshot \
        loadtest loadtest-client loadtest-tf loadtest-sweep

default: build

build:
	go build -o $(BINARY_NAME) .

# --- Release (GoReleaser) ---------------------------------------------------
# Local release tooling. GoReleaser must be installed
# (https://goreleaser.com/install/); CI installs it via the release workflow.
# Real releases are cut by pushing a semver tag (see .github/workflows/release.yml).

# Validate .goreleaser.yml without building.
release-check:
	goreleaser check

# Build the provider for the current platform only (fast sanity build).
build-snapshot:
	goreleaser build --snapshot --clean --single-target

# Full local dry run: build every OS/arch, archives, checksums, and manifest —
# skipping signing and publishing. Inspect the results under dist/.
release-snapshot:
	goreleaser release --snapshot --clean --skip=sign,publish

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)_v$(VERSION)

test:
	go test ./... -v -count=1

testacc:
	TF_ACC=1 go test ./... -v -count=1 -timeout 120m

testacc-safe:
	TF_ACC=1 go test ./internal/resources/... -v -count=1 -timeout 60m

testacc-disruptive:
	TF_ACC=1 TRUENAS_DISRUPTIVE=1 go test ./internal/resources/... -v -count=1 -timeout 90m

generate:
	go generate ./...
	go tool tfplugindocs generate --provider-name truenas
	python3 scripts/set-subcategories.py

fmt:
	gofmt -s -w .

lint:
	golangci-lint run ./...

# Regenerate TEST-INVENTORY.md from the *_test.go sources.
test-inventory:
	python3 scripts/build-test-inventory.py

# Regenerate API-CALLS.md — inventory of every TrueNAS JSON-RPC method called.
# Diff it against a new TrueNAS release to catch API changes affecting drivers.
api-calls:
	python3 scripts/list-api-calls.py

# Regenerate DEPENDENCIES.md — inventory of every external Go module, marking
# which ship in the provider binary (Runtime) vs tooling-only.
go-deps:
	python3 scripts/list-go-deps.py

# Generate SBOMs (SPDX + CycloneDX) from the compiled binary into sbom/.
# Requires syft. Release artifacts get equivalent SBOMs via GoReleaser.
sbom:
	bash scripts/gen-sbom.sh

# Load testing (needs a DISPOSABLE box: TRUENAS_LOAD=1 +
# TRUENAS_LOAD_ALLOWED_ENDPOINT == TRUENAS_ENDPOINT). Reports land in results/.
loadtest: loadtest-client loadtest-tf

loadtest-client:
	TF_ACC=1 go test ./internal/client/ -run TestLoad_ -v -count=1 -timeout 30m

loadtest-tf:
	TF_ACC=1 go test ./test/load/ -run TestLoad_ -v -count=1 -timeout 60m

# Delete stranded tf-load- datasets after a crashed/killed load run.
loadtest-sweep:
	TF_ACC=1 go test ./internal/client/ -run '^TestLoadSweep$$' -v -count=1 -timeout 10m

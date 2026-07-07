BINARY_NAME   = terraform-provider-truenas
VERSION      ?= 0.1.0
OS_ARCH       = $(shell go env GOOS)_$(shell go env GOARCH)
INSTALL_DIR   = ~/.terraform.d/plugins/registry.terraform.io/truenas/truenas/$(VERSION)/$(OS_ARCH)

.PHONY: default build install test testacc testacc-safe testacc-disruptive generate fmt lint

default: build

build:
	go build -o $(BINARY_NAME) .

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

fmt:
	gofmt -s -w .

lint:
	golangci-lint run ./...

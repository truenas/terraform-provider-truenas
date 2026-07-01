BINARY_NAME   = terraform-provider-truenas
VERSION      ?= 0.1.0
OS_ARCH       = $(shell go env GOOS)_$(shell go env GOARCH)
INSTALL_DIR   = ~/.terraform.d/plugins/registry.terraform.io/truenas/truenas/$(VERSION)/$(OS_ARCH)

.PHONY: default build install test testacc generate fmt lint

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

generate:
	go generate ./...
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate

fmt:
	gofmt -s -w .

lint:
	golangci-lint run ./...

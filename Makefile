.PHONY: build test install

build:
	go build -o terraform-provider-mistedo .

test:
	go test ./...

# Version and platform for local filesystem mirror (terraform init without registry).
PROVIDER_VERSION ?= 0.0.1
GOOS   ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
PLATFORM := $(GOOS)_$(GOARCH)
# e.g. registry.terraform.io/mistedo-cloud/mistedo/0.0.1/darwin_arm64
MIRROR_DIR ?= $(HOME)/.terraform.d/plugins/registry.terraform.io/mistedo-cloud/mistedo/$(PROVIDER_VERSION)/$(PLATFORM)

install: build
	@mkdir -p "$(MIRROR_DIR)"
	@cp terraform-provider-mistedo "$(MIRROR_DIR)/terraform-provider-mistedo_$(PROVIDER_VERSION)"
	@echo "Installed to $(MIRROR_DIR)"
	@echo "Use the filesystem_mirror block in ~/.terraformrc (see terraform.rc.example)."

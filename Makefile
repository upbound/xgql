# ====================================================================================
# Setup Project
PROJECT_NAME := xgql
PROJECT_REPO := github.com/negz/$(PROJECT_NAME)

PLATFORMS ?= linux_amd64 linux_arm64
-include build/makelib/common.mk

# Setup Output
-include build/makelib/output.mk

# Setup Go
NPROCS ?= 1
GO_TEST_PARALLEL := $(shell echo $$(( $(NPROCS) / 2 )))
GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/${PROJECT_NAME}
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.Version=$(VERSION)
GO_SUBDIRS += cmd internal
GO111MODULE = on
-include build/makelib/golang.mk

fallthrough: submodules
	@echo Initial setup complete. Running make again . . .
	@make

# Ensure a PR is ready for review.
reviewable: generate lint
	@go mod tidy

# Ensure branch is clean.
check-diff: reviewable
	@$(INFO) checking that branch is clean
	@test -z "$$(git status --porcelain)" || $(FAIL)
	@$(OK) branch is clean

# Update the submodules, such as the common build scripts.
submodules:
	@git submodule sync
	@git submodule update --init --recursive

.PHONY: reviewable submodules fallthrough

# ====================================================================================
# Special Targets

define CROSSPLANE_MAKE_HELP
Crossplane Targets:
    reviewable            Ensure a PR is ready for review.
    submodules            Update the submodules, such as the common build scripts.
    run                   Run crossplane locally, out-of-cluster. Useful for development.

endef
# The reason CROSSPLANE_MAKE_HELP is used instead of CROSSPLANE_HELP is because the crossplane
# binary will try to use CROSSPLANE_HELP if it is set, and this is for something different.
export CROSSPLANE_MAKE_HELP

crossplane.help:
	@echo "$$CROSSPLANE_MAKE_HELP"

help-special: crossplane.help

.PHONY: crossplane.help help-special

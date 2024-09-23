# Setup Project
PROJECT_NAME := xgql
PROJECT_REPO := github.com/upbound/$(PROJECT_NAME)

PLATFORMS ?= linux_amd64 linux_arm64
-include build/makelib/common.mk

# Setup Output
S3_BUCKET ?= public-upbound.releases/$(PROJECT_NAME)
-include build/makelib/output.mk

# Setup Go
NPROCS ?= 1
GO_REQUIRED_VERSION = 1.23.1
GOLANGCILINT_VERSION = 1.61.0
GO_LINT_ARGS ?= "--fix"
GO_TEST_PARALLEL := $(shell echo $$(( $(NPROCS) / 2 )))
GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/${PROJECT_NAME}
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.Version=$(VERSION)
GO_SUBDIRS += cmd internal
GO111MODULE = on
-include build/makelib/golang.mk

# Setup Node (for linting the schema)
-include build/makelib/yarnjs.mk

# Setup Helm
KIND_VERSION = v0.16.0
USE_HELM3 = true
HELM_BASE_URL = https://charts.upbound.io
HELM_S3_BUCKET = public-upbound.charts
HELM_CHARTS = xgql
HELM_CHART_LINT_ARGS_xgql = --set nameOverride='',imagePullSecrets=''
-include build/makelib/k8s_tools.mk
-include build/makelib/helm.mk                                                           
                                                                                         
# Setup Images
DOCKER_REGISTRY = upbound
IMAGES = xgql
OSBASEIMAGE = gcr.io/distroless/static:nonroot         
-include build/makelib/image.mk  

fallthrough: submodules
	@echo Initial setup complete. Running make again . . .
	@make

yarn.lint: yarn.install
	@cd $(YARN_DIR); $(YARN) format
	@cd $(YARN_DIR); $(YARN) lint

# Update the submodules, such as the common build scripts.
submodules:
	@git submodule sync
	@git submodule update --init --recursive

.PHONY: submodules fallthrough

# ====================================================================================
# Special Targets

define XGQL_MAKE_HELP
xgql Targets:
    submodules            Update the submodules, such as the common build scripts.

endef
# The reason XGQL_MAKE_HELP is used instead of XGQL_HELP is because the xgql
# binary will try to use XGQL_HELP if it is set, and this is for something different.
export XGQL_MAKE_HELP

xgql.help:
	@echo "$$XGQL_MAKE_HELP"

help-special: xgql.help

.PHONY: xgql.help help-special

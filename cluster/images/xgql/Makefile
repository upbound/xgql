PLATFORMS := linux_amd64 linux_arm64 linux_arm
include ../../../build/makelib/common.mk

IMAGE = $(BUILD_REGISTRY)/xgql-$(ARCH)
OSBASEIMAGE = gcr.io/distroless/static:nonroot
include ../../../build/makelib/image.mk

ifeq ($(PLATFORM),$(filter $(PLATFORM),darwin_amd64 windows_amd64)) 
$(info Skipping image build for $(PLATFORM))
img.build:
else
img.build:
	@$(INFO) docker build $(IMAGE)
	@cp Dockerfile $(IMAGE_TEMP_DIR) || $(FAIL)
	@cp $(OUTPUT_DIR)/bin/$(OS)_$(ARCH)/xgql $(IMAGE_TEMP_DIR) || $(FAIL)
	@cd $(IMAGE_TEMP_DIR) && $(SED_CMD) 's|BASEIMAGE|$(OSBASEIMAGE)|g' Dockerfile || $(FAIL)
	@docker build $(BUILD_ARGS) \
		--platform linux/$(ARCH) \
		-t $(IMAGE) \
		$(IMAGE_TEMP_DIR) || $(FAIL)
	@$(OK) docker build $(IMAGE)
endif

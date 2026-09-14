ANTITHESIS_DIR := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
REPO_ROOT := $(abspath $(ANTITHESIS_DIR)/../../..)

include $(REPO_ROOT)/build.assets/versions.mk
-include $(ANTITHESIS_DIR)/.env

GO_VERSION := $(patsubst go%,%,$(GOLANG_VERSION))

SUTS := core
SUT ?= core

TAG ?= $(shell REPO_PATH=$(REPO_ROOT) $(ANTITHESIS_DIR)/scripts/find_tag.sh )

REGISTRY ?=
PLATFORMS ?= linux/amd64,linux/arm64
LOCAL_PLATFORM ?= linux/$(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
LOCAL ?= 0
PUSH ?= $(if $(filter 1 true yes,$(LOCAL)),0,1)
LOAD ?= $(if $(filter 1 true yes,$(LOCAL)),1,0)

IMAGE_PREFIX = $(if $(REGISTRY),$(REGISTRY)/,)
DOCKER_PLATFORMS ?= $(if $(filter 1 true yes,$(LOCAL)),$(LOCAL_PLATFORM),$(PLATFORMS))
DOCKER_OUTPUT = $(if $(filter 1 true yes,$(PUSH)),--push,$(if $(filter 1 true yes,$(LOAD)),--load,))
BUILDER_FLAG = $(if $(BUILDER),--builder $(BUILDER),)
PLATFORM_FLAG = $(if $(DOCKER_PLATFORMS),--platform $(DOCKER_PLATFORMS),)
BUILD_ARGS = --build-arg GO_VERSION=$(GO_VERSION)

.PHONY: build-all
build-all: build-teleport build-tbot build-pki build-pg build-nginx build-k3s-rancher build-workload

TELEPORT_IMAGE ?= $(IMAGE_PREFIX)antithesis/teleport:$(TAG)
AUTH_IMAGE ?= $(IMAGE_PREFIX)antithesis/auth:$(TAG)
.PHONY:
build-teleport:
	docker buildx build \
		$(BUILDER_FLAG) \
		$(PLATFORM_FLAG) \
		$(BUILD_ARGS) \
		--target teleport \
		--tag $(TELEPORT_IMAGE) \
		$(DOCKER_OUTPUT) \
		--file $(ANTITHESIS_DIR)/docker/teleport/Dockerfile \
		$(REPO_ROOT)

	docker buildx build \
		$(BUILDER_FLAG) \
		$(PLATFORM_FLAG) \
		$(BUILD_ARGS) \
		--cache-from $(TELEPORT_IMAGE) \
		--target auth \
		--tag $(AUTH_IMAGE) \
		$(DOCKER_OUTPUT) \
		--file $(ANTITHESIS_DIR)/docker/teleport/Dockerfile \
		$(REPO_ROOT)

TBOT_IMAGE ?= $(IMAGE_PREFIX)antithesis/tbot:$(TAG)
.PHONY: build-tbot
build-tbot:
	docker buildx build \
		$(BUILDER_FLAG) \
		$(PLATFORM_FLAG) \
		$(BUILD_ARGS) \
		--target tbot \
		--tag $(TBOT_IMAGE) \
		$(DOCKER_OUTPUT) \
		--file $(ANTITHESIS_DIR)/docker/teleport/Dockerfile \
		$(REPO_ROOT)


PKI_IMAGE ?= $(IMAGE_PREFIX)antithesis/pki:$(TAG)
.PHONY: build-pki
build-pki:
	docker buildx build \
		$(BUILDER_FLAG) \
		$(PLATFORM_FLAG) \
		$(BUILD_ARGS) \
		--tag $(PKI_IMAGE) \
		$(DOCKER_OUTPUT) \
		--file $(ANTITHESIS_DIR)/docker/pki/Dockerfile \
		$(REPO_ROOT)

PG_IMAGE ?= $(IMAGE_PREFIX)antithesis/pg:$(TAG)
.PHONY: build-pg
build-pg:
	docker buildx build \
		$(BUILDER_FLAG) \
		$(PLATFORM_FLAG) \
		--tag $(PG_IMAGE) \
		$(DOCKER_OUTPUT) \
		$(ANTITHESIS_DIR)/docker/pg

NGINX_IMAGE ?= $(IMAGE_PREFIX)antithesis/nginx:$(TAG)
.PHONY: build-nginx
build-nginx:
	docker buildx build \
		$(BUILDER_FLAG) \
		$(PLATFORM_FLAG) \
		--tag $(NGINX_IMAGE) \
		$(DOCKER_OUTPUT) \
		--file $(ANTITHESIS_DIR)/docker/Dockerfile.nginx \
		$(ANTITHESIS_DIR)/docker

# When updating K3S_VERSION, delete the k3s-airgap-images-amd64.tar.zst file to force a re-download of the new version.
K3S_VERSION ?= v1.30.2-k3s1@sha256:0f499649d68b4528de51056e09197942a22dcc116779bea3a2709ebe5e3079c9
K3S_RANCHER_IMAGE ?= $(IMAGE_PREFIX)antithesis/k3s-rancher:$(TAG)
.PHONY: build-k3s-rancher
build-k3s-rancher:
	docker buildx build \
		$(BUILDER_FLAG) \
		$(PLATFORM_FLAG) \
		--build-arg K3S_VERSION=$(K3S_VERSION) \
		--tag $(K3S_RANCHER_IMAGE) \
		$(DOCKER_OUTPUT) \
		--file $(ANTITHESIS_DIR)/docker/k3s-rancher/Dockerfile \
		$(ANTITHESIS_DIR)/docker/k3s-rancher

WORKLOAD_IMAGE ?= $(IMAGE_PREFIX)antithesis/workload-$(SUT):$(TAG)
WORKLOAD_TARGET ?= workload-$(SUT)
.PHONY: build-workload
build-workload:
	docker buildx build \
		$(BUILDER_FLAG) \
		$(PLATFORM_FLAG) \
		$(BUILD_ARGS) \
		--target $(WORKLOAD_TARGET) \
		--tag $(WORKLOAD_IMAGE) \
		$(DOCKER_OUTPUT) \
		--file $(ANTITHESIS_DIR)/docker/teleport/Dockerfile \
		$(REPO_ROOT)

CONFIG_IMAGE ?= $(IMAGE_PREFIX)antithesis/config-$(SUT):$(TAG)
.PHONY: build-config
build-config: write-env build-k3s-preload
	docker buildx build \
		$(BUILDER_FLAG) \
		$(PLATFORM_FLAG) \
		--tag $(CONFIG_IMAGE) \
		$(DOCKER_OUTPUT) \
		$(ANTITHESIS_DIR)/sut/$(SUT)

WORKLOAD_GO_BIN_DIR ?= $(ANTITHESIS_DIR)/workloads/bin
WORKLOAD_GO_TAGS ?= antithesis
.PHONY: build-workload-go
build-workload-go:
	mkdir -p $(WORKLOAD_GO_BIN_DIR)
	cd $(ANTITHESIS_DIR)/workloads && CGO_ENABLED=0 go build -tags '$(WORKLOAD_GO_TAGS)' -o $(WORKLOAD_GO_BIN_DIR)/ ./cmd/...

## Build and load images locally
.PHONY: local
local: local-teleport local-tbot local-pki local-pg local-nginx local-k3s-rancher local-workload

.PHONY: local-teleport
local-teleport: LOCAL=1
local-teleport: build-teleport

.PHONY: local-tbot
local-tbot: LOCAL=1
local-tbot: build-tbot

.PHONY: local-pki
local-pki: LOCAL=1
local-pki: build-pki

.PHONY: local-pg
local-pg: LOCAL=1
local-pg: build-pg

.PHONY: local-nginx
local-nginx: LOCAL=1
local-nginx: build-nginx

.PHONY: local-k3s-rancher
local-k3s-rancher: LOCAL=1
local-k3s-rancher: build-k3s-rancher

.PHONY: local-workload
local-workload: LOCAL=1
local-workload: build-workload

K3S_PRELOAD_DIR ?= $(ANTITHESIS_DIR)/sut/$(SUT)/k3s/preload
K3S_VERSION_TAG := $(firstword $(subst @, ,$(K3S_VERSION)))
$(K3S_PRELOAD_DIR)/k3s-airgap-images-amd64.tar.zst: $(K3S_PRELOAD_DIR)/k3s-airgap-images-amd64.tar.zst.sha256 $(ANTITHESIS_DIR)/scripts/fetch_verify.sh
	$(ANTITHESIS_DIR)/scripts/fetch_verify.sh \
		https://github.com/k3s-io/k3s/releases/download/$(subst -k3s,%2Bk3s,$(K3S_VERSION_TAG)) \
		$(K3S_PRELOAD_DIR)/k3s-airgap-images-amd64.tar.zst.sha256 \
		$(K3S_PRELOAD_DIR) \
		k3s-airgap-images-amd64.tar.zst

K3S_PRELOAD_NGINX := nginx:1.31.1-alpine
K3S_PRELOAD_NGINX_DIGEST := sha256:8b1e78743a03dbb2c95171cc58639fef29abc8816598e27fb910ed2e621e589a
$(K3S_PRELOAD_DIR)/nginx.tar: $(ANTITHESIS_DIR)/Makefile
	if ! docker image inspect $(K3S_PRELOAD_NGINX)@$(K3S_PRELOAD_NGINX_DIGEST) >/dev/null 2>&1; then \
		docker pull $(K3S_PRELOAD_NGINX)@$(K3S_PRELOAD_NGINX_DIGEST); \
	fi
	docker tag $(K3S_PRELOAD_NGINX)@$(K3S_PRELOAD_NGINX_DIGEST) $(K3S_PRELOAD_NGINX)
	mkdir -p $(K3S_PRELOAD_DIR)
	docker save $(K3S_PRELOAD_NGINX) -o $@

# Locally k3s will be able to pull the images it needs. On Antithesis this is not the case, there is no network access
# we preload the amd64 airgap bundle to able able to run it.
.PHONY: build-k3s-preload
build-k3s-preload: $(K3S_PRELOAD_DIR)/k3s-airgap-images-amd64.tar.zst $(K3S_PRELOAD_DIR)/nginx.tar

## Write the env file copied into the Antithesis config image.
.PHONY: write-env
write-env:
	@printf '%s\n' \
		'# Generated by Makefile; used by docker compose and the Antithesis config image.' \
		'TAG=$(TAG)' \
		'REGISTRY=$(REGISTRY)' \
		> $(ANTITHESIS_DIR)/sut/$(SUT)/.env

.PHONY: trigger
trigger: build-config
	scripts/trigger.sh $(SUT)

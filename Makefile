GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOARCH=$(shell GOTOOLCHAIN=local $(GOCMD) env GOARCH)
GOOS=$(shell GOTOOLCHAIN=local $(GOCMD) env GOOS)

MAKEFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
BASE_PATH := $(patsubst %/,%,$(dir $(MAKEFILE_PATH)))
BUILD_PATH = $(BASE_PATH)/build
VERSION ?=
export VERSION
export MCP_GO := $(GOCMD)
BUILD_LDFLAGS := -s -w
ifneq ($(strip $(VERSION)),)
BUILD_LDFLAGS += -X github.com/1Panel-dev/mcp-1panel/utils.Version=$(VERSION)
endif

BIN_NAME=mcp-1panel

DOCKER_IMAGE ?= 1panel/mcp-server
DOCKER_PLATFORM ?= linux/$(GOARCH)
export DOCKER_IMAGE DOCKER_PLATFORM

.PHONY: build build_linux build_linux_all docker_build docker_save docker_test

build:
	@if [ -n "$$VERSION" ]; then sh "$(BASE_PATH)/scripts/validate-version.sh" "$$VERSION"; fi
	mkdir -p "$(BUILD_PATH)"
	cd "$(BASE_PATH)" \
	&& CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) -mod=readonly -buildvcs=false -trimpath -ldflags '$(BUILD_LDFLAGS)' -o "$(BUILD_PATH)/$(BIN_NAME)" .

build_linux:
	sh "$(BASE_PATH)/scripts/build-linux.sh" "$(BUILD_PATH)" "$(GOARCH)"

build_linux_all:
	$(MAKE) -f "$(MAKEFILE_PATH)" build_linux GOARCH=386
	$(MAKE) -f "$(MAKEFILE_PATH)" build_linux GOARCH=amd64
	$(MAKE) -f "$(MAKEFILE_PATH)" build_linux GOARCH=arm64
	$(MAKE) -f "$(MAKEFILE_PATH)" build_linux GOARCH=armv7
	$(MAKE) -f "$(MAKEFILE_PATH)" build_linux GOARCH=s390x
	$(MAKE) -f "$(MAKEFILE_PATH)" build_linux GOARCH=ppc64le
	$(MAKE) -f "$(MAKEFILE_PATH)" build_linux GOARCH=riscv64

# Build/load one platform into the local Docker Engine; never publish.
docker_build:
	sh "$(BASE_PATH)/scripts/docker-image.sh" build "$(BASE_PATH)" "$(BUILD_PATH)"

# Export a previously built image for docker load on the target server.
docker_save:
	sh "$(BASE_PATH)/scripts/docker-image.sh" save "$(BASE_PATH)" "$(BUILD_PATH)"

docker_test:
	python3 "$(BASE_PATH)/scripts/test-docker-image.py"

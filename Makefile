GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOARCH=$(shell go env GOARCH)
GOOS=$(shell go env GOOS )

BASE_PATH := $(shell pwd)
BUILD_PATH = $(BASE_PATH)/build

MAIN_PATH=$(BASE_PATH)/main.go
BIN_NAME=mcp-1panel

.PHONY: build

build:
	mkdir -p $(BUILD_PATH)
	cd $(BASE_PATH) \
	&& GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) -trimpath -ldflags '-s -w' -o $(BUILD_PATH)/$(BIN_NAME) $(MAIN_PATH)
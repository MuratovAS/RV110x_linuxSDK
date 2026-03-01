ifeq ($(APP_PARAM), )
    APP_PARAM:=../Makefile.param
    include $(APP_PARAM)
endif

export LC_ALL=C
SHELL:=/bin/bash

CURRENT_DIR    := $(shell pwd)
PKG_NAME       := uipm
PKG_BIN        := $(PKG_NAME)
PKG_BIN_INSTALL_DIR := out
PKG_BIN_INSTALL     := $(PKG_BIN_INSTALL_DIR)/root/opt/uipm

VERSION        := $(shell git describe --tags --always --dirty=-dirty 2>/dev/null || echo dev)

# Go cross-compilation settings for ARM uclibc
GOARCH      := arm
GOOS        := linux
GOARM       := 7
CGO_ENABLED := 0

ifeq ($(RK_ENABLE_UIPM),y)
PKG_TARGET += build-uipm
endif

all: $(PKG_TARGET)
	@echo "uipm done"

build-frontend:
	@echo "Building $(PKG_NAME) frontend on host..."
	@cd $(CURRENT_DIR) && npm run build

build-backend:
	@echo "Building $(PKG_NAME) backend for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(CURRENT_DIR)/$(PKG_BIN_INSTALL)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) \
		go build -ldflags="-s -w -X main.version=$(VERSION)" \
		-o $(CURRENT_DIR)/$(PKG_BIN_INSTALL)/$(PKG_BIN) \
		*.go

build-uipm: build-frontend build-backend
	@cp -r $(CURRENT_DIR)/dist $(CURRENT_DIR)/$(PKG_BIN_INSTALL)/
	$(call MAROC_COPY_PKG_TO_APP_OUTPUT, $(RK_APP_OUTPUT), $(PKG_BIN_INSTALL_DIR))

clean:
	@rm -rf $(CURRENT_DIR)/$(PKG_BIN_INSTALL_DIR)

info:
ifeq ($(RK_ENABLE_UIPM),y)
	@echo "$(PKG_NAME): UI Package Manager web app (GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM))"
endif

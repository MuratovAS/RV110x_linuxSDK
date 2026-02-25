ifeq ($(APP_PARAM), )
    APP_PARAM:=../Makefile.param
    include $(APP_PARAM)
endif

export LC_ALL=C
SHELL:=/bin/bash

CURRENT_DIR := $(shell pwd)
PKG_NAME    := go_hello
PKG_BIN     := $(PKG_NAME)
PKG_BIN_INSTALL_DIR := out
PKG_BIN_INSTALL     := $(PKG_BIN_INSTALL_DIR)/opt/go_hello

# Go cross-compilation settings for ARM uclibc
GOARCH      := arm
GOOS        := linux
GOARM       := 7
CGO_ENABLED := 0

ifeq ($(RK_ENABLE_GO_HELLO),y)
PKG_TARGET += build-go-hello
endif

all: $(PKG_TARGET)
	@echo "go_hello done"

build-go-hello:
	@echo "Building $(PKG_NAME) for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(CURRENT_DIR)/$(PKG_BIN_INSTALL)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) \
		go build -ldflags="-s -w" -o $(CURRENT_DIR)/$(PKG_BIN_INSTALL)/$(PKG_BIN) $(CURRENT_DIR)/main.go
	$(call MAROC_COPY_PKG_TO_APP_OUTPUT, $(RK_APP_OUTPUT), $(PKG_BIN_INSTALL_DIR))

clean:
	@rm -rf $(CURRENT_DIR)/$(PKG_BIN_INSTALL_DIR)

info:
ifeq ($(RK_ENABLE_GO_HELLO),y)
	@echo "$(PKG_NAME): Hello World in Go (GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM))"
endif

APP := syl-listing-pro-x
GO ?= go
BIN_DIR ?= bin
BIN := $(BIN_DIR)/$(APP)
DESTDIR ?=
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X 'github.com/hooziwang/syl-listing-pro-x/cmd.Version=$(VERSION)' -X 'github.com/hooziwang/syl-listing-pro-x/cmd.Commit=$(COMMIT)' -X 'github.com/hooziwang/syl-listing-pro-x/cmd.BuildTime=$(BUILD_TIME)'
GO_BIN_DIR ?= $(shell sh -c 'gobin="$$( $(GO) env GOBIN )"; if [ -n "$$gobin" ]; then printf "%s" "$$gobin"; else gopath="$$( $(GO) env GOPATH )"; printf "%s/bin" "$${gopath%%:*}"; fi')
INSTALL_BIN_DIR := $(DESTDIR)$(GO_BIN_DIR)
INSTALL_BIN := $(INSTALL_BIN_DIR)/$(APP)

.PHONY: default test build clean install uninstall

DEFAULT_GOAL := default

default: test build install

test:
	$(GO) test ./...

build:
	mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) .

clean:
	rm -rf $(BIN_DIR)

install: build
	mkdir -p "$(INSTALL_BIN_DIR)"
	install -m 0755 "$(BIN)" "$(INSTALL_BIN)"

uninstall:
	rm -f "$(INSTALL_BIN)"

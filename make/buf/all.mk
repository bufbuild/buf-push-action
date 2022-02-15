PROJECT := buf-push-action

# Settable
CACHE ?= $(HOME)/.cache/$(PROJECT)
CACHE_BIN := $(CACHE)/bin
CACHE_VERSIONS := $(CACHE)/versions

# Settable
GOARCH ?= $(shell uname -m)
ifeq ($(GOARCH), x86_64)
	GOARCH := amd64
else ifeq ($(GOARCH), x86)
	GOARCH := 386
endif

# Settable
GOOS ?= $(shell echo $(shell uname -s) | tr '[:upper:]' '[:lower:]')
ifeq ($(findstring mingw,$(GOOS)),mingw)
	GOOS := windows
else ifeq ($(findstring msys,$(GOOS)),msys)
	GOOS := windows
else ifeq ($(findstring cygwin,$(GOOS)),cygwin)
	GOOS := windows
endif

include make/buf/dep_yq.mk

.PHONY: test
test:
	./test/test.bash

.PHONY: yq
yq: $(YQ)

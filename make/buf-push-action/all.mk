GO_BINS := $(GO_BINS) cmd/buf-push-action
DOCKER_BINS := $(DOCKER_BINS) buf-push-action

LICENSE_HEADER_LICENSE_TYPE := apache
LICENSE_HEADER_COPYRIGHT_HOLDER := Buf Technologies, Inc.
LICENSE_HEADER_YEAR_RANGE := 2020-2022
LICENSE_HEADER_IGNORES := \/testdata
FILE_IGNORES := $(FILE_IGNORES) \
	.build/

include make/go/bootstrap.mk
include make/go/go.mk
include make/go/docker.mk
include make/go/buf.mk
include make/go/license_header.mk
include make/go/dep_protoc_gen_go.mk
include make/go/dep_minisign.mk

bufgeneratedeps::

.PHONY: bufgeneratecleango
bufgeneratecleango:

bufgenerateclean:: bufgeneratecleango

.PHONY: bufgeneratego
bufgeneratego:

bufgeneratesteps:: bufgeneratego

.PHONY: release
release: $(MINISIGN)
	DOCKER_IMAGE=golang:1.17.7-buster bash make/buf-push-action/scripts/release.bash

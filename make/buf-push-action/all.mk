GO_BINS := $(GO_BINS) cmd/buf-push-action

LICENSE_HEADER_LICENSE_TYPE := apache
LICENSE_HEADER_COPYRIGHT_HOLDER := Buf Technologies, Inc.
LICENSE_HEADER_YEAR_RANGE := 2020-2022
LICENSE_HEADER_IGNORES := \/testdata
FILE_IGNORES := $(FILE_IGNORES) .build/

# Set a default value for OPEN_CMD so that _assert_var,OPEN_CMD doesn't fail in go.mk if the
# OS is neither Darwin nor Linux.
OPEN_CMD := open

include make/go/bootstrap.mk
include make/go/go.mk
include make/go/license_header.mk

bufgeneratedeps::

.PHONY: bufgeneratecleango
bufgeneratecleango:

bufgenerateclean:: bufgeneratecleango

.PHONY: bufgeneratego
bufgeneratego:

bufgeneratesteps:: bufgeneratego

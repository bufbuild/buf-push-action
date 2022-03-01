#!/usr/bin/env bash
# usage: downloadbinary.bash <git-ref> <output-dir>
#
# checks for the existence of <output-dir>/buf-push-action. If it doesn't exist, downloadbinary.bash downloads it from
# github using <git-ref> as the release tag. If it can't download the buf-push-action binary, it builds it from source.

set -eo pipefail

# writes github action workflow message
# valid types are: "notice", "warning" and "error"
# only the first line is displayed in the github action workflow
workflow_message() {
  local message_type="$1"
  local message="$2"
  echo "::$message_type::$message"
}

# error then exit 1
fail() {
  workflow_message error "$1"
  exit 1
}

if [ $# -ne 2 ]; then
  echo "Usage: $0 <ref> <output_dir>" >&2
  exit 1
fi

REF="$1"
OUTPUT_DIR="$2"

if [ -f "${OUTPUT_DIR}/buf-push-action" ]; then
  workflow_message notice "buf-push-action already exists, skipping download"
  exit 0
fi

# The version of go to use if we have to resort to build_from_source
GO_VERSION="1.17.7"

ARCH="$(uname -m)"
OS="$(uname -s)"
case "${OS}" in
  msys*) OS="Windows" ;;
  mingw*) OS="Windows" ;;
  cygwin*) OS="Windows" ;;
  Linux)
    if [ "${ARCH}" = "arm64" ]; then
      ARCH="aarch64"
    fi
    ;;
esac

DOWNLOAD_ARTIFACT_NAME="buf-push-action-${OS}-${ARCH}"
if [ "${OS}" = "Windows" ]; then
  DOWNLOAD_ARTIFACT_NAME="${DOWNLOAD_ARTIFACT_NAME}.exe"
fi

# for v1 we always download the latest and never build from source
if [ "${REF}" = "v1" ]; then
  gh release download --repo bufbuild/buf-push-action --dir "${OUTPUT_DIR}" --pattern "${DOWNLOAD_ARTIFACT_NAME}"
  mv "${OUTPUT_DIR}/${DOWNLOAD_ARTIFACT_NAME}" "${OUTPUT_DIR}/buf-push-action"
  chmod +x "${OUTPUT_DIR}/buf-push-action"
  exit 0
fi

# try to download the release if available
if gh release download "${REF}" --repo bufbuild/buf-push-action --dir "${OUTPUT_DIR}" --pattern "${DOWNLOAD_ARTIFACT_NAME}"; then
  mv "${OUTPUT_DIR}/${DOWNLOAD_ARTIFACT_NAME}" "${OUTPUT_DIR}/buf-push-action"
  chmod +x "${OUTPUT_DIR}/buf-push-action"
  exit 0
fi

# GOBIN needs to be the absolute path to OUTPUT_DIR
mkdir -p "${OUTPUT_DIR}"
GOBIN="$(cd "$(dirname "${OUTPUT_DIR}")"; pwd -P)/$(basename "${OUTPUT_DIR}")"
export GOBIN
PATH="${GOBIN}:${PATH}"

# as a last resort, build from source
GO_CMD="go"
INSTALLED_GO_VERSION="$(go version | cut -d ' ' -f 3)" || fail "go is not installed"
if [ "${INSTALLED_GO_VERSION}" != "${GO_VERSION}" ]; then
  # When we have the wrong version of go, use the "go get" method to download the correct version to avoid affecting
  # whatever version of go is installed on the system.
  GO111MODULE=off go get "golang.org/dl/go${GO_VERSION}" > /dev/null || fail "could not get go${GO_VERSION}"
  "go${GO_VERSION}" download > /dev/null  || fail "could not download go${GO_VERSION}"
  GO_CMD="go${GO_VERSION}"
fi

"${GO_CMD}" install ./cmd/buf-push-action || fail "could not install buf-push-action"

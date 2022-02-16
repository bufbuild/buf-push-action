#!/usr/bin/env bash

set -eo pipefail

fail() {
  echo "::error::$1"
  exit 1
}

if [ $# -ne 2 ]; then
  echo "Usage: $0 <input> <track>" >&2
  exit 1
fi

BUF_INPUT="$1"
BUF_TRACK="$2"

# Make sure the token isn't accidentally logged
echo "::add-mask::${BUF_TOKEN}"

if [ -z "${GITHUB_SHA}" ]; then
  fail "the commit was not provided"
fi

if [ -z "${BUF_TOKEN}" ]; then
  fail "a buf authentication token was not provided"
fi

NOT_INSTALLED_MESSAGE='buf is not installed; please add the "bufbuild/buf-setup-action" step to your job found at https://github.com/bufbuild/buf-setup-action'

BUF_COMMAND="$(type -P buf)" || fail "$NOT_INSTALLED_MESSAGE"

if [ -z "$BUF_COMMAND" ]; then
  fail "$NOT_INSTALLED_MESSAGE"
fi

TRACK_ARG=""
if [ "$BUF_TRACK" != "main" ]; then
  TRACK_ARG="--track=$BUF_TRACK"

  "${BUF_COMMAND}" push "${TRACK_ARG}" --help >/dev/null 2>&1 ||
    fail "The installed version of buf does not support setting the track. Please use buf v1.0.0-rc11 or newer."
fi

BUF_TOKEN="${BUF_TOKEN}" "${BUF_COMMAND}" push --tag "${GITHUB_SHA}" "${TRACK_ARG}" "${BUF_INPUT}"

#!/usr/bin/env bash

set -eo pipefail

DIR="$(cd "$(dirname "${0}")" && pwd)"
# Don't cd to $DIR because BUF_INPUT will be relative to the working directory.

. "${DIR}/lib.bash"

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

BUF_COMMAND="$(type -P buf)" || fail "${NOT_INSTALLED_MESSAGE}"

if [ -z "${BUF_COMMAND}" ]; then
  fail "${NOT_INSTALLED_MESSAGE}"
fi

if [ "${BUF_TRACK}" != "main" ]; then
  buf_supports_track "${BUF_COMMAND}" ||
    fail "The installed version of buf does not support setting the track. Please use buf v1.0.0-rc11 or newer."

  BUF_TOKEN="${BUF_TOKEN}" "${BUF_COMMAND}" push --tag "${GITHUB_SHA}" --track "${BUF_TRACK}" "${BUF_INPUT}"
  exit 0
fi

BUF_TOKEN="${BUF_TOKEN}" "${BUF_COMMAND}" push --tag "${GITHUB_SHA}" "${BUF_INPUT}"

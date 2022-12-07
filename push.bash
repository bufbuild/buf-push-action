#!/usr/bin/env bash

set -eo pipefail

fail() {
  echo "::error::$1"
  exit 1
}

if [ $# -ne 1 ]; then
  echo "Usage: $0 <input>" >&2
  exit 1
fi

BUF_INPUT="$1"

# Make sure the token isn't accidentally logged
echo "::add-mask::${BUF_TOKEN}"

if [ -z "${GITHUB_SHA}" ]; then
  fail "the commit was not provided"
fi

if [ -z "${GITHUB_REF_NAME}" ]; then
  fail "the commit ref was not provided"
fi

if [ -z "${GITHUB_REF_TYPE}" ]; then
  fail "the commit ref type was not provided"
fi

if [ -z "${BUF_TOKEN}" ]; then
  fail "a buf authentication token was not provided"
fi

NOT_INSTALLED_MESSAGE='buf is not installed; please add the "bufbuild/buf-setup-action" step to your job found at https://github.com/bufbuild/buf-setup-action'

BUF_COMMAND="$(type -P buf)" || fail "$NOT_INSTALLED_MESSAGE"

if [ -z "$BUF_COMMAND" ]; then
  fail "$NOT_INSTALLED_MESSAGE"
fi

if [ "${GITHUB_REF_TYPE}" != "branch" ]; then
  echo "reference type is not branch, skipping" >&2
  exit 0
fi

BUF_ARGS=("--tag" "${GITHUB_SHA}")
if [ "${DRAFT}" == "true" ]; then
  # Check that --draft is supported by running "buf push --draft example --help"
  # and checking for "unknown flag: --draft" in the output.
  set +e
  BUF_HELP_OUTPUT="$("${BUF_COMMAND}" push --draft example --help 2>&1)"
  set -e
  if [[ "${BUF_HELP_OUTPUT}" == *"unknown flag: --draft"* ]]; then
    fail "The installed version of buf does not support setting the draft. Please use buf v1.7.0 or newer."
  fi

  BUF_ARGS=("--draft" "${GITHUB_REF_NAME}")
fi

BUF_TOKEN="${BUF_TOKEN}" "${BUF_COMMAND}" "push" "${BUF_INPUT}" "${BUF_ARGS[@]}"

#!/usr/bin/env bash

set -eo pipefail

fail() {
  echo "::error::$1"
  exit 1
}

output_notice() {
  echo "::notice::$1"
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

BUF_COMMAND="$(type -P buf)" || fail "${NOT_INSTALLED_MESSAGE}"

if [ -z "${BUF_COMMAND}" ]; then
  fail "${NOT_INSTALLED_MESSAGE}"
fi

BUF_ARGS=("push" "--tag" "${GITHUB_SHA}" "${BUF_INPUT}")
if [ "${BUF_TRACK}" != "main" ]; then
  BUF_ARGS+=("--track" "${BUF_TRACK}")

  # Check that --track is supported by running "buf push --track example --help"
  # and checking for "unknown flag: --track" in the output.
  set +e
  BUF_HELP_OUTPUT="$("${BUF_COMMAND}" push --track example --help 2>&1)"
  set -e
  if [[ "${BUF_HELP_OUTPUT}" == *"unknown flag: --track"* ]]; then
    fail "The installed version of buf does not support setting the track. Please use buf v1.0.0-rc11 or newer."
  fi
fi

BUF_OUT_DIR="$(mktemp -d)"
STDOUT_FILE="${BUF_OUT_DIR}/stdout.txt"
STDERR_FILE="${BUF_OUT_DIR}/stderr.txt"
touch "${STDOUT_FILE}" "${STDERR_FILE}"

"${BUF_COMMAND}" "${BUF_ARGS[@]}" >"${STDOUT_FILE}" 2>"${STDERR_FILE}" || fail "$(cat "${STDERR_FILE}")"
STDOUT="$(cat "${STDOUT_FILE}")"
STDERR="$(cat "${STDERR_FILE}")"
rm -rf "${BUF_OUT_DIR}"
[ -z "${STDOUT}" ] || output_notice "pushed commit ${STDOUT}"
# If we have stderr after getting exit code 0, then the message is
# "The latest commit has the same content; not creating a new commit."
# We want to output that as a notice.
[ -z "${STDERR}" ] || output_notice "${STDERR}"

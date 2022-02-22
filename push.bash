#!/usr/bin/env bash

set -eo pipefail

DIR="$(cd "$(dirname "${0}")" && pwd)"

# lib to source. This is here for dependency injection.
: "${LIB:="${DIR}/lib.bash"}"
# shellcheck source=lib.bash
. "${LIB}"

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

# sets an output value for the github action step
set_output() {
  echo "::set-output name=$1::$2"
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

CONFIG_FILE="${BUF_INPUT}/buf.yaml"

if [ ! -f "${CONFIG_FILE}" ]; then
  fail "Config file not found: ${CONFIG_FILE}"
fi

MODULE_NAME="$(yq eval --exit-status '.name' "${CONFIG_FILE}" 2>/dev/null)" ||
  fail "name not found in ${CONFIG_FILE}"

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
# The only stdout from buf push is the name of the pushed commit.
BUF_COMMIT="$(cat "${STDOUT_FILE}")"
STDERR="$(cat "${STDERR_FILE}")"
rm -rf "${BUF_OUT_DIR}"
[ -z "${BUF_COMMIT}" ] || workflow_message notice "pushed commit ${BUF_COMMIT}"

# If we have stderr after getting exit code 0, then the message is
# "The latest commit has the same content; not creating a new commit."
# We want to output that as a notice.
[ -z "${STDERR}" ] || workflow_message notice "${STDERR}"

# When push is successful but no commit is returned, that means there was no digest change. We need to get the
# commit from from "buf beta registry commit get"
if [ -z "${BUF_COMMIT}" ]; then
  BUF_COMMIT="$(
    BUF_TOKEN="${BUF_TOKEN}" "${BUF_COMMAND}" beta registry commit get "${MODULE_NAME}" --format=json |
      jq -r '.commit'
  )"
fi

set_output commit "${BUF_COMMIT}"
set_output commit_url "https://${MODULE_NAME}/tree/${BUF_COMMIT}"
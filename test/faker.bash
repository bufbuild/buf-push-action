#!/usr/bin/env bash

set -euo pipefail

DIR="$(cd "$(dirname "${0}")/.." && pwd)"
cd "${DIR}"

fail () {
  echo -e "$1" > "${FAILURE_LOG_FILE}"
  set +u
  if [ -n "${CI}" ]; then
    echo "::error::$1"
  else
    echo "FAIL: $1" >&2
  fi
  set -u
  exit 1
}

EXPECTATIONS="$(cat "${EXPECTATIONS_FILE}")"
EXPECTATION="$(echo "${EXPECTATIONS}" | jq -r '.[0]')"
echo "${EXPECTATIONS}" | jq 'del(.[0])' > "${BUF_EXPECTATIONS_FILE}"
[ "${EXPECTATION}" != "null" ] || fail "unexpected call to ${COMMAND}:\n got: $*\n expected no call"
EXPECTED_ARGS="$(echo "${EXPECTATION}" | jq -r '.args')"
[ "$*" = "${EXPECTED_ARGS}" ] || fail "unexpected call to ${COMMAND}:\n got: $*\n expected: ${EXPECTED_ARGS}"

EXPECTED_ENV="$(echo "${EXPECTATION}" | jq -r '.env_vars')"
if [ "${EXPECTED_ENV}" != "null" ]; then
  for env_pair in $(echo "$EXPECTED_ENV" | jq -r '.[]'); do
    ENV_KEY="$(echo "${env_pair}" | cut -d '=' -f 1)"
    ENV_VALUE="$(echo "${env_pair}" | cut -d '=' -f 2)"
    if ! [ -v "$ENV_KEY" ]; then
      fail "Missing environment variable: $ENV_KEY"
    fi
    if [ "$ENV_VALUE" != "${!ENV_KEY}" ]; then
      fail "Environment variable mismatch: $ENV_KEY, expected: $ENV_VALUE, got: ${!ENV_KEY}"
    fi
  done
fi

echo "${EXPECTATION}" | jq -r '.stderr' >&2
echo "${EXPECTATION}" | jq -r '.stdout'
exit "$(echo "${EXPECTATION}" | jq -r '.exit_code')"

#!/usr/bin/env bash
# This is meant to be substituted for the real buf command when testing buf-push-action.

set -euo pipefail

fail() {
  set +u
  if [ -n "${CI}" ]; then
    echo "::error::$1"
  else
    echo "FAIL: $1" >&2
  fi
  exit 1
}

: "${OLD_BUF_VERSION:=}"

USAGE_MESSAGE="Usage:
  buf push <source> [flags]
  ...
"

# hardcode the check for --track support.
if [ "$*" = "push --track example --help" ]; then
  if [ -n "${OLD_BUF_VERSION}" ]; then
    echo "${USAGE_MESSAGE}unknown flag: --track" >&2
    exit 1
  fi
  echo -e "${USAGE_MESSAGE}"
  exit 0
fi

if [ "${BUF_TOKEN}" != "${WANT_BUF_TOKEN}" ]; then
  fail "buf-push-action got wrong BUF_TOKEN: '${BUF_TOKEN}' wanted '${WANT_BUF_TOKEN}'"
fi

case "$*" in
  "${WANT_ARGS}")
    ;;
  "beta registry commit get buf.build/example/repo --format=json")
    printf '{"commit":"%s"}\n' "${TEST_BSR_COMMIT}"
    exit 0
    ;;
  *)
    fail "buf-push-action got wrong args: '$*' wanted '${WANT_ARGS}'"
    ;;
esac

: "${NO_DIGEST_CHANGE:=""}"

if [ -n "${NO_DIGEST_CHANGE}" ]; then
  echo "The latest commit has the same content, not creating a new commit." >&2
  exit 0
fi

echo "${TEST_BSR_COMMIT}"

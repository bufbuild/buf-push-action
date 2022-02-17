#!/usr/bin/env bash
# This is meant to be substituted for the real buf command when testing buf-push-action.

set -euo pipefail

fail() {
  set +u
  if [ -n "$CI" ]; then
    echo "::error::$1"
  else
    echo "FAIL: $1" >&2
  fi
  exit 1
}

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

if [ "$BUF_TOKEN" != "$WANT_BUF_TOKEN" ]; then
  fail "buf-push-action got wrong BUF_TOKEN: '$BUF_TOKEN' wanted '$WANT_BUF_TOKEN'"
fi

GOT_ARGS="$(echo "$*" | tr -s ' ')"
if [ "$GOT_ARGS" != "$WANT_ARGS" ]; then
  fail "buf-push-action got wrong args: '$*' wanted '$WANT_ARGS'"
fi

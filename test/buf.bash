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

unset GITHUB_REF_NAME GITHUB_REF_TYPE

GITHUB_REF_NAME=main
GITHUB_REF_TYPE=branch

if [ "$BUF_TOKEN" != "$WANT_BUF_TOKEN" ]; then
  fail "buf-push-action got wrong BUF_TOKEN: '$BUF_TOKEN' wanted '$WANT_BUF_TOKEN'"
fi

if [ "$*" != "$WANT_ARGS" ]; then
  fail "buf-push-action got wrong args: '$*' wanted '$WANT_ARGS'"
fi

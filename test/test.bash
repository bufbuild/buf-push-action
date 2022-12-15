#!/usr/bin/env bash

set -euo pipefail

DIR="$(cd "$(dirname "${0}")/.." && pwd)"
cd "${DIR}"

mkdir -p "${DIR}/tmp/test/bin"
trap 'rm -rf "${DIR}/tmp/test"' EXIT

PATH="${DIR}/tmp/test/bin:$PATH"

cp test/buf.bash tmp/test/bin/buf
chmod +x tmp/test/bin/buf

# prevent the GITHUB_SHA and GITHUB_REF_NAME set by actions from being used in test
unset GITHUB_SHA GITHUB_REF_NAME

test_push() {
  export GITHUB_SHA GITHUB_REF_NAME BUF_TOKEN DRAFT WANT_BUF_TOKEN WANT_ARGS
  set +e
  ./push.bash "$@" > tmp/test/stdout 2> tmp/test/stderr
  GOT_EXIT_CODE="${?}"
  set -e
  if [ "${WANT_STDERR}" != "$(cat tmp/test/stderr)" ]; then
    echo "UNEXPECTED STDERR:" >&2
    diff -u <(echo "${WANT_STDERR}") <(cat tmp/test/stderr) >&2
    exit 1
  fi
  if [ "${WANT_STDOUT}" != "$(cat tmp/test/stdout)" ]; then
    echo "UNEXPECTED STDOUT:" >&2
    diff -u <(echo "${WANT_STDOUT}") <(cat tmp/test/stdout) >&2
    exit 1
  fi
  if [ -n "${WANT_EXIT_CODE}" ]; then
    if [ "${WANT_EXIT_CODE}" != "${GOT_EXIT_CODE}" ]; then
      echo "Expected exit code ${WANT_EXIT_CODE}, got ${GOT_EXIT_CODE}"
      exit 1
    fi
  fi
  rm -f tmp/test/stdout tmp/test/stderr
  unset GITHUB_SHA GITHUB_REF_NAME BUF_TOKEN WANT_BUF_TOKEN WANT_ARGS
}

echo "testing happy path"
GITHUB_SHA=fake-sha
GITHUB_REF_NAME=main
BUF_TOKEN=fake-token
WANT_BUF_TOKEN=fake-token
WANT_ARGS="push some/input/path --tag fake-sha"
WANT_STDOUT="::add-mask::fake-token"
WANT_STDERR=""
WANT_EXIT_CODE=0
test_push some/input/path
echo "ok"

echo "testing happy path draft"
GITHUB_SHA=fake-sha
GITHUB_REF_NAME=fake-ref
BUF_TOKEN=fake-token
DRAFT=true
WANT_BUF_TOKEN=fake-token
WANT_ARGS="push some/input/path --draft fake-ref"
WANT_STDOUT="::add-mask::fake-token"
WANT_STDERR=""
WANT_EXIT_CODE=0
test_push some/input/path
echo "ok"

echo "testing happy path draft on main branch"
GITHUB_SHA=fake-sha
GITHUB_REF_NAME=main
BUF_TOKEN=fake-token
DRAFT=true
WANT_BUF_TOKEN=fake-token
WANT_ARGS="push some/input/path --draft main" # we don't handle this within the action, but this should fail in the server side
WANT_STDOUT="::add-mask::fake-token"
WANT_STDERR=""
WANT_EXIT_CODE=0
test_push some/input/path
echo "ok"

echo "testing no input"
GITHUB_SHA=fake-sha
GITHUB_REF_NAME=main
BUF_TOKEN=fake-token
WANT_STDOUT=""
WANT_STDERR="Usage: ./push.bash <input>"
WANT_EXIT_CODE=1
test_push
echo "ok"

echo "testing no GITHUB_SHA"
BUF_TOKEN=fake-token
WANT_STDOUT='::add-mask::fake-token
::error::the commit was not provided'
WANT_STDERR=""
WANT_EXIT_CODE=1
test_push some/input/
echo "ok"

echo "testing no BUF_TOKEN"
GITHUB_SHA=fake-sha
GITHUB_REF_NAME=main
WANT_STDOUT='::add-mask::
::error::a buf authentication token was not provided'
WANT_STDERR=""
WANT_EXIT_CODE=1
test_push some/input/path
echo "ok"

#!/usr/bin/env bash

set -euo pipefail

DIR="$(cd "$(dirname "${0}")/.." && pwd)"
cd "${DIR}"

mkdir -p "${DIR}/tmp/test/bin"
trap 'rm -rf "${DIR}/tmp/test"' EXIT

PATH="${DIR}/tmp/test/bin:$PATH"

cp test/buf.bash tmp/test/bin/buf
chmod +x tmp/test/bin/buf
cp test/gh.bash tmp/test/bin/gh
chmod +x tmp/test/bin/gh

export TEST_BSR_COMMIT="feedfacecafefeedfacecafefeedface"
export BUF_EXPECTATIONS_FILE="${DIR}/tmp/test/buf.expectations"
export BUF_FAILURE_LOG_FILE="${DIR}/tmp/test/buf.failures.txt"
echo "[]" > "${BUF_EXPECTATIONS_FILE}"

export GH_EXPECTATIONS_FILE="${DIR}/tmp/test/gh.expectations"
export GH_FAILURE_LOG_FILE="${DIR}/tmp/test/gh.failures.txt"
echo "[]" > "${GH_EXPECTATIONS_FILE}"

export REPO_ROOT="${DIR}"

expect() {
  local args="$1" && shift
  local exit_code="$1" && shift
  local stdout="$1" && shift
  local stderr="$1" && shift
  local env_vars="[]"
  local expectation
    expectation="$(
      echo "{}" | jq \
        --arg args "${args}" \
        --arg exit_code "${exit_code}" \
        --arg stdout "${stdout}" \
        --arg stderr "${stderr}" \
        --arg env_vars "${env_vars}" \
        '{args: $args, exit_code: $exit_code, stdout: $stdout, stderr: $stderr, env_vars: []}'
    )"
    for arg in "$@"; do
      expectation="$(echo "${expectation}" | jq --arg arg "${arg}" '.env_vars += [$arg]')"
    done
    local expectations
    expectations="$(cat "${EXPECTATIONS_FILE}")"
    echo "${expectations}"  | jq  ". += [$expectation]" > "${EXPECTATIONS_FILE}"
}

buf_expect() {
  EXPECTATIONS_FILE="${BUF_EXPECTATIONS_FILE}" expect "$@"
}

gh_expect() {
  EXPECTATIONS_FILE="${GH_EXPECTATIONS_FILE}" expect "$@"
}

# prevent the GITHUB_SHA set by actions from being used in test
unset GITHUB_SHA

BUF_USAGE_MESSAGE="Usage:
  buf push <source> [flags]
  ...
"

test_push() {
    rm -f "${BUF_FAILURE_LOG_FILE}"
    export GITHUB_SHA BUF_TOKEN
    set +e
    ./push.bash "$@" > tmp/test/stdout 2> tmp/test/stderr
    GOT_EXIT_CODE="${?}"
    set -e
    RETURN_CODE=0
    if [ "${WANT_STDERR}" != "$(cat tmp/test/stderr)" ]; then
      echo "UNEXPECTED STDERR:" >&2
      diff -u <(echo "${WANT_STDERR}") <(cat tmp/test/stderr) >&2
      RETURN_CODE=1
    fi
    if [ "${WANT_STDOUT}" != "$(cat tmp/test/stdout)" ]; then
      echo "UNEXPECTED STDOUT:" >&2
      diff -u <(echo "${WANT_STDOUT}") <(cat tmp/test/stdout) >&2
      RETURN_CODE=1
    fi
    if [ -n "${WANT_EXIT_CODE}" ]; then
      if [ "${WANT_EXIT_CODE}" != "${GOT_EXIT_CODE}" ]; then
        echo "Expected exit code ${WANT_EXIT_CODE}, got ${GOT_EXIT_CODE}"
        RETURN_CODE=1
      fi
    fi
    if [ -f "${BUF_FAILURE_LOG_FILE}" ]; then
      echo "UNEXPECTED FAILURES:" >&2
      cat "${BUF_FAILURE_LOG_FILE}" >&2
      RETURN_CODE=1
    fi
    unset GITHUB_SHA BUF_TOKEN
    return "${RETURN_CODE}"
}

echo "testing happy path"
GITHUB_SHA=fake-sha
BUF_TOKEN=fake-token
WANT_STDOUT="::add-mask::fake-token
::notice::pushed commit ${TEST_BSR_COMMIT}
::set-output name=commit::${TEST_BSR_COMMIT}
::set-output name=commit_url::https://buf.build/example/repo/tree/${TEST_BSR_COMMIT}"
WANT_STDERR=""
WANT_EXIT_CODE=0
buf_expect \
  "push --tag fake-sha test/proto" \
  "0" \
  "${TEST_BSR_COMMIT}" \
  ""
test_push test/proto main
echo ok

echo "testing no digest change"
GITHUB_SHA=fake-sha
BUF_TOKEN=fake-token
WANT_STDOUT="::add-mask::fake-token
::notice::The latest commit has the same content, not creating a new commit.
::set-output name=commit::${TEST_BSR_COMMIT}
::set-output name=commit_url::https://buf.build/example/repo/tree/${TEST_BSR_COMMIT}"
WANT_EXIT_CODE=0
buf_expect \
  "push --tag fake-sha test/proto" \
  "0" \
  "" \
  "The latest commit has the same content, not creating a new commit."
buf_expect \
  "beta registry commit get buf.build/example/repo --format=json" \
  "0" \
  "$(printf '{"commit": "%s"}' "${TEST_BSR_COMMIT}")" \
  ""
test_push test/proto main
echo "ok"

echo "testing non-main track"
GITHUB_SHA=fake-sha
BUF_TOKEN=fake-token
WANT_STDOUT="::add-mask::fake-token
::notice::pushed commit ${TEST_BSR_COMMIT}
::set-output name=commit::${TEST_BSR_COMMIT}
::set-output name=commit_url::https://buf.build/example/repo/tree/${TEST_BSR_COMMIT}"
WANT_STDERR=""
WANT_EXIT_CODE=0
buf_expect \
  "push --track example --help" \
  "0" \
  "${BUF_USAGE_MESSAGE}" \
  ""
buf_expect \
  "push --tag fake-sha test/proto --track non-main" \
  "0" \
  "${TEST_BSR_COMMIT}" \
  ""
test_push test/proto non-main
echo "ok"

echo "testing non-main track with old buf version"
GITHUB_SHA=fake-sha
BUF_TOKEN=fake-token
WANT_STDOUT="::add-mask::fake-token
::error::The installed version of buf does not support setting the track. Please use buf v1.0.0-rc11 or newer."
WANT_STDERR=""
WANT_EXIT_CODE=1
buf_expect \
  "push --track example --help" \
  "1" \
  "" \
  "${BUF_USAGE_MESSAGE}unknown flag: --track"
test_push test/proto non-main
echo "ok"

echo "testing no input"
GITHUB_SHA=fake-sha
BUF_TOKEN=fake-token
WANT_STDOUT=""
WANT_STDERR="Usage: ./push.bash <input> <track>"
WANT_EXIT_CODE=1
test_push
echo "ok"

echo "testing no GITHUB_SHA"
BUF_TOKEN=fake-token
WANT_STDOUT='::add-mask::fake-token
::error::the commit was not provided'
WANT_STDERR=""
WANT_EXIT_CODE=1
test_push test/proto main
echo "ok"

echo "testing no BUF_TOKEN"
GITHUB_SHA=fake-sha
WANT_STDOUT='::add-mask::
::error::a buf authentication token was not provided'
WANT_STDERR=""
WANT_EXIT_CODE=1
test_push test/proto main
echo "ok"

echo "testing no config file"
GITHUB_SHA=fake-sha
BUF_TOKEN=fake-token
WANT_EXIT_CODE=1
WANT_STDOUT='::add-mask::fake-token
::error::Config file not found: fake/path/buf.yaml'
WANT_STDERR=""
test_push fake/path main
echo "ok"

echo "testing bad config file"
mkdir -p tmp/test-bad-config-file
echo "bad config file" > tmp/test-bad-config-file/buf.yaml
GITHUB_SHA=fake-sha
BUF_TOKEN=fake-token
WANT_EXIT_CODE=1
WANT_STDOUT='::add-mask::fake-token
::error::name not found in tmp/test-bad-config-file/buf.yaml'
WANT_STDERR=""
test_push tmp/test-bad-config-file main
rm -rf tmp/test-bad-config-file
echo "ok"

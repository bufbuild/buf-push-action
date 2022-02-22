#!/usr/bin/env bash

set -euo pipefail

DIR="$(cd "$(dirname "${0}")/.." && pwd)"
cd "${DIR}"

export EXPECTATIONS_FILE="${GH_EXPECTATIONS_FILE}"
export FAILURE_LOG_FILE="${GH_FAILURE_LOG_FILE}"
export COMMAND="$0"
"${REPO_ROOT}"/test/faker.bash "$@"

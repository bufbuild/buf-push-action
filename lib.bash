#!/usr/bin/false

fail() {
  echo "::error::$1"
  exit 1
}

# checks for support of --track by running "buf push --track example --help"
# and checking for "unknown flag: --track" in the output.
buf_supports_track() {
  local buf_command="$1"
  set +e
  output="$("${buf_command}" push --track example --help 2>&1)"
  set -e
  if [[ "${output}" = *"unknown flag: --track"* ]]; then
    return 1
  fi
}

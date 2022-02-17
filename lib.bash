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

run_buf_push() {
  local buf_command="$1"
  local tag="$2"
  local track="$3"
  local input="$4"
  local args=("push" "--tag" "${tag}")
  # Only add --track if track isn't "main".
  # This is because --track is not supported by older versions of buf, and
  # we don't want to break any workflows that use older versions and leave
  # track as the default.
  if [ "${track}" != "main" ]; then
    args+=("--track" "${track}")
  fi
  args+=("${input}")
  local tmpdir stderr stdout
  tmpdir="$(mktemp -d)"
  touch "${tmpdir}/stdout.txt" "${tmpdir}/stderr.txt"
  set +e
  output="$("${buf_command}" "${args[@]}" >"${tmpdir}/stdout.txt" 2>"${tmpdir}/stderr.txt")"
  local exit_code="$?"
  set -e
  stdout="$(cat "${tmpdir}/stdout.txt")"
  stderr="$(cat "${tmpdir}/stderr.txt")"
  rm -rf "${tmpdir}"
  if [[ "${exit_code}" != "0" ]]; then
    fail "${stderr}"
  fi
  echo "${stdout}"
}

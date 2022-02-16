#!/usr/bin/false

# versions >= 1.0.0-rc11 support the --track flag
buf_version_supports_track() {
  version="$1"
  base="$(echo "$version" | cut  -d '-' -f 1)"
  major="$(echo "$base" | cut -d '.' -f 1)"
  if [ "$major" -lt 1 ]; then
    return 1
  fi
  if [ "$major" -gt 1 ]; then
    return 0
  fi
  minor="$(echo "$base" | cut -d '.' -f 2)"
  if [ "$minor" != "0" ]; then
    return 0
  fi
  patch="$(echo "$base" | cut -d '.' -f 3)"
  if [ "$patch" != "0" ]; then
    return 0
  fi
  prerelease="$(echo "$version" | cut  -s -d '-' -f 2)"
  if [ -z "${prerelease}" ]; then
    return 0
  fi
  if [[ "$prerelease" != rc* ]]; then
    return 1
  fi
  rc="${prerelease#"rc"}"
  if [ "$rc" -lt 11 ]; then
    return 1
  fi
}

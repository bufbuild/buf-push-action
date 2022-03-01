#!/usr/bin/env bash
# directoryhash.bash generates a sha256 hash of the contents of a directory
# This is needed to create a cache key for buf-push-action. We need to try
# multiple commands because there is no one sha command that all runners have.
set -eo pipefail

if [ $# -ne 1 ]; then
  echo "Usage: $0 <directory>" >&2
  exit 1
fi

TARGET_DIR="$1"

if command -v "gsha256sum" >/dev/null; then
  find "$TARGET_DIR" -type f -exec gsha256sum {} + |
    cut -d ' ' -f 1 | sort | gsha256sum | cut -d ' ' -f 1
  exit 0
fi
if command -v "sha256sum" >/dev/null; then
  find "$TARGET_DIR" -type f -exec sha256sum {} + |
    cut -d ' ' -f 1 | sort | sha256sum | cut -d ' ' -f 1
  exit 0
fi
if command -v "shasum" >/dev/null; then
  find "$TARGET_DIR" -type f -exec shasum -a 256 {} + |
    cut -d ' ' -f 1  | sort | shasum -a 256 | cut -d ' ' -f 1
  exit 0
fi
if command -v "openssl" >/dev/null; then
  find "$TARGET_DIR" -type f -exec openssl dgst -sha256 {} + |
    cut -d ' ' -f 2 | sort | openssl  dgst -sha256 | cut -d ' ' -f 2
  exit 0
fi

echo "No hashing tool found" >&2
exit 1

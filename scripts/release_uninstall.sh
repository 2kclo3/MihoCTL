#!/usr/bin/env bash

set -euo pipefail

TARGET_BIN="${1:-${HOME}/.local/bin/mihoctl}"

if [[ ! -x "${TARGET_BIN}" ]]; then
  echo "mihoctl binary not found: ${TARGET_BIN}" >&2
  exit 1
fi

"${TARGET_BIN}" self uninstall --yes

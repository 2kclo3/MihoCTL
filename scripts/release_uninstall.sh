#!/usr/bin/env bash

set -euo pipefail

TARGET_BIN="${1:-}"

choose_target_bin() {
  if [[ -n "${TARGET_BIN}" ]]; then
    printf '%s\n' "${TARGET_BIN}"
    return
  fi

  if command -v mihoctl >/dev/null 2>&1; then
    command -v mihoctl
    return
  fi

  for candidate in "${HOME}/.local/bin/mihoctl" "/usr/local/bin/mihoctl"; do
    if [[ -x "${candidate}" ]]; then
      printf '%s\n' "${candidate}"
      return
    fi
  done

  printf '%s\n' "${HOME}/.local/bin/mihoctl"
}

TARGET_BIN="$(choose_target_bin)"

if [[ ! -x "${TARGET_BIN}" ]]; then
  echo "mihoctl binary not found: ${TARGET_BIN}" >&2
  echo "Please pass the installed binary path explicitly, for example: ./uninstall.sh /usr/local/bin/mihoctl" >&2
  exit 1
fi

"${TARGET_BIN}" self uninstall --yes

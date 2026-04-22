#!/usr/bin/env bash

set -euo pipefail

OS_NAME="$(uname -s)"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MIHOCTL_BIN="${SCRIPT_DIR}/mihoctl"

if [[ ! -x "${MIHOCTL_BIN}" ]]; then
  echo "mihoctl binary not found: ${MIHOCTL_BIN}" >&2
  exit 1
fi

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run this helper with sudo." >&2
  exit 1
fi

resolve_target_user() {
  if [[ -n "${SUDO_USER:-}" && "${SUDO_USER}" != "root" ]]; then
    printf '%s\n' "${SUDO_USER}"
    return
  fi
  id -un
}

resolve_user_home() {
  local user_name="$1"
  local user_home=""

  case "${OS_NAME}" in
    Linux)
      if command -v getent >/dev/null 2>&1; then
        user_home="$(getent passwd "${user_name}" | cut -d: -f6 || true)"
      fi
      ;;
    Darwin)
      user_home="$(dscl . -read "/Users/${user_name}" NFSHomeDirectory 2>/dev/null | awk '{print $2}' || true)"
      ;;
  esac

  if [[ -n "${user_home}" ]]; then
    printf '%s\n' "${user_home}"
    return
  fi

  echo "Cannot resolve home directory for user: ${user_name}" >&2
  exit 1
}

resolve_config_path() {
  local user_home="$1"

  case "${OS_NAME}" in
    Darwin)
      printf '%s\n' "${user_home}/Library/Application Support/mihoctl/config.json"
      ;;
    *)
      printf '%s\n' "${user_home}/.config/mihoctl/config.json"
      ;;
  esac
}

resolve_mihomo_bin() {
  local user_home="$1"

  case "${OS_NAME}" in
    Darwin)
      printf '%s\n' "${user_home}/Library/Application Support/mihoctl/bin/mihomo"
      ;;
    *)
      printf '%s\n' "${user_home}/.config/mihoctl/bin/mihomo"
      ;;
  esac
}

run_as_target_user() {
  local user_name="$1"
  shift

  if command -v sudo >/dev/null 2>&1; then
    sudo -u "${user_name}" env HOME="${TARGET_HOME}" "$@"
    return
  fi

  su - "${user_name}" -c "$(printf '%q ' "$@")"
}

restart_user_managed_runtime() {
  echo "==> Restarting user-managed Mihomo process"
  run_as_target_user "${TARGET_USER}" "${MIHOCTL_BIN}" --config "${TARGET_CONFIG}" start >/dev/null || true
}

has_systemd_unit() {
  [[ -f /etc/systemd/system/mihomo.service ]]
}

is_systemd_active() {
  command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet mihomo
}

stop_linux_runtime() {
  local mihomo_bin="$1"
  local was_running=0
  local was_service_active=0

  if pgrep -f "${mihomo_bin}" >/dev/null 2>&1; then
    was_running=1
    echo "==> Stopping running Mihomo process so removed capabilities fully take effect"
    pkill -f "${mihomo_bin}" || true
  fi
  if has_systemd_unit && is_systemd_active; then
    was_service_active=1
  fi

  if [[ "${was_service_active}" -eq 1 ]]; then
    echo "==> Restarting Mihomo systemd service after removing capabilities"
    systemctl restart mihomo
    return
  fi

  if [[ "${was_running}" -eq 1 ]]; then
    restart_user_managed_runtime
  fi
}

stop_darwin_runtime() {
  local mihomo_bin="$1"
  local was_running=0

  if [[ -n "${mihomo_bin}" ]] && pgrep -f "${mihomo_bin}" >/dev/null 2>&1; then
    was_running=1
    echo "==> Stopping running Mihomo process after disabling LaunchDaemon"
    pkill -f "${mihomo_bin}" || true
  fi

  if [[ "${was_running}" -eq 1 ]]; then
    restart_user_managed_runtime
  fi
}

TARGET_USER="$(resolve_target_user)"
TARGET_HOME="$(resolve_user_home "${TARGET_USER}")"
TARGET_CONFIG="$(resolve_config_path "${TARGET_HOME}")"
TARGET_MIHOMO_BIN="$(resolve_mihomo_bin "${TARGET_HOME}")"

case "${OS_NAME}" in
  Linux)
    MIHOMO_BIN="${1:-${TARGET_HOME}/.config/mihoctl/bin/mihomo}"
    if [[ ! -e "${MIHOMO_BIN}" ]]; then
      echo "Mihomo binary not found: ${MIHOMO_BIN}" >&2
      exit 1
    fi

    echo "==> Removing TUN capabilities from ${MIHOMO_BIN}"
    setcap -r "${MIHOMO_BIN}" 2>/dev/null || true
    stop_linux_runtime "${MIHOMO_BIN}"

echo
echo "TUN support has been removed."
echo "If Mihomo was already running, it has been restarted automatically."
echo "If you were using TUN mode, switch back with:"
echo "  mihoctl mode env"
echo "  mihoctl off"
    ;;
  Darwin)
    if [[ ! -f "${TARGET_CONFIG}" ]]; then
      echo "MihoCTL config not found: ${TARGET_CONFIG}" >&2
      echo "Nothing to disable." >&2
      exit 1
    fi

    echo "==> Disabling Mihomo LaunchDaemon used for TUN"
    "${MIHOCTL_BIN}" --config "${TARGET_CONFIG}" service disable
    stop_darwin_runtime "${TARGET_MIHOMO_BIN}"

echo
echo "TUN support has been removed."
echo "If Mihomo was already running, it has been switched back automatically."
echo "If you were using TUN mode, switch back with:"
echo "  mihoctl mode env"
echo "  mihoctl off"
    ;;
  *)
    echo "TUN helper is only supported on Linux and macOS." >&2
    exit 1
    ;;
esac

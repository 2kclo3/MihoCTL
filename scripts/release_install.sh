#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET_DIR="${1:-}"

choose_target_dir() {
  if [[ -n "${TARGET_DIR}" ]]; then
    printf '%s\n' "${TARGET_DIR}"
    return
  fi

  if [[ -w /usr/local/bin ]]; then
    printf '%s\n' "/usr/local/bin"
    return
  fi

  printf '%s\n' "${HOME}/.local/bin"
}

TARGET_DIR="$(choose_target_dir)"
INSTALL_BIN="${TARGET_DIR}/mihoctl"
INSTALL_TUN_HELPER="${TARGET_DIR}/mihoctl-enable-tun"
UNINSTALL_TUN_HELPER="${TARGET_DIR}/mihoctl-disable-tun"
INSTALL_BUNDLED_DIR="${TARGET_DIR}/bundled"
BASH_COMPLETION_DIR="${XDG_DATA_HOME:-${HOME}/.local/share}/bash-completion/completions"
FISH_COMPLETION_DIR="${HOME}/.config/fish/completions"
ZSH_COMPLETION_DIR="${ZDOTDIR:-${HOME}}/.zsh/completions"
SHELL_INTEGRATION_START="# >>> mihoctl shell integration >>>"
SHELL_INTEGRATION_END="# <<< mihoctl shell integration <<<"

install_completion() {
  local shell_name="$1"
  local target_path="$2"

  mkdir -p "$(dirname "${target_path}")"
  if "${INSTALL_BIN}" completion "${shell_name}" > "${target_path}"; then
    echo "==> Installing ${shell_name} completion to ${target_path}"
  else
    echo "==> Skipped ${shell_name} completion installation"
  fi
}

shell_integration_block() {
  local shell_name="$1"
  case "${shell_name}" in
    bash)
      cat <<EOF
${SHELL_INTEGRATION_START}
if [[ -f "${BASH_COMPLETION_DIR}/mihoctl" ]]; then
  . "${BASH_COMPLETION_DIR}/mihoctl"
fi

mihoctl() {
  local _mihoctl_bin="${INSTALL_BIN}"
  if [[ ! -x "\${_mihoctl_bin}" ]]; then
    echo "mihoctl binary not found: \${_mihoctl_bin}" >&2
    return 127
  fi

  "\${_mihoctl_bin}" "\$@"
  local _mihoctl_status=\$?
  case "\${1-}" in
    on|off|mode|stop)
      eval "\$("\${_mihoctl_bin}" config env-shell 2>/dev/null || true)"
    ;;
    sub)
      if [[ "\${2-}" == "remove" ]]; then
        eval "\$("\${_mihoctl_bin}" config env-shell 2>/dev/null || true)"
      fi
    ;;
    self)
      if [[ "\${2-}" == "uninstall" && \${_mihoctl_status} -eq 0 ]]; then
        unset MIHOCTL_SYSTEM_PROXY_ENABLED
        unset http_proxy https_proxy all_proxy HTTP_PROXY HTTPS_PROXY ALL_PROXY
        unset no_proxy NO_PROXY
      fi
    ;;
  esac
  return \${_mihoctl_status}
}
${SHELL_INTEGRATION_END}
EOF
      ;;
    zsh)
      cat <<EOF
${SHELL_INTEGRATION_START}
fpath=("${ZSH_COMPLETION_DIR}" \$fpath)
autoload -Uz compinit
compinit -i

mihoctl() {
  local _mihoctl_bin="${INSTALL_BIN}"
  if [[ ! -x "\${_mihoctl_bin}" ]]; then
    echo "mihoctl binary not found: \${_mihoctl_bin}" >&2
    return 127
  fi

  "\${_mihoctl_bin}" "\$@"
  local _mihoctl_status=\$?
  case "\${1-}" in
    on|off|mode|stop)
      eval "\$("\${_mihoctl_bin}" config env-shell 2>/dev/null || true)"
    ;;
    sub)
      if [[ "\${2-}" == "remove" ]]; then
        eval "\$("\${_mihoctl_bin}" config env-shell 2>/dev/null || true)"
      fi
    ;;
    self)
      if [[ "\${2-}" == "uninstall" && \${_mihoctl_status} -eq 0 ]]; then
        unset MIHOCTL_SYSTEM_PROXY_ENABLED
        unset http_proxy https_proxy all_proxy HTTP_PROXY HTTPS_PROXY ALL_PROXY
        unset no_proxy NO_PROXY
      fi
    ;;
  esac
  return \${_mihoctl_status}
}
${SHELL_INTEGRATION_END}
EOF
      ;;
    *)
      return 1
      ;;
  esac
}

upsert_shell_integration() {
  local shell_name="$1"
  local target_file="$2"
  local block
  local temp_file
  block="$(shell_integration_block "${shell_name}")"
  temp_file="$(mktemp)"

  mkdir -p "$(dirname "${target_file}")"
  if [[ -f "${target_file}" ]]; then
    awk -v start="${SHELL_INTEGRATION_START}" -v end="${SHELL_INTEGRATION_END}" -v block="${block}" '
      BEGIN {
        skip = 0
        replaced = 0
      }
      $0 == start {
        if (!replaced) {
          print block
          replaced = 1
        }
        skip = 1
        next
      }
      $0 == end {
        skip = 0
        next
      }
      !skip {
        print
      }
      END {
        if (!replaced) {
          if (NR > 0) {
            print ""
          }
          print block
        }
      }
    ' "${target_file}" > "${temp_file}"
  else
    printf '%s\n' "${block}" > "${temp_file}"
  fi

  mv "${temp_file}" "${target_file}"
}

echo "==> Installing mihoctl to ${INSTALL_BIN}"
mkdir -p "${TARGET_DIR}"
cp "${SCRIPT_DIR}/mihoctl" "${INSTALL_BIN}"
chmod 755 "${INSTALL_BIN}"

echo "==> Installing TUN helper to ${INSTALL_TUN_HELPER}"
cp "${SCRIPT_DIR}/mihoctl-enable-tun" "${INSTALL_TUN_HELPER}"
chmod 755 "${INSTALL_TUN_HELPER}"

echo "==> Installing TUN cleanup helper to ${UNINSTALL_TUN_HELPER}"
cp "${SCRIPT_DIR}/mihoctl-disable-tun" "${UNINSTALL_TUN_HELPER}"
chmod 755 "${UNINSTALL_TUN_HELPER}"

echo "==> Installing bundled assets to ${INSTALL_BUNDLED_DIR}"
rm -rf "${INSTALL_BUNDLED_DIR}"
cp -R "${SCRIPT_DIR}/bundled" "${INSTALL_BUNDLED_DIR}"

echo "==> Installing bundled Mihomo core"
"${INSTALL_BIN}" core install

install_completion "bash" "${BASH_COMPLETION_DIR}/mihoctl"
install_completion "fish" "${FISH_COMPLETION_DIR}/mihoctl.fish"
install_completion "zsh" "${ZSH_COMPLETION_DIR}/_mihoctl"
upsert_shell_integration "bash" "${HOME}/.bashrc"
upsert_shell_integration "zsh" "${ZDOTDIR:-${HOME}}/.zshrc"

echo
echo "MihoCTL is ready."
echo "Command: ${INSTALL_BIN}"
echo

if [[ ":${PATH}:" != *":${TARGET_DIR}:"* ]]; then
  echo "If this shell still cannot find \`mihoctl\`, add it to PATH:"
  echo "  export PATH=\"${TARGET_DIR}:\$PATH\""
  echo
fi

echo "Quick start:"
echo "  ${INSTALL_BIN} sub add <subscription-url>"
echo "  ${INSTALL_BIN} on"
echo
echo "If you want TUN mode, run once:"
echo "  sudo ${INSTALL_TUN_HELPER}"
echo "Then enable it with:"
echo "  ${INSTALL_BIN} mode tun"
echo "  ${INSTALL_BIN} on"
echo
echo "If you later want to remove TUN authorization or service support:"
echo "  sudo ${UNINSTALL_TUN_HELPER}"
echo
echo "Shell completion was installed for bash/fish/zsh."
echo "Bash/Zsh startup files were updated automatically."
echo "Open a new terminal, or run \`source ~/.zshrc\` / \`source ~/.bashrc\` to use completion right away."
echo "After shell integration is active, running \`mihoctl on\`, \`off\`, or \`mode\` will also sync env-mode changes back to the current terminal."

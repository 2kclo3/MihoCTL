#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GOOS_VALUE="${GOOS:-linux}"
GOARCH_VALUE="${GOARCH:-amd64}"
OUTPUT_NAME="mihoctl-${GOOS_VALUE}-${GOARCH_VALUE}"
DIST_DIR="${ROOT_DIR}/dist/${OUTPUT_NAME}"
ARCHIVE_PATH="${ROOT_DIR}/dist/${OUTPUT_NAME}.tar.gz"

mkdir -p "${ROOT_DIR}/dist"
rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

echo "==> Building ${OUTPUT_NAME}"
(
  cd "${ROOT_DIR}"
  export CGO_ENABLED=0
  export GOOS="${GOOS_VALUE}"
  export GOARCH="${GOARCH_VALUE}"
  export GOCACHE="${GOCACHE:-/tmp/mihoctl-go-build-cache}"
  export GOMODCACHE="${GOMODCACHE:-/tmp/mihoctl-go-mod-cache}"
  go build -trimpath -ldflags="-s -w" -o "${DIST_DIR}/mihoctl" .
)

echo "==> Copying bundled assets"
cp -R "${ROOT_DIR}/assets/bundled" "${DIST_DIR}/bundled"

echo "==> Copying documentation"
cp "${ROOT_DIR}/README.md" "${DIST_DIR}/README.md"
cp "${ROOT_DIR}/assets/bundled/README.md" "${DIST_DIR}/BUNDLED.md"
cp "${ROOT_DIR}/scripts/release_install.sh" "${DIST_DIR}/install.sh"
cp "${ROOT_DIR}/scripts/release_uninstall.sh" "${DIST_DIR}/uninstall.sh"
chmod 755 "${DIST_DIR}/install.sh"
chmod 755 "${DIST_DIR}/uninstall.sh"

cat > "${DIST_DIR}/USE.md" <<'EOF'
# MihoCTL 测试使用说明

## 1. 一键安装

```bash
chmod +x ./install.sh
./install.sh
```

如果需要自定义安装目录：

```bash
./install.sh "$HOME/.local/bin"
```

## 2. 首次运行

```bash
chmod +x ./mihoctl
./mihoctl --help
```

## 3. 离线安装基础 Mihomo 内核

```bash
./mihoctl core install
```

如果发布包自带 `bundled/`，这一步会优先使用本地内核和数据库，不依赖 GitHub。

## 4. 查看当前配置

```bash
./mihoctl config view
```

## 5. 添加订阅并更新

```bash
./mihoctl sub add https://example.com/sub.yaml
./mihoctl sub update
```

## 6. 启动 Mihomo

```bash
./mihoctl start
./mihoctl status
```

## 7. 安装到 PATH

```bash
./mihoctl self install
```

重新打开终端后，通常就可以直接使用：

```bash
mihoctl config view
```

如果你当前终端里仍然提示 `mihoctl: command not found`，请先执行：

```bash
export PATH="$HOME/.local/bin:$PATH"
```

如果你用 zsh，想让 `mihoctl <Tab>` 补全子命令，请确认 `~/.zshrc` 包含：

```bash
fpath=("$HOME/.zsh/completions" $fpath)
autoload -Uz compinit && compinit
```

重新打开终端，或执行 `source ~/.zshrc`。
EOF

echo "==> Creating archive ${ARCHIVE_PATH}"
tar -C "${ROOT_DIR}/dist" -czf "${ARCHIVE_PATH}" "${OUTPUT_NAME}"

echo "==> Done"
echo "${ARCHIVE_PATH}"

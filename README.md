# MihoCTL

MihoCTL 是一个面向 Linux 与 macOS 无头环境的 Mihomo 命令行管理工具，目标是用单文件 CLI 完成内核安装、生命周期控制、订阅更新、策略切换和模式管理。

## 当前已实现

- 支持离线首装：优先从随包附带的 `bundled/` 或 `assets/bundled/` 安装基础版 Mihomo 内核与数据库。
- `mihoctl core install`：从 `MetaCubeX/mihomo` 的 GitHub Release 下载最新 Mihomo 内核并安装到 MihoCTL 自己的 `bin/` 目录。
- `mihoctl core upgrade`：升级 MihoCTL 管理的 Mihomo 内核。
- `mihoctl core install --version vX.Y.Z`：安装指定版本的 Mihomo 内核。
- `mihoctl core upgrade --version vX.Y.Z`：升级或切换到指定版本的 Mihomo 内核。
- `mihoctl self install`：把 MihoCTL 安装到 PATH 目录中，之后可以直接运行 `mihoctl ...`
- `mihoctl completion bash|zsh|fish`：生成 shell 自动补全脚本。
- `mihoctl start|stop|restart|status`
- `mihoctl sub add|list|use|remove|update`，其中 `sub add` 支持直接粘贴常见的一键导入链接，程序会自动解析出真实订阅地址；订阅下载会优先尝试基于兼容高版本号模板生成的 Clash Verge / FlClash / CFW / v2rayN 请求头形态，并支持 URL 中 `user:pass@host` 的 Basic Auth
- `mihoctl proxy list|use|check`
- `mihoctl mode [env|tun]`、`mihoctl on`、`mihoctl off`
  Linux 无头环境下，`env` 模式会通过受控的 shell 环境变量脚本为新终端注入代理设置；默认模式为 `env`
- `mihoctl service enable|disable|status`
- `mihoctl config set-lang|view`

## 发布包使用

如果你拿到的是已经打好的发布包，不需要先装 Go。

1. 解压：

```bash
tar -xzf mihoctl-linux-amd64.tar.gz
cd mihoctl-linux-amd64
```

2. 首次离线安装基础内核：

```bash
chmod +x ./mihoctl
./mihoctl core install
```

更省事的方式是直接运行安装脚本：

```bash
chmod +x ./install.sh
./install.sh
```

它会自动完成：

- 安装 `mihoctl` 到合适目录
- 复制 `bundled/` 到同级目录
- 执行 `mihoctl core install`
- 生成 bash、fish、zsh 的补全脚本

3. 查看配置：

```bash
./mihoctl config view
```

4. 如需装到 PATH：

```bash
./mihoctl self install
```

装好后通常可以直接执行：

```bash
mihoctl config view
```

如果当前 shell 里还是提示 `mihoctl: command not found`，这是因为安装脚本无法修改父 shell 的环境变量。当前终端里执行一次：

```bash
export PATH="$HOME/.local/bin:$PATH"
```

如果你用的是 zsh，想让 `mihoctl <Tab>` 补全命令而不是补全当前目录文件名，还需要确保 `~/.zshrc` 里有：

```bash
fpath=("$HOME/.zsh/completions" $fpath)
autoload -Uz compinit && compinit
```

重新打开终端，或执行 `source ~/.zshrc` 后即可生效。

## 源码方式

1. 编译：

```bash
go build -o mihoctl .
```

2. 首次安装 Mihomo 内核：

```bash
./mihoctl core install
```

如果你想固定到某个版本：

```bash
./mihoctl core install --version v1.19.10
```

如果发行包中已经附带了 `bundled/`，`core install` 会优先使用本地基础内核和数据库，不依赖外网。

3. 把命令安装到 PATH：

```bash
./mihoctl self install
```

安装成功后，新的终端里通常就可以直接使用：

```bash
mihoctl config view
```

4. 查看当前配置：

```bash
./mihoctl config view
```

5. 添加订阅并更新配置：

```bash
./mihoctl sub add https://example.com/sub.yaml
./mihoctl sub update
```

`sub add` 会立即尝试下载并保存这份订阅配置，但不会再偷偷指定默认订阅。你可以显式执行：

```bash
./mihoctl sub use <name|index>
```

如果当前没有默认订阅，相关命令会直接提示你先选择，而不是自动兜底。

订阅拉取时会固定使用 `clash-verge/v999.999.999` 作为请求头 User-Agent，对用户透明处理，无需手动配置。

6. 如果你需要 `tun` 模式，先执行一次：

```bash
sudo mihoctl-enable-tun
```

完成后再切换：

```bash
./mihoctl mode tun
./mihoctl on
```

如果之后不再需要 TUN，可以执行：

```bash
sudo mihoctl-disable-tun
```

7. 如果你只是想开始使用，直接执行：

```bash
./mihoctl on
```

如果你明确想用隐藏的后台启动命令，也可以执行：

```bash
./mihoctl start
```

## 说明

- 默认会把 Mihomo 二进制安装到 MihoCTL 配置目录下的 `bin/mihomo`。
- 默认会把 bundled 数据库文件复制到 `core.database_dir`，默认即 Mihomo 的工作目录。
- 如果命令运行在交互终端中，且检测到 Mihomo 有新版本，MihoCTL 会在合适时机提示是否升级。
- 语言优先级为：命令参数 `--lang` > 环境变量 `MIHOCTL_LANG` > 配置文件 > 默认值。
- `mihomo` 是内核本体，`mihoctl` 才是管理命令；如果运行成 `mihomo sub list`，看到的是 Mihomo 自己的行为，不是 MihoCTL。
- 本仓库当前只做了构建校验，没有在这台机器上执行真实 Mihomo 联调、系统代理切换或服务注册测试。

## 打包发布

直接生成 Linux amd64 发布包：

```bash
make package-linux-amd64
```

生成后会得到：

```text
dist/
  mihoctl-linux-amd64/
  mihoctl-linux-amd64.tar.gz
```

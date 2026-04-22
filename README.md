# MihoCTL

MihoCTL 是一个面向 Linux 和 macOS 的 Mihomo 命令行管理工具。

它更适合这类场景：

- 你在用服务器、远程主机、无头环境，想用命令行管理代理
- 你不想手动维护 Mihomo 内核、配置文件、开关模式
- 你希望用尽量少的命令完成“安装、导入订阅、开启代理、切节点、更新”

MihoCTL 会帮你处理这些事：

- 安装或升级 Mihomo 内核
- 添加订阅并下载配置
- 开启或关闭代理
- 切换 `env` / `tun` 模式
- 切换策略组和节点
- 开机自动启动
- 基础诊断和状态检查

## 快速上手

如果你拿到的是发布包，推荐按这个流程走。

### 1. 安装

```bash
tar -xzf mihoctl-linux-amd64.tar.gz
cd mihoctl-linux-amd64
chmod +x ./install.sh
./install.sh
```

安装脚本会自动完成这些事：

- 安装 `mihoctl`
- 安装随包附带的基础版 Mihomo 内核和数据库
- 安装 shell 自动补全
- 为 bash / zsh 写入 shell 集成，方便直接用 `mihoctl`

如果安装后当前终端还提示 `mihoctl: command not found`，通常是因为当前 shell 还没刷新 PATH。

先执行一次：

```bash
export PATH="$HOME/.local/bin:$PATH"
```

如果你用的是 bash 或 zsh，重新开一个终端通常就可以直接用了。

### 2. 添加订阅

```bash
mihoctl sub add https://example.com/your-subscription
```

`sub add` 会立即做三件事：

- 保存订阅
- 立刻下载配置文件
- 如果这是第一条订阅，自动把它设为当前默认订阅

你可以直接粘贴很多机场提供的“一键导入链接”，MihoCTL 会尽量自动解析出真实订阅地址。

### 3. 开启代理

```bash
mihoctl on
```

默认模式是 `env`，也就是环境变量模式。

这意味着：

- 当前和新开的终端可以继承代理环境
- 更适合服务器、SSH、无桌面环境
- 不需要一上来就折腾 TUN 权限

查看当前状态：

```bash
mihoctl status
```

关闭代理：

```bash
mihoctl off
```

## 最常用的命令

### 订阅管理

查看订阅：

```bash
mihoctl sub list
```

切换默认订阅：

```bash
mihoctl sub use 2
```

更新全部订阅：

```bash
mihoctl sub update
```

删除订阅：

```bash
mihoctl sub remove 2
```

### 节点和策略组

查看策略组和节点：

```bash
mihoctl proxy list
```

切换节点：

```bash
mihoctl proxy use 12
```

或者指定“策略组 + 节点”：

```bash
mihoctl proxy use 2 18
```

测速：

```bash
mihoctl proxy check
```

### 模式切换

查看当前模式：

```bash
mihoctl mode
```

切到环境变量模式：

```bash
mihoctl mode env
```

切到 TUN 模式：

```bash
mihoctl mode tun
```

说明：

- `mode` 只负责切换“以后用哪种方式代理”
- `on` / `off` 负责真正开启或关闭当前模式
- 如果切换失败，MihoCTL 不会把状态修改成功

### 一键更新

更新订阅和内核：

```bash
mihoctl update
```

只更新订阅：

```bash
mihoctl update sub
```

只更新内核：

```bash
mihoctl update core
```

### 诊断

```bash
mihoctl doctor
```

适合用在这些时候：

- 不确定内核有没有装好
- 不确定配置文件是否存在
- 不确定 Controller API 是否可达
- 不确定当前到底是 `env` 还是 `tun`

## `env` 和 `tun` 怎么选

### `env`

推荐先从 `env` 开始。

优点：

- 对服务器最友好
- 配置简单
- 权限要求低
- 出问题时容易恢复

适合：

- SSH 远程主机
- 无头 Linux
- 主要在终端里使用代理

### `tun`

`tun` 更像“全局接管流量”，但它需要额外权限。

适合：

- 你明确知道自己需要 TUN
- 机器具备对应权限
- 你愿意按提示执行一次管理员授权

如果要启用 TUN，先执行一次：

```bash
sudo mihoctl-enable-tun
```

然后再执行：

```bash
mihoctl mode tun
mihoctl on
```

如果以后不想保留 TUN 授权或相关支持，可以执行：

```bash
sudo mihoctl-disable-tun
```

## 开机自动启动

先确认订阅已经可用、`mihoctl on` 能正常工作，再开启开机自启。

开启：

```bash
sudo mihoctl boot on
```

查看状态：

```bash
mihoctl boot status
```

关闭：

```bash
sudo mihoctl boot off
```

说明：

- 开机自启依赖系统级服务，因此通常需要管理员权限
- `boot on` 和 `boot shell on` 不建议同时开启；如果你启用了系统级开机自启，MihoCTL 会自动关闭登录后自恢复启动
- 如果你已经开启 `boot on`，删除订阅前需要先执行 `mihoctl boot off`

## AutoDL 推荐方案

如果你用的是 AutoDL 这类 Docker 容器环境，没有 `systemd`，也不适合走传统的系统级开机自启。

这种场景下，作者更推荐用 MihoCTL 自带的“登录后自恢复启动”。

执行一条命令，把自动恢复逻辑写进你的 shell 配置文件：

```bash
mihoctl boot shell on
```

查看状态：

```bash
mihoctl boot shell status
```

关闭这套逻辑：

```bash
mihoctl boot shell off
```

说明：

- 这套方案会写入 `~/.profile`、`~/.bashrc`、`~/.zshrc`
- 如果已经开启系统级 `boot on`，需要先执行 `mihoctl boot off`，再开启 `boot shell on`
- 以后你在 AutoDL 容器里新开终端时，会自动检查 Mihomo 是否已运行
- 如果 Mihomo 没跑，就会自动执行 `mihoctl start`
- 这更适合容器环境里的“登录后自动恢复”，不是传统意义上依赖 `systemd` 的系统级开机自启

## 常见问题

### 1. 为什么 `mihoctl on` 后当前终端没有立刻生效

如果你是用发布包安装，并且使用 bash / zsh，安装脚本通常已经帮你接好了 shell 集成。

如果当前终端还是没同步，可以执行一次：

```bash
source ~/.zshrc
```

或者：

```bash
source ~/.bashrc
```

重新开一个终端也可以。

### 2. 为什么 `mihoctl proxy list` 提示 Controller API 连不上

这通常表示 Mihomo 还没启动成功，或者配置文件还没准备好。

建议按顺序检查：

1. 先确认已经添加过订阅：`mihoctl sub list`
2. 再看状态：`mihoctl status`
3. 最后做诊断：`mihoctl doctor`

### 3. 为什么 TUN 开不了

通常是这几类原因：

- 没有管理员权限
- Mihomo 二进制还没获得 TUN 所需能力
- 当前系统或内核不支持

优先按提示执行一次：

```bash
sudo mihoctl-enable-tun
```

如果仍然失败，再看日志：

- Linux: `~/.config/mihoctl/logs/mihomo.log`
- macOS: `~/Library/Application Support/mihoctl/logs/mihomo.log`

## 卸载

优先使用：

```bash
mihoctl self uninstall --yes
```

如果当前环境里已经找不到 `mihoctl`，再使用发布包里的：

```bash
./uninstall.sh
```

## 从源码编译

如果你是自己开发或想本地编译：

```bash
go build -o mihoctl .
```

编译后可以直接运行：

```bash
./mihoctl sub add https://example.com/your-subscription
./mihoctl on
```

如果想安装到 PATH：

```bash
./mihoctl self install
```

## 额外说明

- MihoCTL 默认把配置放在用户目录下，而不是写进项目目录
- 默认代理模式是 `env`
- 订阅下载会自动使用兼容性较强的请求头，对用户透明
- 如果发布包内带有 `bundled/`，首次安装即使暂时访问不了 GitHub，也可以先用内置基础内核启动

## 打包发布

生成 Linux amd64 发布包：

```bash
make package-linux-amd64
```

生成后会得到：

```text
dist/
  mihoctl-linux-amd64/
  mihoctl-linux-amd64.tar.gz
```

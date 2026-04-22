# MihoCTL 实现任务拆解

## 阶段 1：基础骨架

- [x] 初始化 Go 模块与 Cobra CLI 入口。
- [x] 搭建根命令与一级命令树：`start`、`stop`、`restart`、`status`、`service`、`sub`、`proxy`、`mode`、`config`。
- [x] 建立统一的配置加载流程，支持命令参数、环境变量、配置文件和默认值。
- [x] 建立统一的中英文文案层，避免业务逻辑中硬编码用户可见字符串。

## 阶段 2：本地状态与错误模型

- [x] 设计 MihoCTL 自身配置文件结构。
- [x] 设计运行状态文件，记录 PID、启动时间、模式切换来源和最近错误。
- [x] 实现结构化错误模型，输出错误码、原因和建议下一步。

## 阶段 3：Mihomo 生命周期控制

- [x] 实现 `core install`，从 GitHub Release 下载并安装最新 Mihomo 内核。
- [x] 实现 `core upgrade`，升级 MihoCTL 管理的 Mihomo 内核。
- [x] 实现 bundled 资产安装，支持无外网时从随包附带的基础内核和数据库首装。
- [x] 实现联网后的新版本检查与升级提示。
- [x] 实现 Mihomo 二进制路径、配置路径和日志路径解析。
- [x] 实现 `start`，支持后台启动、日志落盘、PID 记录。
- [x] 实现 `stop`，支持优雅停止与兜底清理。
- [x] 实现 `restart`。
- [x] 实现 `status`，输出进程状态、PID、运行时长和 Controller API 连通性。

## 阶段 4：订阅管理

- [x] 实现订阅源保存与去重。
- [x] 实现 `sub add <url>`。
- [x] 实现 `sub list`。
- [x] 实现 `sub update [url]`，下载配置文件并尝试通过 Controller API 热重载。

## 阶段 5：代理控制

- [x] 实现 Controller API 客户端。
- [x] 实现 `proxy ls`，列出策略组与当前节点。
- [x] 实现 `proxy use <group> <proxy>`。
- [x] 实现 `proxy check`，批量触发策略组延迟测试。

## 阶段 6：模式控制

- [x] 实现 `mode tun on/off/status`，优先通过 Controller API 切换和查询。
- [x] 实现 `mode sys on/off/status`。
- [x] macOS 上接入 `networksetup` 控制系统代理。
- [x] Linux 上在无法统一控制时给出明确降级提示与手工处理建议。

## 阶段 7：系统服务

- [x] Linux 生成并管理 `systemd` unit。
- [x] macOS 生成并管理 `launchd` LaunchDaemon plist。
- [x] 实现 `service enable/disable/status`。
- [x] 对权限不足、命令缺失、平台不支持提供清晰提示。

## 阶段 8：收尾

- [x] 补充必要的中文注释，重点解释配置优先级、后台进程管理和跨平台分支。
- [x] 完成第二轮收口，补充 README 与更友好的配置展示。
- [x] 增加 `self install`，支持安装到 PATH 后直接以 `mihoctl` 调用。
- [x] 格式化代码。
- [x] 说明由于当前机器环境限制，未在本机运行实际 Mihomo 控制测试。

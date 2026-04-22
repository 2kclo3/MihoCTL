# Bundled 资产目录约定

MihoCTL 会按以下优先级查找内置基础资产：

1. 可执行文件同级目录下的 `bundled/`
2. 可执行文件同级目录下的 `assets/bundled/`
3. 当前工作目录下的 `bundled/`
4. 当前工作目录下的 `assets/bundled/`

推荐目录结构如下：

```text
bundled/
  version.txt
  common/
    geoip.metadb
    geosite.dat
  linux-amd64/
    mihomo
    version.txt
    db/
      Country.mmdb
  darwin-arm64/
    mihomo
    version.txt
    db/
      Country.mmdb
```

说明：

- `common/` 下的普通文件会被复制到 `core.database_dir`。
- `<platform>/db/` 下的普通文件也会被复制到 `core.database_dir`。
- `<platform>/mihomo` 会被安装为 MihoCTL 管理的 Mihomo 内核。
- `<platform>/version.txt` 可选；如果缺失，则回退到根目录 `version.txt`。
- `<platform>` 命名规则使用 Go 平台名，例如 `linux-amd64`、`darwin-arm64`。

# 1Panel MCP Server

[English](../README.md) | [简体中文](README.zh-Hans.md)

1Panel MCP Server 为 MCP 客户端提供 1Panel v2 的节点、系统、网站、证书、应用和 MySQL 管理能力，
包含 21 个工具，支持使用 API Key 的独立部署，以及接入面板凭据授权与审计体系的托管部署。

## 选择接入模式

| 模式 | 连接与授权 | 部署方式 |
| --- | --- | --- |
| 独立：`--mode=legacy`（默认） | 使用配置的 API Key 调用面板公开 `/api/v2` API，受该 Key 及其所有者的面板权限约束 | 二进制或 Docker，支持 stdio 和 Streamable HTTP |
| 托管：`--mode=panel` | 使用面板签发的 MCP 凭据，由 Core 校验凭据所有者的当前权限、节点和工具授权 | 通过面板 MCP 服务页面安装，由主节点管理 Docker 容器 |

独立模式要求目标 1Panel v2 已启用公开 API，且所选工具依赖的接口及请求、响应格式与 MCP 版本兼容，
无需安装面板托管的 MCP 服务。托管模式要求 Core、主节点及子节点 Agent、前端版本配套。

| 凭据 | 用途 |
| --- | --- |
| `PANEL_ACCESS_TOKEN` / `--token` | 独立模式访问面板 API 的 1Panel API Key |
| `MCP_AUTH_TOKEN` / `--mcp-token` | 独立 Streamable HTTP 服务的接入令牌，客户端通过 Bearer 请求头发送 |
| 面板 MCP 页面创建的 Token | 显式授权节点和工具的专用托管凭据；客户端连接面板 HTTPS `/mcp` 端点 |

连接同一独立实例的客户端共用该实例配置的 API Key 身份；接入令牌不提供独立的面板用户身份或逐凭据的节点、工具授权。
面板 API Key、独立接入令牌和托管 MCP 凭据用途不同，不能互换。生产部署应使用权限满足业务需求的最小授权。

## 安装

### 从 Releases 下载

打开 [GitHub Releases](https://github.com/1Panel-dev/mcp-1panel/releases)，
选择已发布且与目标面板公开 API 兼容的 v2 固定版本，下载与运行主机架构匹配的 Linux 二进制
及同一版本的 `SHA256SUMS`。运行二进制无需安装 Go。

发布文件是独立可执行文件，无需解压。以 Linux amd64 为例，将
`mcp-1panel-linux-amd64` 和 `SHA256SUMS` 放在同一目录后执行：

```sh
sha256sum --ignore-missing -c SHA256SUMS
chmod +x ./mcp-1panel-linux-amd64
./mcp-1panel-linux-amd64 --version
```

Linux arm64 主机下载 `mcp-1panel-linux-arm64`，并将命令中的文件名相应替换。
客户端配置须使用可执行文件的实际绝对路径。当前 Release 提供 Linux 二进制；macOS 或 Windows 可使用 Docker 部署。

### Docker

通过 [Docker Compose](../docker-compose.yml) 使用官方
[1panel/mcp-server](https://hub.docker.com/r/1panel/mcp-server) 镜像。
镜像默认运行 stdio，以下 Compose 显式启动独立 Streamable HTTP 服务。
将以下内容保存为 `docker-compose.yml`：

```yaml
services:
  mcp-1panel:
    image: "${MCP_IMAGE:?Set MCP_IMAGE to a published 1panel/mcp-server tag with authenticated remote HTTP support}"
    restart: unless-stopped
    ports:
      - "${MCP_PORT:-16082}:8000"
    environment:
      PANEL_HOST: "${PANEL_HOST:?Set PANEL_HOST to the 1Panel API root URL}"
      PANEL_ACCESS_TOKEN: "${PANEL_ACCESS_TOKEN:?Set PANEL_ACCESS_TOKEN to the 1Panel API Key}"
      PANEL_KEY_ID: "${PANEL_KEY_ID:-}"
      MCP_AUTH_TOKEN: "${MCP_AUTH_TOKEN:?Set a separate strong MCP_AUTH_TOKEN}"
      MCP_ACCESS_LEVEL: "${MCP_ACCESS_LEVEL:-readonly}"
      MCP_LANGUAGE: "${MCP_LANGUAGE:-zh-CN}"
    command:
      - --mode=legacy
      - --transport=streamable-http
      - --addr=http://0.0.0.0:8000
      - --allow-remote-http
      - "--allowed-origins=${MCP_ALLOWED_ORIGINS:-}"
```

在 Compose 同目录创建 `.env`，替换以下占位值。镜像须使用已发布、支持已认证远程 HTTP
且与面板 API、运行主机架构兼容的固定版本：

```dotenv
MCP_IMAGE=1panel/mcp-server:YOUR_VERSION
PANEL_HOST=https://panel.example.com
PANEL_ACCESS_TOKEN=YOUR_PANEL_API_KEY
MCP_AUTH_TOKEN=YOUR_MCP_AUTH_TOKEN
```

`MCP_AUTH_TOKEN` 应为独立生成的强随机令牌。限制 `.env` 的读取权限，不将凭据纳入版本管理。
生产环境的 `PANEL_HOST` 建议使用 HTTPS，并保持证书校验启用。
确认配置后启动服务：

```sh
docker compose pull
docker compose up -d
```

- 生产外部接入建议使用网站 HTTPS：在 1Panel 配置反向代理及有效证书，客户端访问 `https://mcp.example.com/` ，MCP 容器继续提供 HTTP 上游。
- 所有 HTTP 接入请求均须携带 `Authorization: Bearer <MCP_AUTH_TOKEN>`；HTTP 明文传输令牌和业务数据，仅适用于受控网络。
- 默认映射 `16082:8000` 会在宿主机所有接口发布 HTTP 端口。网站启用 HTTPS 不会关闭该入口；采用反向代理时，应按网络拓扑限制其直连访问。

托管模式通过面板安装。

## 连接客户端

### stdio

配置支持 stdio 的 MCP 客户端，由客户端启动并管理服务进程。以下为通用配置，须替换路径和凭据：

```json
{
  "mcpServers": {
    "mcp-1panel": {
      "command": "/absolute/path/to/mcp-1panel-linux-amd64",
      "args": ["--mode=legacy", "--transport=stdio", "--access-level=readonly"],
      "env": {
        "PANEL_HOST": "https://panel.example.com",
        "PANEL_ACCESS_TOKEN": "YOUR_PANEL_API_KEY",
        "MCP_LANGUAGE": "zh-CN"
      }
    }
  }
}
```

stdio 通过进程标准输入、输出通信，不监听网络端口，也不需要 `MCP_AUTH_TOKEN`。
需要独立运行的常驻服务时，使用上述 Compose 部署，并通过 Streamable HTTP 连接。

## 访问级别与工具

独立模式默认使用 `readonly`。`--access-level` / `MCP_ACCESS_LEVEL` 同时限制工具发现和执行，
未授权工具即使被直接调用也会拒绝。实际可执行权限取访问级别与面板 API Key 权限的交集。
托管模式由 Core 执行凭据授权，不使用此选项。

| 级别 | 工具 |
| --- | --- |
| `readonly` | 15 个查询、列表和状态工具 |
| `readwrite` | 19 个工具，增加网站、证书和数据库创建，以及网站证书绑定 |
| `full` | 全部 21 个工具，再增加 MySQL 和 OpenResty 安装 |

| 工具 | 最低级别 | 用途 |
| --- | --- | --- |
| `list_nodes` | `readonly` | 发现有权查看的节点 ID |
| `get_system_info` | `readonly` | 系统信息 |
| `get_dashboard_info` | `readonly` | 概览状态 |
| `list_websites` | `readonly` | 网站列表 |
| `get_website` | `readonly` | 网站元数据 |
| `list_website_domains` | `readonly` | 域名和端口 |
| `get_website_https` | `readonly` | HTTPS 和证书元数据 |
| `list_ssls` | `readonly` | 证书列表 |
| `get_ssl_metadata` | `readonly` | 证书元数据 |
| `list_installed_apps` | `readonly` | 已安装应用列表 |
| `get_installed_app` | `readonly` | 安装状态、版本和端口 |
| `list_app_versions` | `readonly` | 可用应用版本 |
| `list_databases` | `readonly` | 指定本地 MySQL 实例的数据库 |
| `list_mysql_instances` | `readonly` | 已记录的本地 MySQL 实例 |
| `get_mysql_status` | `readonly` | 固定 MySQL 状态计数器 |
| `create_website` | `readwrite` | 创建静态或反向代理网站 |
| `create_ssl` | `readwrite` | 提交证书申请 |
| `bind_ssl` | `readwrite` | 将已就绪证书绑定到静态网站 |
| `create_database` | `readwrite` | 创建本地 MySQL 数据库 |
| `install_mysql` | `full` | 提交 MySQL 安装 |
| `install_openresty` | `full` | 提交 OpenResty 安装 |

独立模式的 `get_installed_app` 可能触发面板内部状态缓存刷新；`readonly` 限制业务工具权限，
不保证查询过程完全不更新内部缓存。托管模式读取已记录的应用状态。

## 节点选择与写入结果

除 `list_nodes` 外，20 个业务工具均接受可选参数 `nodeId`，默认值为 `"local"`。
以 `{}` 调用 `list_nodes` 获取子节点 ID，再传入业务工具，例如 `"nodeId":"7"`；不接受节点名称或 IP。
独立模式在每次调用中将 ID 解析为当前节点名称，通过面板 `CurrentNode` 路由；本地调用不依赖节点发现。
托管模式使用稳定节点 ID，并在每个 API 步骤重新校验授权。目标节点不可用或无权限时均返回错误，不回退至主节点。

`bind_ssl` 要求提供目标节点的 `website_id` 和 `ssl_id`，仅支持运行中的顶级静态网站及具体的 ASCII/punycode DNS 域名。
写入前校验证书就绪状态、有效期及域名覆盖范围；网站已关联其他证书时拒绝操作。
首次绑定会保存一组 HTTPS 设置：HTTP 跳转 HTTPS、TLS 1.3/1.2、面板默认密码套件，HSTS 和 HTTP/3 关闭，
不保证保留尚未绑定证书的网站原有自定义 HTTPS 设置。
`applied` 表示保存成功且回读的证书关联、跳转、协议和 HSTS/HTTP3 设置符合预期；
`already_bound` 表示面板已记录相同证书，本次未执行写入。
上述状态不代表已验证实际提供的 TLS 证书，也不保证与其他并发修改隔离。失败或结果未知时不会自动重试或补偿。
托管模式需单独授予该工具权限；策略更新后，须刷新工具目录并重新确认授权。

证书签发和应用安装返回 `requestStatus:accepted` 时，仅表示请求已受理，应继续查询资源或任务的最终状态。
写入提交后发生网络中断、响应读取失败、HTTP 或业务码 5xx，或成功响应格式无效时，执行结果可能无法确认。
此时须先在 1Panel 核对实际资源和任务，再决定是否重试，避免重复创建或安装。
断开客户端连接或撤销凭据不会回滚已受理的操作。

## 配置参考

命令行配置优先于对应环境变量。

| 选项 | 环境变量 / 默认值 |
| --- | --- |
| `--mode` | `legacy` 或 `panel`，默认 `legacy` |
| `--host`、`--token` | `PANEL_HOST`、`PANEL_ACCESS_TOKEN` |
| `--key-id` | 可选 `PANEL_KEY_ID`，用于支持多个 API Key 的面板 |
| `--transport` | `stdio`（默认）或 `streamable-http` |
| `--addr` | 默认 `http://127.0.0.1:8000`，可包含 `/mcp` |
| `--mcp-token` | `MCP_AUTH_TOKEN`，用于独立 HTTP 认证 |
| `--access-level` | `MCP_ACCESS_LEVEL`，默认 `readonly` |
| `--tls-dir`、`--tls-hosts` | 仅 MCP 自身提供 HTTPS 时使用，网站 HTTPS 反代无需配置 |
| `--allowed-origins` | 逗号分隔的准确 Origin 允许列表 |
| `--allow-remote-http` | 允许已认证 HTTP 或 HTTPS 监听非回环地址 |
| `--allow-insecure-http` | 仅供本地开发的未认证回环传输 |
| 错误提示语言 | `MCP_LANGUAGE=zh-CN` 或 `en`，HTTP `Accept-Language` 优先；不改变工具标识、字段和结果结构 |

`--version` 和 `--catalog` 可在离线环境输出版本及工具目录。
`--listen`、`--panel-socket` 由面板托管部署管理；`--healthcheck` 用于检查托管服务与 Core 的就绪状态。
版本检查或工具发现成功不代表面板 API 及业务操作已验证，部署后仍须完成实际调用验收。

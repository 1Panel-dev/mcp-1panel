# 1Panel MCP Server

[English](../README.md) | [简体中文](README.zh-Hans.md)

面向 1Panel v2 的 MCP 服务，提供节点、系统状态、网站、证书、应用和 MySQL 共 20 个共享工具。
独立模式使用官方 Go MCP SDK；托管模式接入 1Panel 的授权与审计体系。

## 选择接入模式

| 模式 | 连接与授权 | 部署方式 |
| --- | --- | --- |
| 独立：`--mode=legacy`（默认） | 使用配置的 API Key 调用既有公开 `/api/v2` API；每次请求均受面板权限约束 | 二进制或 Docker，支持 stdio 和 Streamable HTTP |
| 托管：`--mode=panel` | 使用专用 MCP 凭据；Core 检查所有者当前权限、节点/工具授权及策略 | 主节点 Docker，通过 **AI → MCP管理 → 1Panel MCP** 安装 |

独立模式要求已安装的 1Panel v2 启用公开 API，并提供所选工具需要的 API 和兼容 DTO，
无需安装托管 MCP。托管模式需要配套的 Core、主/子节点 Agent 和前端。
详见[托管部署](panel-integration.zh-Hans.md#docker-deployment)。

| 凭据 | 用途 |
| --- | --- |
| `PANEL_ACCESS_TOKEN` / `--token` | 独立模式访问面板 API 的 1Panel API Key |
| `MCP_AUTH_TOKEN` / `--mcp-token` | 客户端连接独立 Streamable HTTP 服务时使用的另一份预共享 Bearer |
| 面板 MCP 页面创建的 Token | 显式授权节点和工具的专用托管凭据；客户端连接面板 HTTPS `/mcp` 端点 |

独立 HTTP 令牌允许调用方使用已配置的 API Key，不会创建另一个面板身份。
两种模式的 MCP 凭据不能互换。

## 安装

### 从源码构建

使用 Go 构建，v2 源码分支为 `dev-v2`：

```sh
git clone --branch dev-v2 https://github.com/1Panel-dev/mcp-1panel.git
cd mcp-1panel
go mod download
make build
./build/mcp-1panel --version
```

`make build` 在 `build/mcp-1panel` 生成当前宿主机平台的二进制。
交叉构建 Linux 可用 `make build_linux GOARCH=amd64` 或 `GOARCH=arm64`；
`make build_linux_all` 构建全部七种架构：386、amd64、arm64、armv7、s390x、ppc64le 和 riscv64。
Linux 构建要求本机已有工具链和依赖缓存，产物为 `build/mcp-1panel-linux-{arch}`。

### 发布二进制

从 [Releases](https://github.com/1Panel-dev/mcp-1panel/releases) 选择明确发布、与已安装面板公开 API
兼容的 v2 版本。下载对应架构文件和 `SHA256SUMS`，核对校验和后运行。以 Linux amd64 为例：

```sh
chmod +x ./mcp-1panel-linux-amd64
./mcp-1panel-linux-amd64 --version
```

下方客户端配置中的路径应替换为该二进制的绝对路径。

维护者点击 GitHub 的 **Publish release** 后，工作流自动测试、构建七架构 Linux 二进制，
将产物及 `SHA256SUMS` 上传到该 Release，并推送多架构 Docker 镜像。
普通提交、标签推送和保存草稿不触发此工作流。版本取自 Release 标签；
预发布版本及标签含 `-` 的版本不更新 Docker `latest`。

### Docker

镜像默认运行独立 stdio 模式。本机构建并导出固定版本（不会发布镜像）：

```sh
make docker_build VERSION=v2.0.0-rc1 DOCKER_PLATFORM=linux/amd64
make docker_save VERSION=v2.0.0-rc1 DOCKER_PLATFORM=linux/amd64
# 传输到匹配架构的 Linux 主机后导入：
docker load -i build/mcp-1panel-linux-amd64-v2.0.0-rc1.tar
```

请选择实际使用的固定版本；示例标签不代表已发布。
官方仓库为 `1panel/mcp-server`，Docker 构建支持 `linux/386`、`linux/amd64`、`linux/arm64`、
`linux/arm/v7`、`linux/s390x`、`linux/ppc64le` 和 `linux/riscv64`。
托管模式通过面板安装，详见[容器契约](panel-integration.zh-Hans.md#container-and-socket-contract)。

## 连接客户端

### stdio

由 MCP 客户端启动二进制，填写绝对路径并替换占位值：

```json
{
  "mcpServers": {
    "mcp-1panel": {
      "command": "/absolute/path/to/mcp-1panel",
      "args": ["--mode=legacy", "--transport=stdio", "--access-level=readonly"],
      "env": {
        "PANEL_HOST": "https://panel.example",
        "PANEL_ACCESS_TOKEN": "<1Panel API Key>",
        "MCP_LANGUAGE": "zh-CN"
      }
    }
  }
}
```

stdio 不监听端口，由客户端管理时无需另行启动服务进程。
使用 Docker 时，让客户端以相同环境变量启动以下命令；容器必须能访问面板地址：

```sh
docker run -i --rm -e PANEL_HOST -e PANEL_ACCESS_TOKEN \
  1panel/mcp-server:v2.0.0-rc1 \
  --mode=legacy --transport=stdio --access-level=readonly
```

使用已构建/导入的镜像或实际发布的标签。容器内 `localhost` 不代表 Docker 宿主机。

### HTTPS / Streamable HTTP

先启动服务，再连接客户端。以下示例使用源码构建产物、持久化本地 CA 和独立 MCP Bearer：

```sh
export PANEL_HOST='https://panel.example'
export PANEL_ACCESS_TOKEN='<1Panel API Key>'
export MCP_AUTH_TOKEN='<separate strong random token>'
./build/mcp-1panel \
  --mode=legacy --transport=streamable-http \
  --addr=https://0.0.0.0:8000/mcp --allow-remote-http \
  --tls-hosts=mcp.example.com --tls-dir=./build/tls \
  --access-level=readonly
```

替换域名、配置 DNS 并放行所需端口。Streamable HTTP 客户端填写
`https://mcp.example.com:8000/mcp`，发送请求头 `Authorization: Bearer <MCP_AUTH_TOKEN>`。
核对启动日志中的 CA 指纹，让客户端信任 `build/tls/ca.crt`。
保留 CA 和私钥供后续启动复用，只向客户端提供 CA 证书，不关闭证书校验。

监听非回环地址必须使用 HTTPS、认证令牌和 `--allow-remote-http`。
客户端若发送 `Origin`，通过 `--allowed-origins=https://client.example.com` 允许其准确值。
同机客户端也可使用 `--addr=http://127.0.0.1:8000/mcp` 并省略 TLS 参数，仍需 Bearer。
这是预共享令牌认证，不是 MCP OAuth 流程。

## 访问级别与工具

独立模式默认 `readonly`，通过 `--access-level` / `MCP_ACCESS_LEVEL` 同时控制列举和执行，
直接调用隐藏工具也会检查。本地级别只能收窄已配置 Key 的能力，不能增加面板权限。
托管模式使用 Core 授权，不使用此选项。

| 级别 | 工具 |
| --- | --- |
| `readonly` | 15 个查询、列表和状态工具 |
| `readwrite` | 18 个工具，增加网站、证书和数据库创建 |
| `full` | 全部 20 个工具，再增加 MySQL 和 OpenResty 安装 |

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
| `create_database` | `readwrite` | 创建本地 MySQL 数据库 |
| `install_mysql` | `full` | 提交 MySQL 安装 |
| `install_openresty` | `full` | 提交 OpenResty 安装 |

独立模式的 `get_installed_app` 可能刷新面板状态缓存，查询分类不代表服务端零写入；
托管模式仅读取已记录状态。

## 节点选择与写入结果

19 个业务工具接受可选的 `nodeId`，默认 `"local"`。
以 `{}` 调用 `list_nodes` 获得 `"7"` 等子节点 ID，再传给业务工具，不能使用节点名称或 IP。
独立模式在每次调用中将 ID 解析为当前名称，通过 `CurrentNode` 路由；本地调用不依赖发现。
托管模式使用稳定 ID 路由，每个 API 步骤重新检查授权。两种模式都不会回退到主节点。
详见[节点授权](panel-integration.zh-Hans.md#node-selection-and-authorization)。

证书签发和应用安装返回的 `requestStatus:accepted` 只表示已提交，不表示已完成。
写入提交后遇网络/读取失败、HTTP 或业务码 5xx，或无效的成功响应，均属于**结果未知**。
任何手动重试前，先在 1Panel 检查实际资源或任务，不可自动重复写入。
断开连接或撤销凭据不会回滚已受理的工作。
详见[工具输入与约束](panel-integration.zh-Hans.md#offline-manifest-and-twenty-shared-tools)。

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
| `--tls-dir`、`--tls-hosts` | 持久化 CA/证书目录，以及逗号分隔的证书 SAN |
| `--allowed-origins` | 逗号分隔的准确 Origin 允许列表 |
| `--allow-remote-http` | 允许已认证 HTTPS 监听非回环地址 |
| `--allow-insecure-http` | 仅供本地开发的未认证回环传输 |
| 错误语言 | `MCP_LANGUAGE=zh-CN` 或 `en`，HTTP `Accept-Language` 优先 |

`--version` 和 `--catalog` 离线输出元数据。面板管理 `--listen` 和 `--panel-socket`；
`--healthcheck` 检查托管 adapter/Core 就绪状态。
元数据和工具列表成功不能替代 API 连通性或业务操作验证。

## 文档与验证

- [集成、授权与托管部署](panel-integration.zh-Hans.md)

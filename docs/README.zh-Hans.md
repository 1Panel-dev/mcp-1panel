# 1Panel MCP Server

1Panel MCP 服务器是一个用于 1Panel 的模型上下文协议（Model Context Protocol，MCP）服务器实现。

## 安装

### 前提条件

- Go 1.25.0 或更高版本
- 已有 1Panel

### 从源代码构建

1. 克隆仓库：
   ```bash
   git https://github.com/1Panel-dev/mcp-1panel.git
   cd mcp-1panel
   ```

2. 构建项目：
   ```bash
   make build
   ```
   将 ./build/mcp-1panel 移动至系统环境变量

### 使用 go install 安装
   ```bash
   go install github.com/1Panel-dev/mcp-1panel@latest
   ```

## 使用方法

**Cursor**、**Windsurf** 配置示例:

### stdio 模式
```json
{
  "mcpServers": {
    "mcp-1panel": {
      "command": "mcp-1panel",
      "env": {
        "PANEL_ACCESS_TOKEN": "<your 1Panel access token>",
        "PANEL_HOST": "such as http://localhost:8080"
      }
    }
  }
}
```

### 使用单体自签 TLS 的 Streamable HTTP

`mcp-1panel` 可以自行生成并持久化本地 CA 和 HTTPS 服务端证书，不依赖 1Panel 的证书管理功能。

```bash
MCP_AUTH_TOKEN=<强随机 MCP 令牌> \
PANEL_HOST=<your 1Panel access address> \
PANEL_ACCESS_TOKEN=<your 1Panel access token> \
mcp-1panel \
  -transport streamable-http \
  -addr "https://127.0.0.1:8000/mcp" \
  -tls-hosts "localhost,127.0.0.1"
```

首次启动时，服务会把 CA 路径和 SHA-256 指纹写入 stderr。请让 MCP 客户端信任生成的 `ca.crt`，不要关闭证书校验。CA 会持续复用，服务端证书则会自动续签。

HTTP 传输默认需要独立的 MCP 认证令牌。客户端必须在每个请求中通过 `Authorization: Bearer <令牌>` 发送认证信息；不再接受私有的 `X-MCP-Token` 请求头。该方式是面向单用户私有部署的预共享令牌模式，不是 MCP OAuth 授权流程。需要标准多用户授权时，应将服务部署在支持 OAuth 的网关之后。

优先为本地桌面客户端使用 stdio。监听非回环地址时，必须同时使用 `https://` 地址、`-allow-remote-http`、认证令牌、明确的证书 SAN 和合适的 Origin 白名单。

### 权限级别

服务默认使用 `readonly`。权限在工具注册阶段强制执行，因此越权工具不会出现在 `tools/list` 中，也无法被直接调用。

| 级别 | 能力 |
|---|---|
| `readonly` | 查询、列表和状态读取 |
| `readwrite` | `readonly` 加现有网站、证书和数据库创建工具 |
| `full` | `readwrite` 加现有应用安装工具 |

通过 `-access-level` 或 `MCP_ACCESS_LEVEL` 配置，命令行参数优先。

### 命令行选项

- `-token`：1Panel 访问令牌；优先使用 `PANEL_ACCESS_TOKEN`，避免密钥出现在进程列表
- `-host`：1Panel 访问地址；推荐使用 `PANEL_HOST` 做环境变量配置
- `-transport`：传输类型（stdio 或 streamable-http，默认：stdio）
- `-addr`：HTTP 传输的基础地址（默认：`http://127.0.0.1:8000`）
- `-mcp-token`：HTTP 传输使用的预共享 Bearer 令牌
- `-allowed-origins`：HTTP 传输允许的 Origin，多个值用逗号分隔
- `-allow-insecure-http`：允许未鉴权 HTTP 传输，仅用于本地开发
- `-allow-remote-http`：允许 HTTPS 传输监听非回环地址
- `-access-level`：工具权限级别（`readonly`、`readwrite` 或 `full`，默认：`readonly`）
- `-tls-dir`：本地 CA 和 HTTPS 服务端证书目录
- `-tls-hosts`：HTTPS 服务端证书的 DNS 名称和 IP，多个值用逗号分隔

### 环境变量

您也可以使用环境变量配置服务器：

- `PANEL_HOST`：1Panel 访问地址
- `PANEL_ACCESS_TOKEN`：1Panel 访问令牌
- `MCP_AUTH_TOKEN`：`streamable-http` 使用的预共享 Bearer 令牌
- `MCP_ACCESS_LEVEL`：工具权限级别（`readonly`、`readwrite` 或 `full`）


## 可用工具

服务器提供了各种与 1Panel 交互的工具：

| 工具                          | 类别 | 最低权限 | 描述               |
|-----------------------------|------|----------|------------------|
| **get_dashboard_info**      | 系统 | `readonly` | 列出概览页状态      |
| **get_system_info**         | 系统 | `readonly` | 获取系统信息        |
| **list_websites**           | 网站 | `readonly` | 列出所有网站        |
| **create_website**          | 网站 | `readwrite` | 创建网站           |
| **list_ssls**               | 证书 | `readonly` | 列出所有证书        |
| **create_ssl**              | 证书 | `readwrite` | 创建证书           |
| **list_installed_apps**     | 应用 | `readonly` | 列出所有已安装应用   |
| **install_openresty**       | 应用 | `full` | 安装 OpenResty     |
| **install_mysql**           | 应用 | `full` | 安装 MySQL         |
| **list_databases**          | 数据库 | `readonly` | 列出所有数据库     |
| **create_database**         | 数据库 | `readwrite` | 创建数据库        |

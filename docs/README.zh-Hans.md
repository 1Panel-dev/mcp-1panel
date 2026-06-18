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

### SSE 模式

HTTP 传输（`sse` 和 `streamable-http`）默认需要独立的 MCP 认证令牌。优先使用 stdio；HTTP 传输默认只允许监听回环地址。只有在可信 TLS 反向代理保护下，才使用 `-allow-remote-http` 允许监听非回环地址。

```bash
MCP_AUTH_TOKEN=<强随机 MCP 令牌> \
PANEL_HOST=<your 1Panel access address> \
PANEL_ACCESS_TOKEN=<your 1Panel access token> \
mcp-1panel -transport sse -addr "http://127.0.0.1:8000/sse"
```

```json
{
  "mcpServers": {
    "mcp-1panel": {
      "url": "http://127.0.0.1:8000/sse",
      "headers": {
        "Authorization": "Bearer <强随机 MCP 令牌>"
      }
    }
  }
}
```

### 命令行选项

- `-token`：1Panel 访问令牌；优先使用 `PANEL_ACCESS_TOKEN`，避免密钥出现在进程列表
- `-host`：1Panel 访问地址；推荐使用 `PANEL_HOST` 做环境变量配置
- `-transport`：传输类型（stdio、sse 或 streamable-http，默认：stdio）
- `-addr`：HTTP 传输的基础地址（默认：`http://127.0.0.1:8000`）
- `-mcp-token`：HTTP 传输使用的 MCP 认证令牌
- `-allowed-origins`：HTTP 传输允许的 Origin，多个值用逗号分隔
- `-allow-insecure-http`：允许未鉴权 HTTP 传输，仅用于本地开发
- `-allow-remote-http`：允许 HTTP 传输监听非回环地址；仅在 TLS 保护下使用

### 环境变量

您也可以使用环境变量配置服务器：

- `PANEL_HOST`：1Panel 访问地址
- `PANEL_ACCESS_TOKEN`：1Panel 访问令牌
- `MCP_AUTH_TOKEN`：`sse` 和 `streamable-http` 使用的 MCP HTTP 认证令牌


## 可用工具

服务器提供了各种与 1Panel 交互的工具：

| 工具                          | 类别 | 描述               |
|-----------------------------|------|------------------|
| **get_dashboard_info**      | 系统 | 列出概览页状态      |
| **get_system_info**         | 系统 | 获取系统信息        |
| **list_websites**           | 网站 | 列出所有网站        |
| **create_website**          | 网站 | 创建网站           |
| **list_ssls**               | 证书 | 列出所有证书        |
| **create_ssl**              | 证书 | 创建证书           |
| **list_installed_apps**     | 应用 | 列出所有已安装应用   |
| **install_openresty**       | 应用 | 安装 OpenResty     |
| **install_mysql**           | 应用 | 安装 MySQL         |
| **list_databases**          | 数据库 | 列出所有数据库     |
| **create_database**         | 数据库 | 创建数据库        |

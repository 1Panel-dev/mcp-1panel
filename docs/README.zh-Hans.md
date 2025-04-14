# 1Panel MCP Server

**1Panel MCP Server** 是 [1Panel](https://github.com/1Panel-dev/1Panel) 的 Model Context Protocol (MCP) 协议服务端实现。

---

## 安装方式

### ✅ 方式一：从 Release 页面下载安装包（推荐）

1. 访问 [Releases 页面](https://github.com/1Panel-dev/mcp-1panel/releases)，下载对应系统的可执行文件。

2. 安装示例（以 `amd64` 为例）：

```bash
chmod +x mcp-1panel-linux-amd64
mv mcp-1panel-linux-amd64 /usr/local/bin/mcp-1panel
```

---

### 🛠️ 方式二：通过源码构建

确保本地已安装 Go 1.23 或更高版本，执行以下命令：

1. 克隆代码仓库：

```bash
git clone https://github.com/1Panel-dev/mcp-1panel.git
cd mcp-1panel
```

2. 构建可执行文件：

```bash
make build
```

3. 可执行文件生成路径为：`./build/mcp-1panel`，建议移动到系统 PATH 目录中。

---

### 🚀 方式三：通过 `go install` 安装

确保本地已安装 Go 1.23 或更高版本：

```bash
go install github.com/1Panel-dev/mcp-1panel@latest
```

---

### 🐳 方式四：通过 Docker 安装

确保本地已正确安装并配置好 Docker。

我们官方提供的镜像支持以下五种架构：

- `amd64`
- `arm64`
- `arm/v7`
- `s390x`
- `ppc64le`

---

## 使用方式

1Panel MCP Server 支持两种运行模式：**stdio** 和 **sse**

---

### 模式一：stdio（默认）

#### 📦 使用本地二进制文件

在 Cursor 或 Windsurf 的配置文件中添加如下内容：

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

#### 🐳 使用 Docker 方式运行

```json
{
  "mcpServers": {
    "mcp-1panel": {
      "command": "docker",
      "args": [
        "run",
        "-i",
        "--rm",
        "-e",
        "PANEL_HOST",
        "-e",
        "PANEL_ACCESS_TOKEN",
        "1panel/1panel-mcp-server"
      ],
      "env": {
        "PANEL_HOST": "such as http://localhost:8080",
        "PANEL_ACCESS_TOKEN": "<your 1Panel access token>"
      }
    }
  }
}
```

---

### 模式二：sse

#### 🚀 启动 MCP Server：

```bash
mcp-1panel -host http://localhost:8080 -token <your 1Panel access token> -transport sse -addr http://localhost:8000
```

#### ⚙️ 配置 Cursor 或 Windsurf：

```json
{
  "mcpServers": {
    "mcp-1panel": {
      "url": "http://localhost:8000/sse"
    }
  }
}
```

---

### 🔧 命令行参数

- `-token`: 1Panel 的访问令牌
- `-host`: 1Panel 的地址，如：http://localhost:8080
- `-transport`: 传输方式：`stdio` 或 `sse`，默认是 `stdio`
- `-addr`: SSE 服务监听地址，默认是 `http://localhost:8000`

---

## 🧰 可用工具（Tools）

以下是 MCP Server 提供的工具列表，用于与 1Panel 交互：

| 工具名称                | 分类        | 描述                             |
|-------------------------|-------------|----------------------------------|
| `get_dashboard_info`    | System      | 获取仪表盘状态                   |
| `get_system_info`       | System      | 获取系统信息                     |
| `list_websites`         | Website     | 列出所有网站                     |
| `create_website`        | Website     | 创建新网站                       |
| `list_ssls`             | Certificate | 列出所有证书                     |
| `create_ssl`            | Certificate | 创建新证书                       |
| `list_installed_apps`   | Application | 列出已安装应用                   |
| `install_openresty`     | Application | 安装 OpenResty                   |
| `install_mysql`         | Application | 安装 MySQL                       |
| `list_databases`        | Database    | 列出所有数据库                   |
| `create_database`       | Database    | 创建新数据库                     |

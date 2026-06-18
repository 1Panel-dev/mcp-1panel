# 1Panel MCP Server

1Panel MCP Server is an implementation of the Model Context Protocol (MCP) server for 1Panel.

## Installation

### Prerequisites

- Go 1.25.0 or higher
- Existing 1Panel

### Build from Source

1. Clone the repository:
   ```bash
   git clone https://github.com/1Panel-dev/mcp-1panel.git
   cd mcp-1panel
   ```

2. Build the project:
   ```bash
   make build
   ```
   Move `./build/mcp-1panel` to the system environment path.

### Install using go install
   ```bash
   go install github.com/1Panel-dev/mcp-1panel@latest
   ```

## Usage

**Cursor** and **Windsurf** configuration example:

### stdio mode
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

### sse mode

start mcp server through sse
```
MCP_AUTH_TOKEN=<strong random MCP token> \
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
          "Authorization": "Bearer <strong random MCP token>"
        }
    }
  }
}
```

HTTP transports (`sse` and `streamable-http`) require an MCP authentication token by default. Use stdio for local desktop clients when possible. HTTP transports listen on loopback addresses only by default. If you expose an HTTP transport beyond loopback with `-allow-remote-http`, terminate TLS at a trusted reverse proxy and set an explicit Origin allowlist.

### Command Line Options

- `-token`: 1Panel access token; prefer `PANEL_ACCESS_TOKEN` to avoid exposing secrets in process lists
- `-host`: 1Panel access address; prefer `PANEL_HOST` for environment-based configuration
- `-transport`: Transport type (stdio, sse, or streamable-http; default: stdio)
- `-addr`: Base URL for HTTP transports (default: `http://127.0.0.1:8000`)
- `-mcp-token`: MCP HTTP authentication token for HTTP transports
- `-allowed-origins`: Comma-separated Origin allowlist for HTTP transports
- `-allow-insecure-http`: Allow unauthenticated HTTP transports; only use for local development
- `-allow-remote-http`: Allow HTTP transports to listen on non-loopback addresses; only use behind TLS

### Environment Variables

You can also configure the server using environment variables:

- `PANEL_HOST`: 1Panel access address
- `PANEL_ACCESS_TOKEN`: 1Panel access token
- `MCP_AUTH_TOKEN`: MCP HTTP authentication token for `sse` and `streamable-http`

## Available Tools

The server provides various tools for interacting with 1Panel:

| Tool                        | Category | Description            |
|-----------------------------|----------|------------------------|
| **get_dashboard_info**      | System   | List dashboard status  |
| **get_system_info**         | System   | Get system information |
| **list_websites**           | Website  | List all websites      |
| **create_website**          | Website  | Create a website       |
| **list_ssls**               | Certificate | List all certificates |
| **create_ssl**              | Certificate | Create a certificate  |
| **list_installed_apps**     | Application | List all installed applications |
| **install_openresty**       | Application | Install OpenResty     |
| **install_mysql**           | Application | Install MySQL         |
| **list_databases**          | Database | List all databases     |
| **create_database**         | Database | Create a database      |

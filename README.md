# 1Panel MCP Server

**1Panel MCP Server** is an implementation of the Model Context Protocol (MCP) server for [1Panel](https://github.com/1Panel-dev/1Panel).

## Installation Methods

### Method 1: Download from Release Page (Recommended)

1. Visit the [Releases Page](https://github.com/1Panel-dev/mcp-1panel/releases) and download the executable file corresponding to your system.

2. Example installation (for amd64):

```bash
chmod +x mcp-1panel-linux-amd64
mv mcp-1panel-linux-amd64 /usr/local/bin/mcp-1panel
```

### Method 2: Build from Source

Make sure Go 1.23 or later is installed locally. Then run:

1. Clone the repository:

```bash
git clone https://github.com/1Panel-dev/mcp-1panel.git
cd mcp-1panel
```

2. Build the executable:

```bash
make build
```

> Move ./build/mcp-1panel to a directory included in your system's PATH.

### Method 3: Install via go install

Make sure Go 1.23 or later is installed locally. Then run:

```bash
go install github.com/1Panel-dev/mcp-1panel@latest
```

### Method 4: Install via Docker

Make sure Docker is correctly installed and configured on your machine.

The official image supports the following architectures:

- amd64
- arm64
- arm/v7
- s390x
- ppc64le

## Usage

1Panel MCP Server supports two running modes: `stdio` and `sse`.

### stdio Mode

#### Using Local Binary

In the configuration file of Cursor or Windsurf, add:

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

#### Running in Docker

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

### sse Mode

1. Start the MCP Server:

```bash
mcp-1panel -host http://localhost:8080 -token <your 1Panel access token> -transport sse -addr http://localhost:8000
```

2. Configure in Cursor or Windsurf:

```json
{
  "mcpServers": {
    "mcp-1panel": {
      "url": "http://localhost:8000/sse"
    }
  }
}
```

#### Command Line Options

- `-token`: 1Panel access token
- `-host`: 1Panel access address
- `-transport`: Transport type (stdio or sse, default: stdio)
- `-addr`: Start SSE server address (default: http://localhost:8000)

## Available Tools

The server provides various tools for interacting with 1Panel:

| Tool                        | Category     | Description               |
|-----------------------------|--------------|---------------------------|
| **get_dashboard_info**      | System       | List dashboard status     |
| **get_system_info**         | System       | Get system information    |
| **list_websites**           | Website      | List all websites         |
| **create_website**          | Website      | Create a website          |
| **list_ssls**               | Certificate  | List all certificates     |
| **create_ssl**              | Certificate  | Create a certificate      |
| **list_installed_apps**     | Application  | List installed apps       |
| **install_openresty**       | Application  | Install OpenResty         |
| **install_mysql**           | Application  | Install MySQL             |
| **list_databases**          | Database     | List all databases        |
| **create_database**         | Database     | Create a database         |
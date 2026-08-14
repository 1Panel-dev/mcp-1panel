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

### Streamable HTTP with standalone TLS

`mcp-1panel` can create and persist its own local CA and HTTPS server certificate. It does not depend on 1Panel certificate management.

```bash
MCP_AUTH_TOKEN=<strong random MCP token> \
PANEL_HOST=<your 1Panel access address> \
PANEL_ACCESS_TOKEN=<your 1Panel access token> \
mcp-1panel \
  -transport streamable-http \
  -addr "https://127.0.0.1:8000/mcp" \
  -tls-hosts "localhost,127.0.0.1"
```

On first startup, the server writes the CA path and SHA-256 fingerprint to stderr. Configure the MCP client to trust the generated `ca.crt`; do not disable certificate verification. The CA is reused while the server certificate is renewed automatically.

HTTP transports require an MCP authentication token by default. Clients must send it on every request as `Authorization: Bearer <token>`; the private `X-MCP-Token` header is not accepted. This is a pre-shared token mode intended for a single-user/private deployment, not the MCP OAuth authorization flow. Put the server behind an OAuth-capable gateway when standards-based multi-user authorization is required.

Use stdio for local desktop clients when possible. Non-loopback listeners require an `https://` address, `-allow-remote-http`, a token, explicit certificate SANs, and an appropriate Origin allowlist.

### Access levels

The server defaults to `readonly`. Tool permissions are enforced when tools are registered, so disallowed tools are not returned by `tools/list` and cannot be called directly.

| Level | Tools |
|---|---|
| `readonly` | Queries, lists, and status reads |
| `readwrite` | `readonly` plus existing website, certificate, and database creation tools |
| `full` | `readwrite` plus existing application installation tools |

Set the level with `-access-level` or `MCP_ACCESS_LEVEL`. Command-line configuration takes precedence.

### Command Line Options

- `-token`: 1Panel access token; prefer `PANEL_ACCESS_TOKEN` to avoid exposing secrets in process lists
- `-host`: 1Panel access address; prefer `PANEL_HOST` for environment-based configuration
- `-transport`: Transport type (stdio or streamable-http; default: stdio)
- `-addr`: Base URL for HTTP transports (default: `http://127.0.0.1:8000`)
- `-mcp-token`: Pre-shared Bearer token for HTTP transports
- `-allowed-origins`: Comma-separated Origin allowlist for HTTP transports
- `-allow-insecure-http`: Allow unauthenticated HTTP transports; only use for local development
- `-allow-remote-http`: Allow HTTPS transports to listen on non-loopback addresses
- `-access-level`: Tool access level (`readonly`, `readwrite`, or `full`; default: `readonly`)
- `-tls-dir`: Directory for the local CA and HTTPS server certificate
- `-tls-hosts`: Comma-separated DNS names and IP addresses for the HTTPS server certificate

### Environment Variables

You can also configure the server using environment variables:

- `PANEL_HOST`: 1Panel access address
- `PANEL_ACCESS_TOKEN`: 1Panel access token
- `MCP_AUTH_TOKEN`: Pre-shared Bearer token for `streamable-http`
- `MCP_ACCESS_LEVEL`: Tool access level (`readonly`, `readwrite`, or `full`)

## Available Tools

The server provides various tools for interacting with 1Panel:

| Tool                        | Category | Minimum access | Description            |
|-----------------------------|----------|----------------|------------------------|
| **get_dashboard_info**      | System   | `readonly`     | List dashboard status  |
| **get_system_info**         | System   | `readonly`     | Get system information |
| **list_websites**           | Website  | `readonly`     | List all websites      |
| **create_website**          | Website  | `readwrite`    | Create a website       |
| **list_ssls**               | Certificate | `readonly`  | List all certificates |
| **create_ssl**              | Certificate | `readwrite` | Create a certificate  |
| **list_installed_apps**     | Application | `readonly`  | List all installed applications |
| **install_openresty**       | Application | `full`      | Install OpenResty     |
| **install_mysql**           | Application | `full`      | Install MySQL         |
| **list_databases**          | Database | `readonly`     | List all databases     |
| **create_database**         | Database | `readwrite`    | Create a database      |

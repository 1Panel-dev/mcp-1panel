# 1Panel MCP Server

[English](README.md) | [简体中文](docs/README.zh-Hans.md)

1Panel MCP Server provides 21 tools for managing 1Panel v2 nodes, system status,
websites, certificates, applications and MySQL through MCP clients. It supports
independent deployment with a panel API Key and managed deployment with 1Panel's
credential authorization and audit controls.

## Choose a mode

| Mode | Connection and authorization | Deployment |
| --- | --- | --- |
| Independent: `--mode=legacy` (default) | Calls the panel's public APIs using a configured API Key; panel permissions apply to every request | stdio or Streamable HTTP, as a binary or Docker container |
| Managed: `--mode=panel` | Uses a dedicated MCP credential; Core checks the owner's current permissions, node and tool grants, and policy | Docker on the primary node, installed and managed from the panel's MCP service page |

Independent mode requires the panel's public API to be enabled and the installed
1Panel v2 release to support the selected tools. A managed MCP installation is not
required. Managed mode requires compatible versions of Core, the primary and child
Agents, and the frontend.

| Credential | Purpose |
| --- | --- |
| `PANEL_ACCESS_TOKEN` / `--token` | 1Panel API Key used by independent mode for outgoing panel requests |
| `MCP_AUTH_TOKEN` / `--mcp-token` | Separate pre-shared Bearer used by clients connecting to independent Streamable HTTP |
| Token created in the panel's MCP page | Dedicated managed credential with explicit node/tool grants; clients use the panel's HTTPS `/mcp` endpoint |

Clients using the same independent HTTP token operate under the configured API
Key's identity and permissions. This token does not provide a separate panel
identity or per-client authorization. Credentials for the two modes are not
interchangeable.

## Install

### Download from Releases

Select a published v2 version from
[GitHub Releases](https://github.com/1Panel-dev/mcp-1panel/releases) that is
compatible with the installed panel's public APIs. Download the Linux binary for
the target architecture and `SHA256SUMS` from the same release. Running a release
binary does not require Go.

Release assets are standalone executables and do not need extraction. For example,
on Linux amd64, place `mcp-1panel-linux-amd64` and `SHA256SUMS` in the same directory:

```sh
sha256sum --ignore-missing -c SHA256SUMS
chmod +x ./mcp-1panel-linux-amd64
./mcp-1panel-linux-amd64 --version
```

For Linux arm64, download `mcp-1panel-linux-arm64` and use that filename instead.
Use the binary's absolute path in the client configuration. For macOS or Windows,
use the Docker deployment described below.

### Docker

Use the official [1panel/mcp-server](https://hub.docker.com/r/1panel/mcp-server)
image with [Docker Compose](docker-compose.yml). The image defaults to stdio;
the following configuration runs an independent Streamable HTTP service.
Save the following as `docker-compose.yml`:

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

Select a published fixed image tag that includes authenticated remote HTTP support
and is compatible with the panel APIs and host architecture. Replace `YOUR_VERSION`
with the selected release tag.
Create a `.env` file alongside `docker-compose.yml` and replace the placeholders:

```dotenv
MCP_IMAGE=1panel/mcp-server:YOUR_VERSION
PANEL_HOST=https://panel.example.com
PANEL_ACCESS_TOKEN=YOUR_PANEL_API_KEY
MCP_AUTH_TOKEN=YOUR_MCP_AUTH_TOKEN
```

Use a separate, strong random value for `MCP_AUTH_TOKEN`. Restrict access to this
credential-bearing file to service administrators. HTTPS with certificate
verification is recommended for `PANEL_HOST`.

Start the service:

```sh
docker compose pull
docker compose up -d
```

- HTTP endpoint: `http://<server-IP>:16082/`, for trusted networks or the proxy upstream.
- Authentication header: `Authorization: Bearer <MCP_AUTH_TOKEN>`.
- Recommended production endpoint: configure a reverse-proxy website in 1Panel
  with a valid certificate, then connect to `https://mcp.example.com/`.

The Compose configuration publishes the HTTP port on all host interfaces by
default. Restrict access to this upstream port according to the deployment's
network topology. Website HTTPS terminates TLS at the proxy; the MCP container
continues to use HTTP. For proxy networking, paths and optional direct binary
HTTPS.

## Connect a client

### stdio

Configure the MCP client to launch the binary. Replace the executable path and
credential placeholders:

```json
{
  "mcpServers": {
    "mcp-1panel": {
      "command": "/absolute/path/to/mcp-1panel-linux-amd64",
      "args": ["--mode=legacy", "--transport=stdio", "--access-level=readonly"],
      "env": {
        "PANEL_HOST": "https://panel.example.com",
        "PANEL_ACCESS_TOKEN": "YOUR_PANEL_API_KEY",
        "MCP_LANGUAGE": "en"
      }
    }
  }
}
```

With stdio, the client manages the server process and communicates over standard
input and output; no network port is opened. For a persistent network service,
use the Compose deployment above.

## Access levels and tools

Independent mode defaults to `readonly`. `--access-level` / `MCP_ACCESS_LEVEL`
controls both tool visibility and execution, including direct calls to tools
excluded from the listing. It restricts the configured API Key's capabilities and
cannot add panel permissions. Managed mode uses Core grants instead of this option.

| Level | Tools |
| --- | --- |
| `readonly` | 15 query, list and status tools |
| `readwrite` | 19 tools, adding website, certificate and database creation, and existing certificate binding |
| `full` | All 21, adding MySQL and OpenResty installation |

| Tool | Minimum level | Purpose |
| --- | --- | --- |
| `list_nodes` | `readonly` | Discover permitted node IDs |
| `get_system_info` | `readonly` | System information |
| `get_dashboard_info` | `readonly` | Dashboard status |
| `list_websites` | `readonly` | Website list |
| `get_website` | `readonly` | Website metadata |
| `list_website_domains` | `readonly` | Domains and ports |
| `get_website_https` | `readonly` | HTTPS and certificate metadata |
| `list_ssls` | `readonly` | Certificate list |
| `get_ssl_metadata` | `readonly` | Certificate metadata |
| `list_installed_apps` | `readonly` | Installed application list |
| `get_installed_app` | `readonly` | Installation state, version and ports |
| `list_app_versions` | `readonly` | Available application versions |
| `list_databases` | `readonly` | Databases in a selected local MySQL instance |
| `list_mysql_instances` | `readonly` | Recorded local MySQL instances |
| `get_mysql_status` | `readonly` | Fixed MySQL status counters |
| `create_website` | `readwrite` | Create a static or proxy website |
| `create_ssl` | `readwrite` | Submit certificate issuance |
| `create_database` | `readwrite` | Create a local MySQL database |
| `install_mysql` | `full` | Submit MySQL installation |
| `install_openresty` | `full` | Submit OpenResty installation |

In independent mode, `get_installed_app` may refresh the panel's status cache.
Its query classification therefore does not imply an operation without server-side
writes. Managed mode reads the recorded state.

## Nodes and write outcomes

The 20 business tools accept an optional `nodeId`, which defaults to `"local"`.
Use `list_nodes` with `{}` to obtain permitted child-node IDs, such as `"7"`, and
pass the selected ID to subsequent tool calls. Node names and IP addresses are not
valid selectors. Independent mode resolves the ID against the current node list
for each call; local calls do not require node discovery. Managed mode routes by
stable ID and rechecks authorization at each API step. Neither mode falls back to
the primary node when the selected node is unavailable or unauthorized. 

`bind_ssl` requires explicit `website_id` and `ssl_id` values on the selected node.
It supports running, top-level static websites with concrete ASCII or punycode DNS
names. Before writing, it checks website, domain, HTTPS and certificate metadata,
including certificate readiness, validity dates and domain coverage. Binding is
rejected if the website already records a different certificate. Managed
credentials require an explicit grant for this tool. After a tool policy update,
refresh the managed tool catalog and explicitly confirm its authorization.

Initial HTTPS configuration enables HTTP-to-HTTPS redirection and TLS 1.3/1.2,
uses the panel's default ciphers, and disables HSTS and HTTP/3. This saves the full
HTTPS configuration; custom settings on a previously unbound website may not be
preserved. The `applied` result confirms that the save succeeded and the subsequent
readback matched the certificate association, redirect, protocol, HSTS and HTTP/3
settings. The `already_bound` result means the same certificate was already
recorded and no write was submitted. Neither result verifies the certificate
actually served over TLS or provides isolation from concurrent panel or API
changes. Failed or uncertain writes are not automatically retried or reversed.

Certificate issuance and application installation return `requestStatus:accepted`
when the request has been submitted; this does not confirm completion. A submitted
write followed by a network or read failure, an HTTP or business 5xx error, or an
invalid success response has an **unknown outcome**. Verify the resource or task
state in 1Panel before any manual retry. Never automatically repeat a write with
an unknown outcome. Disconnecting or revoking credentials does not roll back
accepted work.

## Configuration reference

Command-line values take precedence over their environment counterparts.

| Option | Environment / default |
| --- | --- |
| `--mode` | `legacy` or `panel`; default `legacy` |
| `--host`, `--token` | `PANEL_HOST`, `PANEL_ACCESS_TOKEN` |
| `--key-id` | Optional `PANEL_KEY_ID` for panels with multiple API Keys |
| `--transport` | `stdio` (default) or `streamable-http` |
| `--addr` | Default `http://127.0.0.1:8000`; include `/mcp` when desired |
| `--mcp-token` | `MCP_AUTH_TOKEN`, required for independent HTTP authentication |
| `--access-level` | `MCP_ACCESS_LEVEL`; default `readonly` |
| `--tls-dir`, `--tls-hosts` | Only for HTTPS served by MCP itself; not needed for website HTTPS proxying |
| `--allowed-origins` | Comma-separated exact Origin allowlist |
| `--allow-remote-http` | Permit authenticated HTTP or HTTPS on non-loopback addresses |
| `--allow-insecure-http` | Unauthenticated loopback transport for local development only |
| Error language | `MCP_LANGUAGE=zh-CN` or `en`; HTTP `Accept-Language` takes priority; tool identifiers and result fields remain unchanged |

`--version` and `--catalog` print metadata offline. The panel manages `--listen`
and `--panel-socket`; `--healthcheck` checks managed adapter/Core readiness.
Metadata and tool listing alone do not validate API connectivity or business operations.

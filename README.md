# 1Panel MCP Server

[English](README.md) | [简体中文](docs/README.zh-Hans.md)

An MCP server for 1Panel v2, with 20 shared tools for nodes, system status,
websites, certificates, applications and MySQL. Independent mode uses the
official Go MCP SDK; managed mode integrates with 1Panel's authorization and audit.

## Choose a mode

| Mode | Connection and authorization | Deployment |
| --- | --- | --- |
| Independent: `--mode=legacy` (default) | Configured API Key calls existing public `/api/v2` APIs; panel permissions apply to every request | stdio or Streamable HTTP, as a binary or Docker container |
| Managed: `--mode=panel` | Dedicated MCP credential; Core checks the owner's current permissions, node/tool grants and policy | Docker on the primary node, installed from **AI → MCP Management → 1Panel MCP** |

Independent mode requires an enabled public API and compatible APIs/DTOs for the
selected tools on your installed 1Panel v2 release. It needs no managed MCP
installation. Managed mode requires matching Core, primary/child Agents and frontend.
See [managed deployment](docs/panel-integration.md#docker-deployment).

| Credential | Purpose |
| --- | --- |
| `PANEL_ACCESS_TOKEN` / `--token` | 1Panel API Key used by independent mode for outgoing panel requests |
| `MCP_AUTH_TOKEN` / `--mcp-token` | Separate pre-shared Bearer used by clients connecting to independent Streamable HTTP |
| Token created in the panel's MCP page | Dedicated managed credential with explicit node/tool grants; clients use the panel's HTTPS `/mcp` endpoint |

The independent HTTP token permits use of the configured API Key; it does not
create another panel identity. The two modes' MCP credentials are not interchangeable.

## Install

### Source

Use Go **1.27.3**. The v2 source branch is `dev-v2`:

```sh
git clone --branch dev-v2 https://github.com/1Panel-dev/mcp-1panel.git
cd mcp-1panel
go mod download
make build
./build/mcp-1panel --version
```

`make build` creates a host-platform binary at `build/mcp-1panel`.
To cross-build Linux, use `make build_linux GOARCH=amd64` or `GOARCH=arm64`;
`make build_linux_all` builds all seven architectures: 386, amd64, arm64, armv7, s390x, ppc64le and riscv64.
Linux builds require a locally available toolchain and cached dependencies;
outputs are `build/mcp-1panel-linux-{arch}`.

### Release binary

Choose an explicitly published v2 release from [Releases](https://github.com/1Panel-dev/mcp-1panel/releases)
compatible with your installed panel's public APIs. Download the matching architecture
and `SHA256SUMS`, verify the checksum, then run, for example on Linux amd64:

```sh
chmod +x ./mcp-1panel-linux-amd64
./mcp-1panel-linux-amd64 --version
```

Use that binary's absolute path in the client configuration below.

For maintainers, publishing a GitHub Release triggers tests, builds all seven Linux binaries,
uploads them with `SHA256SUMS` to that release, and pushes the multi-platform Docker image.
Ordinary commits, tag pushes and saved drafts do not trigger this workflow. The release tag
provides the version; prereleases and tags containing `-` do not update Docker `latest`.

### Docker

The image defaults to independent stdio mode. To build and export a fixed version
locally (these commands do not publish an image):

```sh
make docker_build VERSION=v2.0.0-rc1 DOCKER_PLATFORM=linux/amd64
make docker_save VERSION=v2.0.0-rc1 DOCKER_PLATFORM=linux/amd64
# Transfer to a matching Linux host, then import:
docker load -i build/mcp-1panel-linux-amd64-v2.0.0-rc1.tar
```

Choose your own fixed version; the example tag is not a publication claim.
The official repository is `1panel/mcp-server`. Docker builds support
`linux/386`, `linux/amd64`, `linux/arm64`, `linux/arm/v7`, `linux/s390x`, `linux/ppc64le` and
`linux/riscv64`. For managed installation, use the panel's installation dialog and
[container contract](docs/panel-integration.md#container-and-socket-contract).

## Connect a client

### stdio

Let the MCP client start the binary; use its absolute path and replace placeholders:

```json
{
  "mcpServers": {
    "mcp-1panel": {
      "command": "/absolute/path/to/mcp-1panel",
      "args": ["--mode=legacy", "--transport=stdio", "--access-level=readonly"],
      "env": {
        "PANEL_HOST": "https://panel.example",
        "PANEL_ACCESS_TOKEN": "<1Panel API Key>",
        "MCP_LANGUAGE": "en"
      }
    }
  }
}
```

stdio does not listen on a port. When client-managed, no separate server process
is needed. To use Docker instead, the client must launch this command with the
same environment variables; the panel URL must be reachable from the container:

```sh
docker run -i --rm -e PANEL_HOST -e PANEL_ACCESS_TOKEN \
  1panel/mcp-server:v2.0.0-rc1 \
  --mode=legacy --transport=stdio --access-level=readonly
```

Use an image you built/imported or an actually published tag. Container `localhost`
is not the Docker host.

### HTTPS / Streamable HTTP

Start the server before connecting. This example uses the source-built binary,
a persistent local CA, and an independent MCP Bearer:

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

Replace the domain, configure DNS and allow the required port. Configure a
Streamable HTTP client with `https://mcp.example.com:8000/mcp` and the header
`Authorization: Bearer <MCP_AUTH_TOKEN>`. Verify the CA fingerprint printed at
startup and trust `build/tls/ca.crt` in the client. Keep the CA/private keys for
reuse; share only the CA certificate. Do not disable certificate verification.

Non-loopback listening requires HTTPS, authentication and `--allow-remote-http`.
If the client sends `Origin`, allow its exact value with
`--allowed-origins=https://client.example.com`. Same-host clients can instead use
`--addr=http://127.0.0.1:8000/mcp` without TLS flags; the Bearer is still required.
This is pre-shared authentication, not an MCP OAuth flow.

## Access levels and tools

Independent mode defaults to `readonly`. `--access-level` / `MCP_ACCESS_LEVEL`
controls both listing and execution, including direct calls to hidden tools.
It restricts the configured Key and cannot add panel permissions. Managed mode
uses Core grants instead of this option.

| Level | Tools |
| --- | --- |
| `readonly` | 15 query, list and status tools |
| `readwrite` | 18 tools, adding website, certificate and database creation |
| `full` | All 20, adding MySQL and OpenResty installation |

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

`get_installed_app` may refresh the panel's status cache in independent mode;
its query classification does not promise zero server writes. Managed mode reads recorded state.

## Nodes and write outcomes

The 19 business tools accept optional `nodeId`, defaulting to `"local"`. Call
`list_nodes` with `{}` for child IDs such as `"7"`, then pass the ID to a business
tool. Names and IPs are not node selectors. Independent mode resolves ID to current
name per call and routes through `CurrentNode`; local calls need no discovery.
Managed mode uses stable-ID routing and rechecks grants on each API step.
Neither mode falls back to the primary node. See [node authorization](docs/panel-integration.md#node-selection-and-authorization).

Certificate issuance and app installation return `requestStatus:accepted`, which
means submitted, not completed. A submitted write followed by a network/read
failure, HTTP/business 5xx or an invalid success response has an **unknown outcome**.
Check the actual resource or task in 1Panel before any manual retry; never automatically
repeat the write. Disconnecting or revoking credentials does not roll back accepted work.
See [tool inputs and constraints](docs/panel-integration.md#offline-manifest-and-twenty-shared-tools).

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
| `--tls-dir`, `--tls-hosts` | Persistent CA/certificate directory and comma-separated certificate SANs |
| `--allowed-origins` | Comma-separated exact Origin allowlist |
| `--allow-remote-http` | Permit authenticated HTTPS on non-loopback addresses |
| `--allow-insecure-http` | Unauthenticated loopback transport for local development only |
| Error language | `MCP_LANGUAGE=zh-CN` or `en`; HTTP `Accept-Language` takes priority |

`--version` and `--catalog` print metadata offline. The panel manages `--listen`
and `--panel-socket`; `--healthcheck` checks managed adapter/Core readiness.
Metadata and tool listing alone do not validate API connectivity or business operations.

## Documentation and validation

- [Integration, authorization and managed deployment](docs/panel-integration.md)

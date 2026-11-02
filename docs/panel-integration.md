[English](panel-integration.md) | [中文(简体)](panel-integration.zh-Hans.md)

<a id="1panel-v2-mcp-integration"></a>

# 1Panel v2 MCP integration

This guide covers deployment, authorization and the runtime contract. For downloads, source builds and
client startup, see the [README](../README.md). Both modes share 20 tools, strict input schemas and
result projections.

| Mode | Authentication and execution | Deployment requirement |
| --- | --- | --- |
| Independent (`--mode=legacy`) | Configured API Key → existing public `/api/v2` APIs | Installed v2 APIs and compatible DTOs; no managed MCP installation |
| Managed (`--mode=panel`) | Dedicated MCP credential → Core authorization → target Agent | Matching Core, primary/child Agents, frontend and adapter contract v5 |

<a id="independent-api-key-mode"></a>

## Independent API Key mode

Configure `PANEL_HOST` / `PANEL_ACCESS_TOKEN`; optional `PANEL_KEY_ID` selects a multiple-key identity
if supported by the panel. The host/key configuration is immutable. Every request receives fresh
`1Panel-Timestamp` and `1Panel-Token` signatures and optional `1Panel-Key-ID`. Redirects, environment
proxies and ambient cookies are disabled. Tool arguments cannot change the host, credentials or headers.

`readonly` exposes 15 query tools; `readwrite` exposes 18, adding website, certificate and database
creation; `full` exposes all 20, adding MySQL/OpenResty installation. The adapter enforces this policy
on every listing and call, including direct calls to hidden tools. It cannot grant additional panel
permissions. Every API request still follows the account's ordinary API Key and business rules. This
mode does not acquire managed MCP grants, admission tickets, invocation audit, Agent snapshot semantics
or managed final resource revalidation.

For Streamable HTTP, `MCP_AUTH_TOKEN` / `--mcp-token` protects the incoming MCP endpoint. This
pre-shared Bearer is separate from the outgoing API Key and managed MCP credentials. Follow the README's
HTTPS, Origin and listen restrictions. The official Go MCP SDK handles negotiation and lifecycle. Tools
require the APIs and DTOs supported by the installed v2 release; support for every release is not
implied.

<a id="managed-panel-mode"></a>
<a id="docker-deployment"></a>
<a id="install-and-configure-from-1panel"></a>

## Managed deployment and upgrades

Managed MCP runs as one Docker container on the primary node. Core/Agent reads its tool manifest,
authenticates operations as the credential's current owner and executes existing APIs. No adapter or MCP
credential is deployed to child nodes.

1. Deploy matching Core, frontend and primary/child Agents. Child Agents must
   support the operation-bound identity and resource guards required by Core.
2. Prepare the primary node's rootful Docker Engine and a compatible adapter image.
   Seven image architectures are supported: 386, amd64, arm64, arm/v7, s390x, ppc64le and riscv64.
3. Open **AI → MCP管理 → 1Panel MCP → Install**. Choose a fixed semantic version tag
   or SHA256 digest from `1panel/mcp-server`, the loopback port (default `18081`),
   source access policy and external HTTPS `/mcp` URL as appropriate.
4. Wait for installation to finish. The panel validates the image platform,
   `--version`, `--catalog` and contract v5, then starts the verified immutable image ID.
   First installation enables the service; upgrades preserve its enabled state.
   If installation fails, inspect the installation and recovery status before retrying.
5. Create a dedicated credential, explicitly grant nodes and tools, and configure
   the client using the displayed HTTPS endpoint and token. Grant `list_nodes`
   separately when discovery is required.

`1panel/mcp-server:<release-version>` is a placeholder, not a published-version claim. Select a version
actually available for the target architecture; `latest` and custom repositories are rejected for
managed installation. Source build, export and release commands are documented in the README. Local
build/export does not publish an image. Bare binaries are independent-mode artifacts, not managed
installs.

Install, configuration and lifecycle actions require service-management permission and
superadministrator identity. Changes use the current `revision`; stale edits are rejected. Let accepted
calls finish before upgrading and use the panel as the lifecycle owner for install, start/stop/restart
and configuration.

After upgrading, refresh the catalog. Core compares current capabilities with the approved authorization
snapshot: equivalent or provably narrower tools retain grants; expanded or unverifiable capabilities
require explicit review and saving. New tools never join credentials automatically. Every call uses the
exact current schema and policy hash. Agent API changes require the matching Agent release.

<a id="container-and-socket-contract"></a>

### Container and Socket requirements

The supported host is Linux with rootful Docker on the same primary node as Core, without a remote
daemon, rootless Docker or user-namespace remapping. Host networking keeps the adapter on literal
`127.0.0.1:<ListenPort>`; clients use the panel's HTTPS `/mcp`, not a public container port. The
container command is:

```text
--mode=panel --listen=127.0.0.1:18081 --panel-socket=/etc/1panel/mcp-runtime/core.sock
```

Core owns `/etc/1panel/mcp-runtime/core.sock`. Mount its dedicated directory `/etc/1panel/mcp-runtime`
read-only at the same path so Core can recreate the Socket without recreating the adapter. Container
root must map to the host Socket owner. Do not create the Socket or relax its owner-only permissions
manually.

The container has a read-only root filesystem, all capabilities dropped, no-new-privileges, 256 MiB
memory with no extra swap, one CPU and 128 PIDs. It mounts no Docker/Agent Socket or panel data
directory and receives no API Key or user token at startup. Its Core client uses only the configured
absolute Unix Socket, fixed `http://unix` authority, and no proxy, redirects or TCP fallback. Panel mode
ignores `PANEL_HOST` / `PANEL_ACCESS_TOKEN` and rejects `--host` / `--token`.

[docker-compose.panel.yml](../docker-compose.panel.yml) is a manual deployment reference. Panel adoption
requires the official repository, `1panel-mcp` name, matching port, ownership labels and verified full
configuration/image identity. After adoption, manage it only through the panel; do not apply Compose
updates.

<a id="transport-and-authorization"></a>
<a id="node-selection-and-authorization"></a>

## Client transport, credentials and nodes

Managed mode accepts stateless `POST /mcp`, protocol `2025-03-26` and JSON responses. `GET /mcp` and `DELETE /mcp` return 405; notifications return 202 without a body. Every request carries
`Authorization: Bearer mcp_...`, `Content-Type: application/json` and
`Accept: application/json, text/event-stream`. Any `Origin` header, including `null`, is rejected.
This supports native CLI/desktop clients, not browser CORS access. Batches contain at most 32 messages
and requests are limited to 1 MiB. Independent mode's SDK also supports `2025-06-18` and `2025-11-25`.
Resources, prompts, notification streams and background MCP tasks are not offered.

Managed authorization is the intersection of the credential's explicit tools and non-empty node
allowlist with its owner's current permissions. Core rechecks credential status, owner, node membership
and ordinary API permission on every API subcall. Revocation between discovery and execution or between
composite steps stops subsequent work. Listing a tool or node never authorizes execution. Grants cover
all resources supported by that tool's APIs on an authorized node; they are not per-site or per-database
resource allowlists. Arbitrary URLs, headers, SQL and owner identities cannot be supplied through the
invocation contract. Dedicated MCP tokens cannot authenticate ordinary public APIs or credential
management; manage credentials through an authorized panel session.

All 19 business tools accept optional `nodeId`, defaulting to `"local"` (primary). Child IDs are
positive decimal strings returned by `list_nodes`, such as `"7"`; names, IP addresses, empty strings,
zero, leading zeroes and non-string values are invalid. `list_nodes` accepts `{}` and returns only safe
metadata for visible nodes. Managed discovery requires its own tool grant and intersects the explicit
node allowlist with the owner's current node permissions; it exposes no addresses or secrets.

```json
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_nodes","arguments":{}}}
```

```json
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_system_info","arguments":{"nodeId":"7"}}}
```

Managed subcalls carry a stable node ID; Core resolves the current registered Agent connection and
rejects missing, revoked or offline targets. Renaming or recreating a node under the same name does not
transfer a grant. The adapter verifies the returned `nodeId`; composite steps never fall back to
primary or switch targets.

Independent local calls need no discovery. Child calls resolve ID → current name through
`POST /api/v2/core/nodes/list` with `{"type":"all"}`, then use an internally set
`CurrentNode: url.QueryEscape(name)` for that whole call. This follows public node-list permissions,
with no cross-call cache or primary fallback. An unavailable or forbidden discovery API blocks child
calls, not local calls. Public API responses do not attest a stable execution ID. Both modes keep
targets request scoped and remove `nodeId` from business DTOs; caller headers cannot select a target.

<a id="offline-manifest-and-twenty-shared-tools"></a>
<a id="query-batch-a-2026-10-04"></a>
<a id="shared-tool-registration"></a>

## Tools and business boundaries

`--catalog` emits an offline contract-v5 manifest. The shared registry in `internal/toolset` binds
definitions and handlers; `shared` owns schemas, policy hashing and response projection. Categories
(`system`, `website`, `certificate`, `app`, `database` ) are display metadata, not grants. CLI
catalog output has no credential grants or policy hash; the authorized Core catalog supplies the latter.
Business tool schemas are version 2; `list_nodes` uses version 1. Unknown fields and parameter aliases
are rejected. Use the current catalog for complete schemas.

| Tool | Purpose / required input |
| --- | --- |
| `list_nodes` | Visible node metadata; no input |
| `get_system_info` | OS metadata |
| `get_dashboard_info` | Dashboard metrics |
| `list_websites` | Website list; optional `name` filter |
| `create_website` | Static/proxy site; `domain`, `website_type` |
| `list_ssls` | Certificate metadata list |
| `create_ssl` | Asynchronous issuance; `domain`, `provider` |
| `install_mysql` | Asynchronous install; `name`, `version`, `port`, `root_password` |
| `install_openresty` | Asynchronous install; `name`, `version`, `http_port`, `https_port` |
| `list_installed_apps` | Installed applications, including `appKey` |
| `list_databases` | Database list; `database` |
| `create_database` | Create database; `database`, `name` |
| `get_website` | Website metadata; `website_id` |
| `list_website_domains` | Domains; `website_id` |
| `get_website_https` | HTTPS metadata; `website_id` |
| `get_ssl_metadata` | Certificate metadata; `ssl_id` |
| `get_installed_app` | Application metadata; `app_install_id` |
| `list_app_versions` | Catalog versions; `app_key` |
| `list_mysql_instances` | Installed local MySQL instances |
| `get_mysql_status` | Fixed MySQL counters; `database` |

- IDs for website, certificate and installed-app queries are positive integers.
  Domain/version/instance lists accept `page` and `page_size` (defaults 1/50,
  maximum 100 per page), returning `total`, `page`, `pageSize`, `items`.
  Searches are bounded to 500 items; metadata source lists to 10,000 entries.
- Database tools require an explicitly named installed local MySQL instance.
  Creation uses `from:local`, `format:utf8mb4` and localhost access by default;
  the existing API/MySQL validates names and optional username/password/Host.
  Managed final APIs revalidate the installed instance. No arbitrary SQL is exposed.
- Website creation accepts optional integer `port` (1–65535). If omitted, it reads
  the selected node's installed OpenResty `httpPort`; failed lookup stops creation,
  with no fallback to 80. Proxy URLs require HTTP(S), no user credentials or raw
  Nginx directive/control characters. Wildcard storage aliases use a bounded hash.
  Success returns `status:completed` and the requested domain, not an invented ID.
- Certificate issuance uses existing accounts. Omitted selectors resolve only a
  single account; ambiguous accounts require explicit IDs. `dnsAccount` and
  `dns_account_id` are mutually exclusive; HTTP rejects wildcard/DNS selectors.
  Bare IPv4/IPv6 requires HTTP, Let's Encrypt and panel IP-issuance support;
  CA address policy and reachability still apply. No ports, CIDRs or zone IDs.
- App installation requires an exact version from that node's `list_app_versions`,
  an explicit 2–30-character ASCII name and integer ports 1–65535. Names allow
  letters/digits plus internal `-`/`_`; OpenResty ports must differ. MySQL root
  passwords are explicit, 8–128 characters and never returned. No compose, mounts,
  arbitrary environment or node-push input is accepted. OpenResty internally reads
  the node's website directory; lookup failure stops installation.
- `get_installed_app` reports recorded state in managed mode, not live Docker
  health. Independent mode uses the ordinary API and may synchronize its cache.
  Managed MySQL queries exclude remote/orphaned records; status SQL has a 30-second
  total budget. These managed Agent checks require the matching Agent release.
- Responses project declared scalar/nested fields and exclude passwords, account
  secrets, certificate/PEM keys, application environment/compose data and sensitive
  paths. Malformed/oversized responses fail, without silent truncation or raw errors.
  Transport response bodies and metadata results are limited to 2 MiB.

SSL issuance and app installation return `requestStatus:accepted`, not completion. Inspect the returned
resource/task in the panel or query tools. Managed results include `invocationId`, actual `nodeId` and
`result`; discovery has no required invocation ID. Independent results have `{nodeId,result}` and
ordinary API logs. A submitted write followed by network/read failure, HTTP/business 5xx, invalid
success data or unconfirmed failure audit has an **unknown outcome**. Inspect the actual resource/task
before a manual retry. The adapter never retries writes; `requestStatus:accepted` and JSON-RPC request
IDs are not idempotency guarantees. Disconnecting or revoking a credential does not roll back an
accepted operation.

<a id="contract-v5-admission-and-policy-binding"></a>

## Private bridge contract

Contract v5 is independent of external MCP protocol versions. The following endpoints are private to the
Unix bridge; `/healthz` requires no credential:

```text
GET  /api/v2/core/mcp/runtime/tools
POST /api/v2/core/mcp/runtime/admit
POST /api/v2/core/mcp/runtime/invoke
POST /api/v2/core/mcp/runtime/complete
GET  /healthz
```

Runtime requests carry the original dedicated Bearer. Core's public proxy adds `X-Panel-MCP-Access`, a
short-lived signed proof bound to the credential and verified client address. The adapter forwards it
only in request context; it cannot sign proofs or derive identity from forwarded headers. Direct adapter
access cannot bypass Core's source policy. Proofs and tickets never appear in MCP responses or logs.

Before a handler runs, `/admit` binds complete arguments, tool, node, schema, policy and request ID to
one opaque ticket. The ticket covers the whole composite call, expires after 350 seconds and allows at
most 64 internal operations. Every subcall still passes current authorization. `/complete` releases the
reservation; a handler-local failure also supplies a controlled `errorCode` for final audit. Completion
uses the original credential/proof context with an independent timeout; Core can release by ticket and
original token digest after revocation/proof expiry. Admission/completion are never retried; failed
admission never executes without a ticket.

`ToolPolicyVersion=4` hashes compact Go JSON in the order `version`, `schemaVersion`, `inputSchema`,
`operations`; operations and response fields are sorted. Descriptions, names and category display
fields do not affect grants. Core and adapter validate the complete operation/schema declaration and
current `policyHash`. API paths come from the installed manifest; `pathParams` binds only declared
placeholders and GET bodies are `{}`. Raw HTTP responses are not exposed to handlers or clients.

The total tool/batch budget is 345 seconds, shared by all composite steps. API invokes use 340 seconds;
catalog requests 30 seconds (5 before a call), admission 5 seconds, completion 2 seconds and the Core
bridge health check 2 seconds. Cancellation propagates; accepted writes may still finish their bounded
work. Current Core limits apply at different boundaries:

| Boundary | Limit |
| --- | --- |
| Public `/mcp` ingress | 1200 requests / 60 seconds, service-wide |
| Admission attempts | 120 / minute per credential |
| Accepted tool calls | 240 / 60 seconds, service-wide |
| Concurrent tool calls | 8, service-wide |

Composite subcalls do not consume another accepted-call quota or concurrency slot. Honor
`retryAfterSeconds`, also returned as MCP `_meta.retryAfterSeconds`. Most failures use `isError:true`
; authentication/disabled failures retain HTTP status. `MCP_POLICY_CHANGED` is 409;
`MCP_CATALOG_UNAVAILABLE` is 503. Fixed safe messages support per-request language preferences; see the
README. Codes, IDs, schemas and policy hashes are unchanged.

<a id="health-logging-and-releases"></a>

## Health and troubleshooting

`--version` reports metadata offline. Managed `/healthz` checks the actual Core bridge and contract,
returning 503 when unavailable/incompatible. It reports no credentials or user data; operational logs
use stderr without bodies or tokens. The container probe is:

```sh
mcp-1panel --healthcheck --listen=127.0.0.1:18081
```

Health alone does not verify enabled state, credential grants, a business operation or external HTTPS
connectivity. Diagnose in this order:

| Symptom | Check |
| --- | --- |
| Install/upgrade fails | Installation task, recovery status, official fixed image reference, architecture and contract |
| Container unhealthy | Core readiness, Socket directory mount/ownership and matching contract |
| 401 / disabled | Dedicated token, credential status/expiry and service enabled state |
| Access denied / empty tools | Source policy, node/tool grants, current owner permissions and refreshed catalog |
| Policy mismatch | Matching image/Core/Agents; refresh catalog and review changed authorization |
| Rate/concurrency limit | `retryAfterSeconds`; avoid automatic write retries |
| Unknown write / disconnected client | Actual resource/task outcome and correlated invocation audit before retry |

<a id="build-native-development-and-legacy-artifacts"></a>
<a id="local-validation"></a>

## Validation and release acceptance

Use the README for [build and client setup](../README.md). Run `go test ./...` and `make docker_test`
for local regression checks. Local tests and mock bridges do not replace Linux Core/Agent deployment
acceptance. Before release, verify HTTPS initialize → tools/list → an
allowed read, a denied direct call, node isolation, revocation between steps and one controlled write on
disposable resources. Confirm asynchronous completion and inspect audit records.

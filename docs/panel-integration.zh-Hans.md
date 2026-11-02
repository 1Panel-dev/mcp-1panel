[English](panel-integration.md) | [中文(简体)](panel-integration.zh-Hans.md)

<a id="1panel-v2-mcp-integration"></a>

# 1Panel v2 MCP 集成

本文说明部署、授权和运行时契约。下载、源码构建及客户端启动方式见
[中文 README](README.zh-Hans.md)。两种模式共享 20 个工具、严格输入 schema 和结果投影。

| 模式 | 认证与执行 | 部署要求 |
| --- | --- | --- |
| 独立模式（`--mode=legacy`） | 配置的 API Key → 现有公开 `/api/v2` API | 已安装的 v2 版本提供所需 API 和兼容 DTO；无需安装托管 MCP |
| 托管模式（`--mode=panel`） | 专用 MCP 凭据 → Core 授权 → 目标 Agent | 配套的 Core、主/子节点 Agent、前端及契约 v5 适配器 |

<a id="independent-api-key-mode"></a>

## 独立 API Key 模式

配置 `PANEL_HOST` / `PANEL_ACCESS_TOKEN`；面板支持多 Key 身份时，可通过可选的
`PANEL_KEY_ID` 选择身份。主机和 Key 配置不可变。每次请求都会使用新的
`1Panel-Timestamp` 和 `1Panel-Token` 签名，并可选携带 `1Panel-Key-ID`。
禁用重定向、环境代理和环境中已有的 Cookie。工具参数不能更改主机、凭据或请求头。

`readonly` 开放 15 个查询工具；`readwrite` 开放 18 个，增加网站、证书和数据库创建；
`full` 开放全部 20 个，增加 MySQL/OpenResty 安装。适配器在每次列举和调用时实施该策略，
包括直接调用隐藏工具的情况；它不能授予额外的面板权限。
每个 API 请求仍遵循账户的常规 API Key 和业务权限规则。
此模式不具备托管 MCP 工具授权、准入票据、调用审计、Agent 快照语义或托管模式的最终资源重新校验。

对于 Streamable HTTP，`MCP_AUTH_TOKEN` / `--mcp-token` 保护传入的 MCP 端点。
该预共享 Bearer 与向外请求使用的 API Key、托管 MCP 凭据分别独立。
请遵循 README 的 HTTPS、Origin 和监听限制。官方 Go MCP SDK 处理协商和生命周期。
工具要求已安装的 v2 版本支持所需 API 和 DTO，并不意味着支持所有版本。

<a id="managed-panel-mode"></a>
<a id="docker-deployment"></a>
<a id="install-and-configure-from-1panel"></a>

## 托管部署与升级

托管 MCP 以单个 Docker 容器运行在主节点。Core/Agent 读取其工具清单，
以凭据当前所有者身份对操作进行鉴权，并执行现有 API。子节点不部署适配器或 MCP 凭据。

1. 部署配套的 Core、前端和主/子节点 Agent。子节点 Agent 必须支持 Core 要求的
   操作绑定身份和资源保护机制。
2. 准备主节点的 rootful Docker Engine 和兼容适配器镜像。
   支持七种镜像架构：386、amd64、arm64、arm/v7、s390x、ppc64le 和 riscv64。
3. 打开 **AI → MCP管理 → 1Panel MCP → 安装**。选择 `1panel/mcp-server` 的固定语义版本标签
   或 SHA256 digest、回环监听端口（默认 `18081`），并按需配置来源访问策略及外部 HTTPS `/mcp` URL。
4. 等待安装完成。面板校验镜像架构、`--version`、`--catalog` 和契约 v5，
   然后启动验证后的不可变镜像 ID。首次安装启用服务；升级保留原启用状态。
   安装失败时，先检查安装和恢复状态，再决定是否重试。
5. 创建专用凭据，显式授权节点和工具，并使用显示的 HTTPS 端点和 token 配置客户端。
   需要节点发现时，单独授权 `list_nodes`。

`1panel/mcp-server:<release-version>` 是占位符，不表示该版本已发布。
请选择目标架构实际可用的版本；托管安装拒绝 `latest` 和自定义镜像仓库。
源码构建、导出和发布命令见 README。本地构建或导出不会发布镜像。
裸二进制用于独立模式，不是托管安装制品。

安装、配置和生命周期操作要求服务管理权限及超级管理员身份。
变更使用当前 `revision`；过期编辑会被拒绝。升级前应等待已受理调用完成，
并通过面板统一管理安装、启动、停止、重启和配置。

升级后刷新目录。Core 将当前能力与已批准的授权快照比较：等价或可证明收窄的工具保留授权；
能力扩大或无法验证时，要求显式审核并保存。新工具不会自动加入凭据。
每次调用都使用准确的当前 schema 和策略哈希。Agent API 变化要求配套的 Agent 版本。

<a id="container-and-socket-contract"></a>

### 容器与 Socket 要求

支持的主机环境是 Linux，使用与 Core 位于同一主节点的 rootful Docker，
不支持远程 daemon、rootless Docker 或用户命名空间重映射。
Host 网络使适配器仅监听字面量 `127.0.0.1:<ListenPort>`；客户端使用面板 HTTPS `/mcp`，
不使用公开的容器端口。容器启动命令为：

```text
--mode=panel --listen=127.0.0.1:18081 --panel-socket=/etc/1panel/mcp-runtime/core.sock
```

Core 管理 `/etc/1panel/mcp-runtime/core.sock`。将其专用目录 `/etc/1panel/mcp-runtime`
只读挂载到相同路径，以便 Core 重建 Socket，而无需重建适配器。
容器 root 必须映射到宿主机 Socket 所有者。不要手动创建 Socket 或放宽仅所有者可访问的权限。

容器使用只读根文件系统、移除全部 capabilities、no-new-privileges，限制为 256 MiB 内存且无额外 swap、
一个 CPU 和 128 个 PID。不挂载 Docker/Agent Socket 或面板数据目录，启动时不传入 API Key 或用户 token。
Core 客户端仅使用配置的绝对路径 Unix Socket，固定 `http://unix` authority，
不使用代理、重定向或 TCP 回退。面板模式忽略 `PANEL_HOST` / `PANEL_ACCESS_TOKEN`，
并拒绝 `--host` / `--token`。

[docker-compose.panel.yml](../docker-compose.panel.yml) 用于手动部署参考。
面板接管要求官方镜像仓库、`1panel-mcp` 名称、匹配的端口、归属标签，以及通过验证的完整配置和镜像身份。
接管后仅通过面板管理，不再执行 Compose 更新。

<a id="transport-and-authorization"></a>
<a id="node-selection-and-authorization"></a>

## 客户端传输、凭据与节点

托管模式接受无状态 `POST /mcp`、协议 `2025-03-26` 和 JSON 响应。
`GET /mcp` 和 `DELETE /mcp` 返回 405；通知返回 202 且无响应体。每个请求携带
`Authorization: Bearer mcp_...`、`Content-Type: application/json` 和
`Accept: application/json, text/event-stream`。任何 `Origin` 请求头（包括 `null`）都会被拒绝。
支持原生 CLI/桌面客户端，不提供浏览器 CORS 访问。批次最多 32 条消息，请求限制为 1 MiB。
独立模式 SDK 还支持 `2025-06-18` 和 `2025-11-25`。
不提供资源、提示词、通知流或后台 MCP 任务。

托管授权取凭据显式工具及非空节点允许列表与所有者当前权限的交集。
Core 在每个 API 子调用中重新检查凭据状态、所有者、节点成员关系和常规 API 权限。
发现与执行之间或复合步骤之间撤销权限，会停止后续工作。
列出工具或节点从不等于执行授权。工具授权覆盖已授权节点上该工具 API 支持的全部资源，
不是按网站或数据库设置的资源允许列表。调用契约不接受任意 URL、请求头、SQL 或所有者身份。
专用 MCP token 不能认证普通公开 API 或管理凭据；请通过已授权的面板 Session 管理凭据。

全部 19 个业务工具接受可选 `nodeId`，默认 `"local"`（主节点）。
子节点 ID 是 `list_nodes` 返回的正十进制数字字符串，例如 `"7"`；
名称、IP 地址、空字符串、零、前导零及非字符串值均无效。
`list_nodes` 接受 `{}`，仅返回可见节点的安全元数据。
托管节点发现需要单独的工具授权，并取显式节点允许列表与所有者当前节点权限的交集，
不暴露地址或秘密。

```json
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_nodes","arguments":{}}}
```

```json
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_system_info","arguments":{"nodeId":"7"}}}
```

托管子调用携带稳定节点 ID；Core 解析当前注册的 Agent 连接，拒绝缺失、已撤权或离线的目标。
重命名或用同一名称重建节点不会转移授权。适配器校验返回的 `nodeId`；
复合步骤不会回退主节点或切换目标。

独立模式的本地调用不需要节点发现。子节点调用通过 `POST /api/v2/core/nodes/list` 和
`{"type":"all"}` 将 ID 解析为当前名称，再为整次调用内部设置
`CurrentNode: url.QueryEscape(name)`。该流程遵循公开节点列表权限，无跨调用缓存或主节点回退。
发现 API 不可用或禁止访问时，阻止子节点调用，不影响本地调用。
公开 API 响应不证明稳定的执行节点 ID。两种模式均将目标限定在当前请求，
从业务 DTO 移除 `nodeId`，且不允许调用方请求头选择目标。

<a id="offline-manifest-and-twenty-shared-tools"></a>
<a id="query-batch-a-2026-10-04"></a>
<a id="shared-tool-registration"></a>

## 工具与业务边界

`--catalog` 输出离线契约 v5 清单。`internal/toolset` 的共享注册表绑定定义和处理器；
`shared` 负责 schema、策略哈希和响应投影。
分类（`system`、`website`、`certificate`、`app`、`database`）是展示元数据，不是授权。
CLI 目录输出不包含凭据授权或策略哈希；策略哈希由 Core 的授权目录提供。
业务工具 schema 为版本 2，`list_nodes` 为版本 1。拒绝未知字段和参数别名。
完整 schema 以当前目录为准。

| 工具 | 用途 / 必填输入 |
| --- | --- |
| `list_nodes` | 可见节点元数据；无输入 |
| `get_system_info` | 操作系统元数据 |
| `get_dashboard_info` | 概览指标 |
| `list_websites` | 网站列表；可选 `name` 筛选 |
| `create_website` | 静态或反向代理网站；`domain`、`website_type` |
| `list_ssls` | 证书元数据列表 |
| `create_ssl` | 异步签发；`domain`、`provider` |
| `install_mysql` | 异步安装；`name`、`version`、`port`、`root_password` |
| `install_openresty` | 异步安装；`name`、`version`、`http_port`、`https_port` |
| `list_installed_apps` | 已安装应用，包含 `appKey` |
| `list_databases` | 数据库列表；`database` |
| `create_database` | 创建数据库；`database`、`name` |
| `get_website` | 网站元数据；`website_id` |
| `list_website_domains` | 域名列表；`website_id` |
| `get_website_https` | HTTPS 元数据；`website_id` |
| `get_ssl_metadata` | 证书元数据；`ssl_id` |
| `get_installed_app` | 应用元数据；`app_install_id` |
| `list_app_versions` | 目录版本；`app_key` |
| `list_mysql_instances` | 已安装的本地 MySQL 实例 |
| `get_mysql_status` | 固定 MySQL 计数器；`database` |

- 网站、证书和已安装应用查询的 ID 为正整数。域名、版本和实例列表接受 `page` 和 `page_size`
  （默认 1/50，每页最多 100 条），返回 `total`、`page`、`pageSize`、`items`。
  搜索最多 500 条，元数据源列表最多 10,000 条。
- 数据库工具要求显式指定已安装的本地 MySQL 实例。创建使用 `from:local`、`format:utf8mb4`，
  默认访问权限为 localhost；现有 API/MySQL 校验名称及可选用户名、密码和 Host。
  托管最终 API 重新校验已安装实例。不暴露任意 SQL。
- 网站创建接受可选整数 `port`（1–65535）。省略时读取目标节点已安装 OpenResty 的 `httpPort`；
  查询失败则停止创建，不回退到 80。代理 URL 要求 HTTP(S)，不得携带用户凭据或原始 Nginx 指令/控制字符。
  通配符存储别名使用有界哈希。成功返回 `status:completed` 和请求的域名，不虚构 ID。
- 证书签发使用现有账户。省略选择器时仅能解析唯一账户；多个账户需要显式 ID。
  `dnsAccount` 与 `dns_account_id` 互斥；HTTP 拒绝通配符和 DNS 选择器。
  裸 IPv4/IPv6 要求 HTTP、Let's Encrypt 和面板 IP 签发支持，仍受 CA 地址策略和可达性限制。
  不接受端口、CIDR 或区域标识符。
- 应用安装要求目标节点 `list_app_versions` 中的准确版本、显式 2–30 字符 ASCII 名称和 1–65535 整数端口。
  名称允许字母和数字，中间可含 `-`/`_`；OpenResty 两个端口必须不同。
  MySQL root 密码须显式提供，长度 8–128 字符，且从不返回。不接受 compose、挂载、任意环境变量或节点推送输入。
  OpenResty 内部读取节点网站目录；查询失败则停止安装。
- 托管模式 `get_installed_app` 返回已记录状态，不是实时 Docker 健康状态。
  独立模式使用普通 API，可能同步其缓存。托管 MySQL 查询排除远程或失去关联的记录，
  状态 SQL 总预算为 30 秒。这些托管 Agent 检查要求配套的 Agent 版本。
- 响应投影仅保留声明的标量和嵌套字段，排除密码、账户秘密、证书/PEM 密钥、应用环境/compose 数据及敏感路径。
  格式错误或超大的响应会失败，不静默截断或返回原始错误。
  传输响应体和元数据结果均限制为 2 MiB。

SSL 签发和应用安装返回 `requestStatus:accepted`，不代表完成。
请在面板或查询工具中检查返回的资源或任务。托管结果包含 `invocationId`、实际 `nodeId` 和 `result`；
节点发现不要求调用 ID。独立模式结果为 `{nodeId,result}`，使用常规 API 日志。
已提交写入后发生网络/读取失败、HTTP/业务码 5xx、无效成功数据或无法确认失败审计时，**结果未知**。
手动重试前先检查实际资源或任务。适配器从不重试写入；`requestStatus:accepted` 和 JSON-RPC 请求 ID 不保证幂等。
断开连接或撤销凭据不会回滚已受理操作。

<a id="contract-v5-admission-and-policy-binding"></a>

## 私有桥接契约

契约 v5 独立于外部 MCP 协议版本。以下接口仅通过私有 Unix 桥接暴露；`/healthz` 无需凭据：

```text
GET  /api/v2/core/mcp/runtime/tools
POST /api/v2/core/mcp/runtime/admit
POST /api/v2/core/mcp/runtime/invoke
POST /api/v2/core/mcp/runtime/complete
GET  /healthz
```

运行时请求携带原始专用 Bearer。Core 公开代理添加 `X-Panel-MCP-Access`，
这是绑定凭据和已验证客户端地址的短期签名证明。
适配器仅在请求上下文中转发，不能签发证明或从转发请求头推导身份。
直接访问适配器不能绕过 Core 来源策略。证明和票据从不出现在 MCP 响应或日志中。

处理器运行前，`/admit` 将完整参数、工具、节点、schema、策略和请求 ID 绑定到一个不透明票据。
票据覆盖整次复合调用，350 秒后过期，最多允许 64 个内部操作。
每个子调用仍通过当前授权。`/complete` 释放预留资源；处理器本地失败时还提供受控 `errorCode` 记录最终审计。
完成请求使用原始凭据/证明上下文及独立超时；撤销或证明过期后，Core 仍可凭票据和原始 token 摘要释放资源。
准入和完成请求从不重试；准入失败后绝不执行无票据操作。

`ToolPolicyVersion=4` 按 `version`、`schemaVersion`、`inputSchema`、`operations` 顺序，
对紧凑 Go JSON 计算哈希；操作和响应字段排序。描述、名称和分类展示字段不影响授权。
Core 和适配器校验完整操作/schema 声明及当前 `policyHash`。
API 路径来自已安装清单；`pathParams` 仅绑定声明的占位符，GET 请求体为 `{}`。
处理器或客户端不接收原始 HTTP 响应。

工具/批次总预算为 345 秒，由所有复合步骤共享。API invoke 为 340 秒，
目录请求 30 秒（调用前为 5 秒），准入 5 秒，完成 2 秒，Core 桥接健康检查 2 秒。
取消信号向下传播；已受理写入仍可能在限定时间内完成。
当前 Core 在不同边界实施限制：

| 边界 | 限制 |
| --- | --- |
| 公开 `/mcp` 入口 | 全服务 1200 请求 / 60 秒 |
| 准入尝试 | 每凭据 120 次 / 分钟 |
| 已受理工具调用 | 全服务 240 次 / 60 秒 |
| 并发工具调用 | 全服务 8 次 |

复合子调用不再消耗已受理调用额度或另占并发槽位。
应遵循 `retryAfterSeconds`，MCP `_meta.retryAfterSeconds` 也会返回该值。
大多数失败使用 `isError:true`；认证失败或禁用保留 HTTP 状态。
`MCP_POLICY_CHANGED` 为 409，`MCP_CATALOG_UNAVAILABLE` 为 503。
固定安全消息支持请求级语言偏好，见 README。错误码、ID、schema 和策略哈希保持不变。

<a id="health-logging-and-releases"></a>

## 健康检查与排障

`--version` 离线报告元数据。托管 `/healthz` 检查实际 Core 桥接和契约，
不可用或不兼容时返回 503。不报告凭据或用户数据；运行日志输出到 stderr，不包含正文或 token。
容器探针为：

```sh
mcp-1panel --healthcheck --listen=127.0.0.1:18081
```

健康状态不能单独验证启用状态、凭据授权、业务操作或外部 HTTPS 连通性。请依次排查：

| 现象 | 检查项 |
| --- | --- |
| 安装/升级失败 | 安装任务、恢复状态、官方固定镜像引用、架构和契约 |
| 容器不健康 | Core 就绪状态、Socket 目录挂载/所有权和匹配的契约 |
| 401 / 禁用 | 专用 token、凭据状态/过期时间和服务启用状态 |
| 拒绝访问 / 工具为空 | 来源策略、节点/工具授权、所有者当前权限和已刷新目录 |
| 策略不匹配 | 配套镜像/Core/Agent；刷新目录并审核授权变更 |
| 速率/并发限制 | `retryAfterSeconds`；不要自动重试写入 |
| 写入结果未知 / 客户端断开 | 重试前检查实际资源/任务结果及关联调用审计 |

<a id="build-native-development-and-legacy-artifacts"></a>
<a id="local-validation"></a>

## 验证与发布验收

构建和客户端配置见 [中文 README](README.zh-Hans.md)。本地回归执行 `go test ./...` 和 `make docker_test`。
本地测试和模拟桥接不能代替 Linux Core/Agent 部署验收。
发布前验证 HTTPS initialize → tools/list → 已授权读取、直接调用被拒绝、节点隔离、
步骤间撤权，以及对一次性业务资源进行受控写入。确认异步完成并检查审计记录。

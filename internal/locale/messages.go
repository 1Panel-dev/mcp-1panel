package locale

import (
	"context"

	"github.com/1Panel-dev/mcp-1panel/internal/toolset/shared"
)

// chineseMessages translates only fixed, safe messages. Match the original
// English text exactly so arbitrary upstream errors cannot become templates.
// Keep these translations at the output boundary: internal error classification
// still uses the original messages, including the unknown-write marker.
var chineseMessages = map[string]string{
	// Shared validation and execution.
	shared.UnknownWriteMessage:                                   "操作结果尚不确定；请先在 1Panel 中确认目标资源的实际状态，不要自动重试写入操作",
	shared.WebsiteAliasHint:                                      "1Panel 报告网站标识已存在。",
	shared.WebsiteDomainHint:                                     "1Panel 报告域名已被网站使用。",
	shared.PortOccupiedHint:                                      "1Panel 报告所请求的端口已被占用。",
	shared.DatabaseExistsHint:                                    "1Panel 报告数据库或其面板记录已存在。",
	shared.MySQLStoppedMessage:                                   "所选 MySQL 实例未运行。请启动数据库服务后重新查询。",
	shared.MySQLConnectionFailedMessage:                          "无法连接所选 MySQL 实例。请检查数据库服务、连接配置和面板日志。",
	shared.MySQLStatusFailedMessage:                              "无法读取所选 MySQL 实例状态。请检查数据库服务是否运行、连接配置和面板日志。",
	shared.SystemInfoFailedMessage:                               "无法读取所选 1Panel 节点的系统信息。请检查 Agent 服务和面板日志。",
	shared.WebsiteAliasHint + " " + shared.UnknownWriteMessage:   "1Panel 报告网站标识已存在。操作结果尚不确定；请先在 1Panel 中确认目标资源的实际状态，不要自动重试写入操作",
	shared.WebsiteDomainHint + " " + shared.UnknownWriteMessage:  "1Panel 报告域名已被网站使用。操作结果尚不确定；请先在 1Panel 中确认目标资源的实际状态，不要自动重试写入操作",
	shared.PortOccupiedHint + " " + shared.UnknownWriteMessage:   "1Panel 报告所请求的端口已被占用。操作结果尚不确定；请先在 1Panel 中确认目标资源的实际状态，不要自动重试写入操作",
	shared.DatabaseExistsHint + " " + shared.UnknownWriteMessage: "1Panel 报告数据库或其面板记录已存在。操作结果尚不确定；请先在 1Panel 中确认目标资源的实际状态，不要自动重试写入操作",
	"Invalid tool arguments":                                     "工具参数无效",
	"Tool transport is unavailable":                              "工具传输服务不可用",
	"Tool transport or handler is unavailable":                   "工具传输服务或处理程序不可用",
	"Invalid API response":                                       "API 响应无效",
	"Query response exceeds its limit":                           "查询响应超过允许的大小限制",
	"Invalid API operation result":                               "API 操作结果无效",

	// Applications.
	"Unexpected application result":                      "应用查询结果与预期不符",
	"Requested application version is unavailable":       "请求的应用版本不可用",
	"Unexpected application detail result":               "应用详情结果与预期不符",
	"The requested installed application does not exist": "请求的已安装应用不存在",
	"Unexpected installed application result":            "已安装应用的查询结果与预期不符",
	"The requested application does not exist":           "请求的应用不存在",
	"Invalid application versions result":                "应用版本列表结果无效",
	"Invalid application version":                        "应用版本无效",
	"Invalid website directory result":                   "网站目录查询结果无效",

	// Certificates. Account kinds are a closed set, not user-controlled text.
	"IP certificates support HTTP validation only":              "IP 证书仅支持 HTTP 验证",
	"IP certificates require a Let's Encrypt ACME account":      "IP 证书需要使用 Let's Encrypt ACME 账户",
	"Invalid certificate metadata result":                       "证书元数据结果无效",
	"Invalid certificate account list":                          "证书账户列表无效",
	"Certificate account list changed during selection":         "选择过程中证书账户列表发生变化",
	"Incomplete certificate account list":                       "证书账户列表不完整",
	"Certificate account list exceeds its limit":                "证书账户列表超过允许的数量限制",
	"The account name is ambiguous; specify its account ID":     "存在同名账户；请指定账户 ID",
	"No ACME account exists":                                    "不存在 ACME 账户",
	"More than one ACME account exists; specify its account ID": "存在多个 ACME 账户；请指定账户 ID",
	"The requested ACME account does not exist":                 "请求的 ACME 账户不存在",
	"No DNS account exists":                                     "不存在 DNS 账户",
	"More than one DNS account exists; specify its account ID":  "存在多个 DNS 账户；请指定账户 ID",
	"The requested DNS account does not exist":                  "请求的 DNS 账户不存在",

	// Databases.
	"The requested local MySQL instance is unavailable": "请求的本地 MySQL 实例不可用",
	"Too many MySQL instances":                          "MySQL 实例数量超过允许的限制",
	"Invalid MySQL instance result":                     "MySQL 实例查询结果无效",
	"Invalid MySQL status result":                       "MySQL 状态查询结果无效",

	// Websites.
	"Invalid website metadata result":                   "网站元数据结果无效",
	"Website domain result exceeds the supported limit": "网站域名查询结果超过支持的数量限制",
	"Invalid website domain result":                     "网站域名查询结果无效",
	"Invalid website HTTPS result":                      "网站 HTTPS 查询结果无效",
	"Invalid OpenResty installation result":             "OpenResty 安装状态查询结果无效",
	"OpenResty is not installed on the selected node":   "所选节点未安装 OpenResty",
	"Invalid OpenResty HTTP port":                       "OpenResty 的 HTTP 端口无效",
	"No default website group exists":                   "不存在默认网站分组",

	// Standalone API Key errors, after safe-message normalization.
	"API Key authentication failed":                                                         "API Key 身份验证失败",
	"The API Key or owner cannot perform this operation":                                    "此 API Key 或其所属用户无权执行该操作",
	"The requested operation is currently unavailable":                                      "请求的操作当前不可用",
	"The operation policy changed":                                                          "操作授权策略已变更",
	"Cannot reach the selected 1Panel node. Check the node status and Agent service.":       "无法连接所选 1Panel 节点。请检查节点状态和 Agent 服务。",
	"The API Key MCP request limit was reached":                                             "已达到 API Key 的 MCP 请求频率限制",
	"Invalid 1Panel API response":                                                           "1Panel API 响应无效",
	"The request to the selected 1Panel node failed. Check the node status and panel logs.": "请求所选 1Panel 节点失败。请检查节点状态和面板日志。",
	shared.NodeAbnormalMessage:                                                              "1Panel 报告所选节点状态异常。请检查节点状态、Agent 服务和面板日志。",

	// Managed adapter and Core bridge.
	"Core MCP bridge is not ready or compatible":       "Core MCP 桥接服务尚未就绪或版本不兼容",
	"Unsupported Core MCP contract version":            "不支持该 Core MCP 契约版本",
	"Invalid Core MCP tool definition":                 "Core MCP 工具定义无效",
	"Core MCP tool policy does not match this adapter": "Core MCP 工具策略与当前适配器不匹配",
	"Invalid Core MCP invocation response":             "Core MCP 调用响应无效",
	"Invalid Core MCP admission response":              "Core MCP 调用准入响应无效",
	"Cannot connect to the 1Panel MCP runtime":         "无法连接 1Panel MCP 运行服务",
	"Invalid Core MCP response":                        "Core MCP 响应无效",
	"The 1Panel MCP runtime rejected the request":      "1Panel MCP 运行服务拒绝了请求",
	"The 1Panel MCP request failed":                    "1Panel MCP 请求失败",
	"API operation is not declared by this tool":       "此工具未声明该 API 操作",
	"Core accepted an unlisted tool":                   "Core 接受了未在目录中列出的工具",

	// MCP HTTP and JSON-RPC errors. Protocol tokens remain unchanged.
	"Browser origins are not supported":                          "不支持来自浏览器的 Origin 请求",
	"MCP authentication required":                                "需要 MCP 身份验证",
	"A dedicated MCP Bearer token is required":                   "需要专用的 MCP Bearer Token",
	"Unsupported MCP protocol version":                           "不支持该 MCP 协议版本",
	"Content-Type must be application/json":                      "Content-Type 必须为 application/json",
	"Accept must include application/json and text/event-stream": "Accept 必须包含 application/json 和 text/event-stream",
	"MCP request is too large":                                   "MCP 请求超过允许的大小限制",
	"Invalid JSON":                                               "JSON 无效",
	"A batch must contain between 1 and 32 messages":             "一个批次必须包含 1 至 32 条消息",
	"Invalid JSON-RPC request":                                   "JSON-RPC 请求无效",
	"A request ID is required":                                   "必须提供请求 ID",
	"Invalid initialization parameters":                          "初始化参数无效",
	"Tool pagination is not supported":                           "不支持工具分页",
	"Method not found":                                           "未找到该方法",
	"Invalid tool parameters":                                    "工具调用参数无效",
	"Tool arguments must be an object":                           "工具参数必须为对象",
	"Request ID is too long":                                     "请求 ID 过长",
	"Unknown managed tool":                                       "未知的托管工具",
	"origin not allowed":                                         "不允许该 Origin 来源",
	"missing or invalid MCP authentication token":                "MCP 身份验证 Token 缺失或无效",
	"Internal Server Error":                                      "服务器内部错误",

	// Fixed safe errors received from Core. Do not translate arbitrary API data.
	"The installed MCP tool catalog is unavailable or incompatible":                                      "已安装的 MCP 工具目录不可用或版本不兼容",
	"Tool API policy changed; reload the catalog and explicitly save the credential authorization again": "工具 API 策略已变更；请重新加载目录，并明确确认后再次保存凭据授权",
	"Invalid MCP credential":                "MCP 凭据无效",
	"MCP credential is inactive or expired": "MCP 凭据已停用、撤销或过期",
	"MCP access is disabled":                "MCP 接入已禁用",
	"Unknown tool":                          "未知工具",
	"Invocation audit is unavailable":       "调用审计服务不可用",
	"Tool execution failed; inspect the panel invocation record before retrying":   "工具执行失败；重试前请先检查面板调用记录，涉及写入时不要自动重试",
	"Invocation outcome could not be recorded; do not retry a write automatically": "无法记录调用结果；不要自动重试写入操作",
	"Selected node is unavailable":                                       "所选节点不可用",
	"Request ended before execution":                                     "请求在执行前已结束",
	"Credential admission is unavailable":                                "凭据准入校验不可用",
	"MCP Bearer credential required":                                     "需要 MCP Bearer 凭据",
	"Too many MCP admission attempts":                                    "MCP 调用准入尝试过于频繁",
	"Admission audit is unavailable":                                     "调用准入审计不可用",
	"Too many concurrent MCP calls":                                      "MCP 并发调用数量过多",
	"MCP global call limit exceeded":                                     "已达到 MCP 全局调用频率限制",
	"Admission could not be created":                                     "无法创建调用准入记录",
	"Panel API execution failed; inspect the invocation before retrying": "面板 API 执行失败；重试前请先检查调用记录，涉及写入时不要自动重试",
	"Panel API returned an incompatible result":                          "面板 API 返回了不兼容的结果",
	"This credential or its owner cannot perform this operation":         "此凭据或其所属用户无权执行该操作",
	"MCP entrance request limit exceeded":                                "已达到 MCP 入口请求频率限制",
}

const unknownExecutionChinese = "MCP 请求执行失败；若涉及写入，请先确认结果，不要自动重试"

var chineseCodeMessages = map[string]string{
	"MCP_UNAUTHENTICATED":     "MCP 身份验证失败",
	"MCP_FORBIDDEN":           "此凭据或其所属用户无权执行该操作",
	"MCP_INVALID_ARGUMENTS":   "工具参数无效",
	"MCP_CONFLICT":            "请求的操作当前不可用",
	"MCP_POLICY_CHANGED":      "工具授权策略已变更；请重新加载目录并明确确认授权",
	"MCP_NODE_UNAVAILABLE":    "所选节点不可用；请检查节点状态和 Agent 服务",
	"MCP_RATE_LIMITED":        "已达到 MCP 请求频率或并发数量限制",
	"MCP_SCHEMA_MISMATCH":     "MCP 响应与预期格式不符",
	"MCP_EXECUTION_FAILED":    unknownExecutionChinese,
	"MCP_UNKNOWN_TOOL":        "未知工具",
	"MCP_CATALOG_UNAVAILABLE": "MCP 工具目录不可用或版本不兼容",
	"MCP_DISABLED":            "MCP 接入已禁用",
	"MCP_RESOURCE_NOT_FOUND":  "请求的资源不存在",
}

var chineseArgumentReasons = map[string]string{
	"is required":                       "为必填项",
	"must be an object":                 "必须是对象",
	"must be a string":                  "必须是字符串",
	"must be a number":                  "必须是数值",
	"must be an integer":                "必须是整数",
	"must be a boolean":                 "必须是布尔值",
	"must be an array":                  "必须是数组",
	"must be null":                      "必须是 null",
	"has an invalid length":             "长度不符合要求",
	"must be one of the allowed values": "必须是允许的选项之一",
	"has an invalid format":             "格式不符合要求",
	"is outside the allowed range":      "超出允许的范围",
	"has an invalid number of items":    "项目数量不符合要求",
	"contains unsupported fields":       "包含不支持的字段",
	"does not match the tool schema":    "不符合工具参数要求",
	"is not supported by this tool":     "不受此工具支持",
}

// Text translates fixed safe messages for Chinese requests and otherwise leaves
// text unchanged. It never interpolates or exposes additional upstream data.
func Text(ctx context.Context, message string) string {
	if Current(ctx) == Chinese {
		if translated, ok := chineseMessages[message]; ok {
			return translated
		}
	}
	return message
}

// Error returns an independent value with only its human-readable message
// localized. Unknown Chinese messages use a safe code-based fallback; English
// messages remain unchanged. Metadata and the caller's original error survive.
func Error(ctx context.Context, failure *shared.ExecutionError) *shared.ExecutionError {
	if failure == nil {
		return nil
	}
	localized := *failure
	if Current(ctx) != Chinese {
		return &localized
	}
	if path, reason, ok := shared.ValidationDetails(failure); ok {
		if translated, exists := chineseArgumentReasons[reason]; exists {
			localized.Message = "工具参数 " + path + "：" + translated
			return &localized
		}
	}
	if translated, ok := chineseMessages[failure.Message]; ok {
		localized.Message = translated
	} else if translated, ok := chineseCodeMessages[failure.Code]; ok {
		localized.Message = translated
	} else {
		localized.Message = unknownExecutionChinese
	}
	return &localized
}

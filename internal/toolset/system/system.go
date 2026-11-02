package system

import (
	"context"
	"encoding/json"

	"github.com/1Panel-dev/mcp-1panel/internal/toolset/shared"
)

// Tools returns this module's tool definitions and handlers.
func Tools() []shared.ToolSpec {
	category := shared.Category{ID: "system", Name: "System", Order: 10}
	tools := []shared.ToolSpec{}
	tools = append(tools, shared.NewTool(category, listNodes, "list_nodes", "List node IDs allowed by this credential and the owner's current node permissions; use an ID as nodeId in other tools", true, shared.Empty(), shared.NewOperation("nodes", "GET", "/core/mcp/runtime/nodes", shared.Empty(), "items.id", "items.name", "items.status", "items.isLocal")))
	tools = append(tools, shared.NewTool(category, getSystemInfo, "get_system_info", "Read operating system information on the primary node; diskSize is bytes", true, shared.Empty(), shared.NewOperation("system_info", "GET", "/dashboard/base/os", shared.Empty(), "os", "platform", "platformFamily", "kernelArch", "kernelVersion", "diskSize")))
	tools = append(tools, shared.NewTool(category, getDashboardInfo, "get_dashboard_info", "Read dashboard counts and resource usage on the primary node", true, shared.Empty(), shared.NewOperation("dashboard", "GET", "/dashboard/base/all/all", shared.Empty(), "websiteNumber", "databaseNumber", "cronjobNumber", "appInstalledNumber", "hostname", "os", "platform", "platformFamily", "platformVersion", "kernelArch", "kernelVersion", "virtualizationSystem", "ipV4Addr", "cpuCores", "cpuLogicalCores", "cpuModelName", "currentInfo.uptime", "currentInfo.timeSinceUptime", "currentInfo.procs", "currentInfo.load1", "currentInfo.load5", "currentInfo.load15", "currentInfo.loadUsagePercent", "currentInfo.cpuPercent", "currentInfo.cpuUsedPercent", "currentInfo.cpuUsed", "currentInfo.cpuTotal", "currentInfo.memoryTotal", "currentInfo.memoryAvailable", "currentInfo.memoryUsed", "currentInfo.memoryUsedPercent", "currentInfo.swapMemoryTotal", "currentInfo.swapMemoryAvailable", "currentInfo.swapMemoryUsed", "currentInfo.swapMemoryUsedPercent", "currentInfo.netBytesSent", "currentInfo.netBytesRecv", "currentInfo.shotTime", "currentInfo.diskData.path", "currentInfo.diskData.total", "currentInfo.diskData.free", "currentInfo.diskData.used", "currentInfo.diskData.usedPercent")))
	return tools
}
func getSystemInfo(ctx context.Context, transport shared.APITransport, _ map[string]any) (json.RawMessage, error) {
	return shared.API(ctx, transport, "GET", "/dashboard/base/os", nil)
}
func getDashboardInfo(ctx context.Context, transport shared.APITransport, _ map[string]any) (json.RawMessage, error) {
	return shared.API(ctx, transport, "GET", "/dashboard/base/all/all", nil)
}

func listNodes(ctx context.Context, transport shared.APITransport, _ map[string]any) (json.RawMessage, error) {
	return shared.QueryAPI(ctx, transport, "GET", "/core/mcp/runtime/nodes", nil)
}

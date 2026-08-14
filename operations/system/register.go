package system

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/1Panel-dev/mcp-1panel/operations/types"
	"github.com/1Panel-dev/mcp-1panel/utils"
)

func RegisterTools(s *mcp.Server, allowed func(string) bool) {
	if allowed(GetSystemInfo) {
		mcp.AddTool(s, &mcp.Tool{
			Name:        GetSystemInfo,
			Title:       "Get system information",
			Description: "Get operating system, kernel, architecture, and disk information. diskSize is measured in bytes.",
			Annotations: readOnlyAnnotations(),
		}, utils.TypedToolHandler[GetSystemInfoInput, *types.OsInfoRes](getSystemInfo))
	}
	if allowed(GetDashboardInfo) {
		mcp.AddTool(s, &mcp.Tool{
			Name:        GetDashboardInfo,
			Title:       "Get dashboard information",
			Description: "Get the current 1Panel dashboard resource counts and host status.",
			Annotations: readOnlyAnnotations(),
		}, utils.TypedToolHandler[GetDashboardInfoInput, *types.DashboardRes](getDashboardInfo))
	}
}

func readOnlyAnnotations() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		ReadOnlyHint:    true,
		DestructiveHint: new(bool),
		IdempotentHint:  true,
		OpenWorldHint:   new(bool),
	}
}

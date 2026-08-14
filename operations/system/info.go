package system

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/1Panel-dev/mcp-1panel/operations/types"
	"github.com/1Panel-dev/mcp-1panel/utils"
)

const (
	GetSystemInfo = "get_system_info"
)

func getSystemInfo(ctx context.Context, _ *mcp.CallToolRequest, input GetSystemInfoInput) (*mcp.CallToolResult, any, error) {
	client := utils.NewPanelClient("GET", "/dashboard/base/os")
	osInfo := &types.OsInfoRes{}
	result, err := client.Request(ctx, osInfo)
	if result != nil {
		result.StructuredContent = osInfo
	}
	return utils.ToolResult(result, err)
}

type GetSystemInfoInput struct{}

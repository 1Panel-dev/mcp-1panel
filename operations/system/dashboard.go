package system

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/1Panel-dev/mcp-1panel/operations/types"
	"github.com/1Panel-dev/mcp-1panel/utils"
)

const (
	GetDashboardInfo = "get_dashboard_info"
)

func getDashboardInfo(ctx context.Context, _ *mcp.CallToolRequest, input GetDashboardInfoInput) (*mcp.CallToolResult, any, error) {
	client := utils.NewPanelClient("GET", "/dashboard/base/all/all")
	info := &types.DashboardRes{}
	result, err := client.Request(ctx, info)
	if result != nil {
		result.StructuredContent = info
	}
	return utils.ToolResult(result, err)
}

type GetDashboardInfoInput struct{}

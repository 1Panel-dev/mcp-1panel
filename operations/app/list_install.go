package app

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/1Panel-dev/mcp-1panel/operations/types"
	"github.com/1Panel-dev/mcp-1panel/utils"
)

const (
	ListInstalledApps = "list_installed_apps"
)

func listInstalledApps(ctx context.Context, _ *mcp.CallToolRequest, input ListInstalledAppsInput) (*mcp.CallToolResult, any, error) {
	req := &types.PageRequest{
		Page:     1,
		PageSize: 500,
	}
	appListRes := &types.AppInstalledListResponse{}
	result, err := utils.NewPanelClient("POST", "/apps/installed/search", utils.WithPayload(req)).Request(ctx, appListRes)
	if result != nil {
		result.StructuredContent = appListRes
	}
	return utils.ToolResult(result, err)
}

type ListInstalledAppsInput struct {
}

package app

import (
	"context"
	"mcp-1panel/operations/types"
	"mcp-1panel/utils"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ListInstalledApps = "list_installed_apps"
)

var ListInstalledAppsTool = mcp.NewTool(
	ListInstalledApps,
	mcp.WithDescription("list installed apps"),
)
func ListInstalledAppsHandle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	req := &types.PageRequest{
		Page:     1,
		PageSize: 500,
	}
	appListRes := &types.AppInstalledListResponse{}
	return utils.NewPanelClient("POST", "/apps/installed/search", utils.WithPayload(req)).Request(appListRes)
}
